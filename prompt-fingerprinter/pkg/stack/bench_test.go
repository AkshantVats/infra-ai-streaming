// SPDX-License-Identifier: MIT

package stack

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/akshantvats/prompt-fingerprinter/pkg/fingerprint"
)

// This file is Day 75's implementation of DESIGN.md §6's commitment:
// "Day 75 ships the exact-match cache with a real hit-rate benchmark
// and BENCHMARKS.md." Two things are measured here, both against
// MemRedis rather than a live Redis instance — no Docker daemon in
// this sandbox, the same constraint every prior Redis-dependent day in
// this repo (56, 64, 65, 70) has logged:
//
//  1. Lookup latency percentiles for an L1 hit versus the
//     "semantic-only" path this cache sits in front of (L1 miss, L2
//     hit) — BenchmarkStack_Get_L1Hit / BenchmarkStack_Get_L2Only and
//     TestLatencyPercentiles_L1Hit below.
//  2. What fraction of a realistic duplicate-heavy workload L1
//     resolves without ever reaching L2 — TestHitRate_DuplicateWorkload.
//
// MemRedis's Get/Set are an in-process map lookup under a mutex, not a
// network round trip — so the absolute nanosecond numbers here are a
// lower bound on what a live Redis GET would cost, not a claim about
// production latency. What they do measure honestly: the fixed
// overhead Stack.Get adds on top of whatever the Redis round trip
// costs (fingerprint computation, span creation, map access) — the
// part of the latency budget this module actually controls.

// slowL2 simulates "the semantic-only path this module sits in front
// of": a fixed artificial delay standing in for semantic-cache-engine's
// embedding-model call + pgvector search (DESIGN.md §1/§4), so
// BenchmarkStack_Get_L2Only's number is comparable to what a request
// pays when it has to reach L2 — L1 miss or no L1 at all.
type slowL2 struct {
	delay    time.Duration
	response string
	calls    int
}

func (s *slowL2) Get(_ context.Context, _ string, _ fingerprint.PromptRequest) (string, bool, error) {
	s.calls++
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	return s.response, true, nil
}

func benchReq(i int) fingerprint.PromptRequest {
	return fingerprint.PromptRequest{
		Messages: []fingerprint.Message{{Role: "user", Content: fmt.Sprintf("summarize document #%d for the weekly digest", i)}},
		Model:    "gpt-4o",
	}
}

// BenchmarkStack_Get_L1Hit measures the cost of a request that resolves
// entirely at L1 — DESIGN.md §3's fast path, the one every byte-identical
// retry or replayed batch job should take.
func BenchmarkStack_Get_L1Hit(b *testing.B) {
	ctx := context.Background()
	req := benchReq(0)
	redis := NewMemRedis()
	key := fingerprint.RedisKey("tenant-bench", fingerprint.Fingerprint(req))
	if err := redis.Set(ctx, key, "cached response", HardTTL); err != nil {
		b.Fatalf("seed redis: %v", err)
	}
	s := &Stack{Redis: redis, L2: &slowL2{delay: 15 * time.Millisecond, response: "should never be reached"}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.Get(ctx, "tenant-bench", req); err != nil {
			b.Fatalf("Get: %v", err)
		}
	}
}

// BenchmarkStack_Get_L2Only measures the "semantic-only" baseline this
// module exists to avoid: every request misses L1 (a distinct prompt
// each time, so no fingerprint ever repeats) and pays the full L2 cost.
// This is the number DESIGN.md §4's "sub-millisecond lookup" claim is
// measured against — the gap between this and BenchmarkStack_Get_L1Hit
// is exactly what the fingerprint cache buys back.
func BenchmarkStack_Get_L2Only(b *testing.B) {
	ctx := context.Background()
	redis := NewMemRedis()
	s := &Stack{Redis: redis, L2: &slowL2{delay: 15 * time.Millisecond, response: "semantic response"}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.Get(ctx, "tenant-bench", benchReq(i)); err != nil {
			b.Fatalf("Get: %v", err)
		}
	}
}

