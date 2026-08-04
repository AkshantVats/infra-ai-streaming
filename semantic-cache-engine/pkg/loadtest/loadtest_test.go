// SPDX-License-Identifier: MIT

package loadtest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/akshantvats/semantic-cache-engine/pkg/cachestore"
)

func TestComputePercentilesEmpty(t *testing.T) {
	p50, p95, p99 := computePercentiles(nil)
	if p50 != 0 || p95 != 0 || p99 != 0 {
		t.Fatalf("computePercentiles(nil) = %v/%v/%v, want all zero", p50, p95, p99)
	}
}

func TestComputePercentilesKnownDistribution(t *testing.T) {
	// 100 evenly spaced durations, 1ms .. 100ms, in shuffled order --
	// nearest-rank over a sorted copy should still recover exact
	// percentiles regardless of input order.
	durations := make([]time.Duration, 100)
	for i := range durations {
		// Interleave so the input isn't already sorted.
		idx := (i*37 + 1) % 100
		durations[i] = time.Duration(idx+1) * time.Millisecond
	}

	p50, p95, p99 := computePercentiles(durations)
	if p50 <= 0 || p95 <= 0 || p99 <= 0 {
		t.Fatalf("computePercentiles = %v/%v/%v, want all positive", p50, p95, p99)
	}
	if p50 > p95 || p95 > p99 {
		t.Fatalf("computePercentiles = %v/%v/%v, want p50 <= p95 <= p99", p50, p95, p99)
	}
	// With 100 samples of 1ms..100ms, p99 should land near the top of
	// the range and p50 near the middle.
	if p99 < 95*time.Millisecond {
		t.Errorf("p99 = %v, want >= 95ms for this distribution", p99)
	}
	if p50 < 45*time.Millisecond || p50 > 55*time.Millisecond {
		t.Errorf("p50 = %v, want within 45-55ms for this distribution", p50)
	}
}

func TestComputePercentilesSingleValue(t *testing.T) {
	p50, p95, p99 := computePercentiles([]time.Duration{7 * time.Millisecond})
	if p50 != 7*time.Millisecond || p95 != 7*time.Millisecond || p99 != 7*time.Millisecond {
		t.Fatalf("computePercentiles(single) = %v/%v/%v, want all 7ms", p50, p95, p99)
	}
}

func TestMemStoreFindNearestHitAndMiss(t *testing.T) {
	store := NewMemStore(0)
	store.Seed("tenant-a", cachestore.Match{PromptHash: "hash-a", Response: "cached", Similarity: 0.97})

	match, ok, err := store.FindNearest(context.Background(), "tenant-a", []float32{0.1, 0.2})
	if err != nil {
		t.Fatalf("FindNearest: %v", err)
	}
	if !ok || match.Response != "cached" {
		t.Fatalf("FindNearest = %+v, %v, want a hit with the seeded response", match, ok)
	}

	_, ok, err = store.FindNearest(context.Background(), "tenant-b", []float32{0.1, 0.2})
	if err != nil {
		t.Fatalf("FindNearest for unseeded tenant: %v", err)
	}
	if ok {
		t.Fatalf("FindNearest for unseeded tenant reported a hit, want a miss")
	}
}

func TestMemStoreFindExactAlwaysMiss(t *testing.T) {
	store := NewMemStore(0)
	store.Seed("tenant-a", cachestore.Match{PromptHash: "hash-a", Response: "cached"})

	_, ok, err := store.FindExact(context.Background(), "tenant-a", "hash-a")
	if err != nil {
		t.Fatalf("FindExact: %v", err)
	}
	if ok {
		t.Fatalf("FindExact reported a hit, want a miss -- Day 64's harness only drives FindNearest")
	}
}

func TestMemStoreLatencyIsApplied(t *testing.T) {
	store := NewMemStore(20 * time.Millisecond)
	start := time.Now()
	if _, _, err := store.FindNearest(context.Background(), "tenant-a", nil); err != nil {
		t.Fatalf("FindNearest: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 20*time.Millisecond {
		t.Fatalf("FindNearest returned after %v, want >= 20ms simulated latency", elapsed)
	}
}

func TestMemStoreRespectsContextCancellation(t *testing.T) {
	store := NewMemStore(time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, _, _ = store.FindNearest(ctx, "tenant-a", nil)
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("FindNearest ignored context cancellation, took %v", elapsed)
	}
}

func TestRunValidatesConfig(t *testing.T) {
	store := NewMemStore(0)
	base := Config{QPS: 10, Duration: 50 * time.Millisecond, Concurrency: 4, TenantID: "tenant-a"}

	cases := []struct {
		name string
		mut  func(c Config) Config
	}{
		{"zero QPS", func(c Config) Config { c.QPS = 0; return c }},
		{"zero duration", func(c Config) Config { c.Duration = 0; return c }},
		{"zero concurrency", func(c Config) Config { c.Concurrency = 0; return c }},
		{"empty tenant", func(c Config) Config { c.TenantID = ""; return c }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Run(context.Background(), tc.mut(base), store); err == nil {
				t.Fatalf("Run with %s: want error, got nil", tc.name)
			}
		})
	}
}

func TestRunAgainstMemStore(t *testing.T) {
	store := NewMemStore(1 * time.Millisecond)
	store.Seed("tenant-a", cachestore.Match{PromptHash: "hash-a", Response: "cached", Similarity: 0.95})

	cfg := Config{
		QPS:         200,
		Duration:    100 * time.Millisecond,
		Concurrency: 16,
		TenantID:    "tenant-a",
		Embedding:   []float32{0.1, 0.2, 0.3},
	}

	result, err := Run(context.Background(), cfg, store)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Requests == 0 {
		t.Fatalf("Run: got zero requests, want at least a few over 100ms at 200 QPS")
	}
	if result.Errors != 0 {
		t.Fatalf("Run: got %d errors against a healthy MemStore, want 0", result.Errors)
	}
	if result.Hits != result.Requests-result.Errors {
		t.Fatalf("Run: hits=%d requests=%d errors=%d, want every successful call to hit the seeded tenant",
			result.Hits, result.Requests, result.Errors)
	}
	if result.P50 > result.P95 || result.P95 > result.P99 {
		t.Fatalf("Run: percentiles out of order p50=%v p95=%v p99=%v", result.P50, result.P95, result.P99)
	}
	if result.Achieved <= 0 {
		t.Fatalf("Run: Achieved = %v, want positive throughput", result.Achieved)
	}
}

// erroringStore always returns an error, so Run's error-counting path is
// exercised without needing a real failure mode from MemStore.
type erroringStore struct{}

func (erroringStore) FindExact(context.Context, string, string) (cachestore.Match, bool, error) {
	return cachestore.Match{}, false, errors.New("boom")
}

func (erroringStore) FindNearest(context.Context, string, []float32) (cachestore.Match, bool, error) {
	return cachestore.Match{}, false, errors.New("boom")
}

func TestRunCountsErrors(t *testing.T) {
	cfg := Config{QPS: 100, Duration: 50 * time.Millisecond, Concurrency: 8, TenantID: "tenant-a"}
	result, err := Run(context.Background(), cfg, erroringStore{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Requests == 0 {
		t.Fatalf("Run: got zero requests, want at least a few over 50ms at 100 QPS")
	}
	if result.Errors != result.Requests {
		t.Fatalf("Run: errors=%d requests=%d, want every call against erroringStore to fail", result.Errors, result.Requests)
	}
}
