// SPDX-License-Identifier: MIT

// Package loadtest drives concurrent semantic-cache-engine cache lookups
// at a target QPS for a fixed duration and reports the resulting p50/p95/p99
// latency plus achieved throughput -- Day 64's harness for the "load test
// 1k QPS lookups, p99 under 15ms" plan item.
//
// Run exercises cachestore.Reader.FindNearest specifically, not the full
// pkg/lookup.Lookup path: FindNearest is pgvector's hnsw index query, the
// part of the lookup whose cost actually depends on index size and
// tuning, while the embedding call Lookup also makes has its own,
// unrelated SLA (OpenAI's API latency). Measuring both together would
// blame the index for however long the embedding API happens to take
// that day. See DESIGN.md §10 for the full rationale.
package loadtest

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/akshantvats/semantic-cache-engine/pkg/cachestore"
)

// Config controls one load test run.
type Config struct {
	// QPS is the target requests/sec, evenly spaced by a ticker.
	QPS int
	// Duration is how long to generate load for.
	Duration time.Duration
	// Concurrency bounds how many FindNearest calls can be in flight at
	// once, so a slow tick doesn't spawn unbounded goroutines.
	Concurrency int
	// TenantID is the tenant every simulated lookup queries.
	TenantID string
	// Embedding is the query vector every simulated lookup searches
	// with. A fixed vector is enough to exercise the index query path;
	// Day 64's harness measures the store's read latency, not recall
	// across varied queries.
	Embedding []float32
}

// Result is the outcome of a Run call.
type Result struct {
	// Requests is the total number of FindNearest calls attempted.
	Requests int
	// Errors is how many of those calls returned a non-nil error.
	Errors int
	// Hits is how many calls found a candidate (regardless of whether a
	// caller's similarity threshold would treat it as a cache hit --
	// Run measures store latency, not lookup.Lookup's threshold logic).
	Hits int
	// Misses is Requests - Errors - Hits.
	Misses int
	// P50, P95, P99 are latency percentiles over successful calls only.
	P50, P95, P99 time.Duration
	// Achieved is the actual requests/sec observed, which can trail
	// Config.QPS if Concurrency is too low to keep up with the ticker.
	Achieved float64
}

// Run drives Config.QPS FindNearest calls/sec against store for
// Config.Duration, using a closed-loop ticker (fire at a fixed rate,
// dispatch onto a bounded worker pool) rather than an open-loop load
// generator. A closed-loop harness under-counts tail latency during a
// real slowdown -- a worker stuck on a slow call stops generating new
// load instead of piling requests up the way independent real clients
// would (the "coordinated omission" problem) -- so Run's numbers answer
// "is the store's own query cost within budget at this concurrency," not
// "what would production traffic's actual p99 be." See DESIGN.md §10.
func Run(ctx context.Context, cfg Config, store cachestore.Reader) (Result, error) {
	if cfg.QPS <= 0 {
		return Result{}, fmt.Errorf("loadtest: QPS must be positive, got %d", cfg.QPS)
	}
	if cfg.Duration <= 0 {
		return Result{}, fmt.Errorf("loadtest: Duration must be positive, got %s", cfg.Duration)
	}
	if cfg.Concurrency <= 0 {
		return Result{}, fmt.Errorf("loadtest: Concurrency must be positive, got %d", cfg.Concurrency)
	}
	if cfg.TenantID == "" {
		return Result{}, fmt.Errorf("loadtest: TenantID is required")
	}

	interval := time.Second / time.Duration(cfg.QPS)
	totalTicks := int(cfg.Duration / interval)

	var (
		mu         sync.Mutex
		latencies  = make([]time.Duration, 0, totalTicks)
		errorCount int64
		hitCount   int64
	)
	sem := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup

	start := time.Now()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

ticks:
	for tick := 0; tick < totalTicks; tick++ {
		select {
		case <-ctx.Done():
			break ticks
		case <-ticker.C:
			sem <- struct{}{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-sem }()

				callStart := time.Now()
				_, ok, err := store.FindNearest(ctx, cfg.TenantID, cfg.Embedding)
				latency := time.Since(callStart)

				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					return
				}
				if ok {
					atomic.AddInt64(&hitCount, 1)
				}
				mu.Lock()
				latencies = append(latencies, latency)
				mu.Unlock()
			}()
		}
	}
	wg.Wait()
	elapsed := time.Since(start)

	p50, p95, p99 := computePercentiles(latencies)
	requests := len(latencies) + int(errorCount)

	return Result{
		Requests: requests,
		Errors:   int(errorCount),
		Hits:     int(hitCount),
		Misses:   len(latencies) - int(hitCount),
		P50:      p50,
		P95:      p95,
		P99:      p99,
		Achieved: float64(requests) / elapsed.Seconds(),
	}, nil
}

// computePercentiles returns the p50/p95/p99 of durations using
// nearest-rank selection over a sorted copy. An empty input returns all
// zeros rather than panicking or dividing by zero, since a run that
// produced zero successful calls (e.g. every call errored) still needs a
// well-defined Result.
func computePercentiles(durations []time.Duration) (p50, p95, p99 time.Duration) {
	if len(durations) == 0 {
		return 0, 0, 0
	}
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	return percentile(sorted, 0.50), percentile(sorted, 0.95), percentile(sorted, 0.99)
}

// percentile returns the nearest-rank p-th percentile of an
// already-sorted slice.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 1 {
		return sorted[0]
	}
	idx := int(p * float64(len(sorted)-1))
	return sorted[idx]
}