// TestLatencyPercentiles_L1Hit records per-call wall-clock duration for
// b.N-style repeated L1 hits and reports p50/p95/p99, rather than only
// testing.B's mean ns/op — the plan's target ("p50 lookup <2ms") is a
// percentile claim, and a mean can hide a heavy tail a percentile
// would catch. Numbers are logged with `go test -v` and are what
// BENCHMARKS.md quotes.
func TestLatencyPercentiles_L1Hit(t *testing.T) {
	const iterations = 5000
	ctx := context.Background()
	req := benchReq(0)
	redis := NewMemRedis()
	key := fingerprint.RedisKey("tenant-pctl", fingerprint.Fingerprint(req))
	if err := redis.Set(ctx, key, "cached response", HardTTL); err != nil {
		t.Fatalf("seed redis: %v", err)
	}
	s := &Stack{Redis: redis, L2: &slowL2{delay: 15 * time.Millisecond}}

	durations := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		start := time.Now()
		if _, err := s.Get(ctx, "tenant-pctl", req); err != nil {
			t.Fatalf("Get: %v", err)
		}
		durations = append(durations, time.Since(start))
	}

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p50 := durations[iterations*50/100]
	p95 := durations[iterations*95/100]
	p99 := durations[iterations*99/100]
	t.Logf("L1 hit latency over %d iterations: p50=%s p95=%s p99=%s", iterations, p50, p95, p99)

	if p50 >= 2*time.Millisecond {
		t.Fatalf("p50 = %s, want < 2ms (DESIGN.md §4 / plan.json Day 75 target)", p50)
	}
}

// TestHitRate_DuplicateWorkload simulates a realistic gateway workload:
// a pool of "canonical" prompts that get retried, replayed, or
// double-submitted at a known rate, mixed with genuinely novel prompts
// that always miss L1. It reports what fraction of the workload L1
// absorbs without ever reaching L2 — the number DESIGN.md §3 argues
// for but never measured (no live Redis in that design-only day).
//
// dupRate is deliberately conservative (35%) rather than picking a
// number that flatters the cache: retries and batch replays are real
// but not the majority of traffic in most gateways, and the point of
// this test is an honest lower bound, not a best case.
func TestHitRate_DuplicateWorkload(t *testing.T) {
	const (
		totalRequests = 4000
		poolSize      = 50
		dupRate       = 0.35
	)
	ctx := context.Background()
	redis := NewMemRedis()
	l2 := &slowL2{response: "semantic response"}
	metrics := NewMemMetrics()
	s := &Stack{Redis: redis, L2: l2, Metrics: metrics}

	rng := rand.New(rand.NewSource(75)) // fixed seed: Day 75, reproducible workload
	pool := make([]fingerprint.PromptRequest, poolSize)
	for i := range pool {
		pool[i] = benchReq(i)
	}

	var l1Hits, l2Hits int
	novelCounter := poolSize
	for i := 0; i < totalRequests; i++ {
		var req fingerprint.PromptRequest
		if rng.Float64() < dupRate {
			req = pool[rng.Intn(poolSize)] // a retry/replay of an earlier prompt
		} else {
			req = benchReq(novelCounter) // a genuinely new prompt, never seen before
			novelCounter++
		}
		result, err := s.Get(ctx, "tenant-workload", req)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		switch result.Tier {
		case TierL1:
			l1Hits++
		case TierL2:
			l2Hits++
		}
	}

	hitRate := float64(l1Hits) / float64(totalRequests)
	l2CallsAvoided := totalRequests - l2.calls
	t.Logf("workload=%d dupRate=%.2f pool=%d -> L1 hits=%d L2 calls=%d hitRate=%.1f%% L2 calls avoided vs semantic-only=%d (%.1f%%)",
		totalRequests, dupRate, poolSize, l1Hits, l2.calls, hitRate*100, l2CallsAvoided, float64(l2CallsAvoided)/float64(totalRequests)*100)

	// A weak sanity bound, not a tight one: the first occurrence of every
	// distinct prompt must miss L1 (nothing to backfill from yet), so
	// hitRate is necessarily below dupRate. It should still land well
	// above zero — if it doesn't, the backfill-on-L2-hit path is broken.
	if l1Hits == 0 {
		t.Fatalf("want a non-zero L1 hit rate on a workload with %.0f%% duplicates, got 0", dupRate*100)
	}
}
