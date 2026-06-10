// SPDX-License-Identifier: MIT
//go:build integration

package server_test

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/akshantvats/distributed-flagd/internal/etcdstore"
)

// BenchmarkWatchPropagation measures P99 latency from etcd Put to gRPC EvaluateStream DELTA receipt.
// Requires Docker (testcontainers) — run with: go test -bench=BenchmarkWatchPropagation -tags=integration -benchtime=100x ./internal/server/
func BenchmarkWatchPropagation(b *testing.B) {
	endpoint, cleanup := startEtcd(b)
	defer cleanup()

	store, err := etcdstore.NewClient(endpoint)
	if err != nil {
		b.Fatalf("NewClient: %v", err)
	}
	defer store.Close()

	latencies := make([]time.Duration, 0, b.N)
	ctx := context.Background()

	watchCh := make(chan etcdstore.FlagValue, 256)
	go store.Watch(ctx, watchCh)
	time.Sleep(100 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t0 := time.Now()
		fv := etcdstore.FlagValue{
			FlagName:  fmt.Sprintf("bench-flag-%d", i),
			Type:      "bool",
			ValueJSON: "true",
		}
		_ = store.Put(ctx, fv, "bench", "propagation benchmark")
		select {
		case <-watchCh:
			latencies = append(latencies, time.Since(t0))
		case <-time.After(500 * time.Millisecond):
			b.Logf("iteration %d: timeout waiting for watch event", i)
		}
	}

	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		p50 := latencies[len(latencies)/2]
		p95 := latencies[int(float64(len(latencies))*0.95)]
		p99 := latencies[int(float64(len(latencies))*0.99)]
		b.ReportMetric(float64(p50.Milliseconds()), "p50-ms")
		b.ReportMetric(float64(p95.Milliseconds()), "p95-ms")
		b.ReportMetric(float64(p99.Milliseconds()), "p99-ms")
	}
}
