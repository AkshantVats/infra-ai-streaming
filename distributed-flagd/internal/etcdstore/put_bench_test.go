// SPDX-License-Identifier: MIT
//go:build integration

package etcdstore_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/akshantvats/distributed-flagd/internal/etcdstore"
)

// BenchmarkPut measures etcd write throughput including the atomic audit Txn.
// Requires Docker (testcontainers) — run with: go test -bench=BenchmarkPut -tags=integration -benchtime=5s ./internal/etcdstore/
func BenchmarkPut(b *testing.B) {
	endpoint, cleanup := startEtcd(b)
	defer cleanup()

	c, err := etcdstore.NewClient(endpoint)
	if err != nil {
		b.Fatalf("NewClient: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fv := etcdstore.FlagValue{
			FlagName:  fmt.Sprintf("bench-flag-%d", i%100),
			Type:      "bool",
			ValueJSON: "true",
		}
		_ = c.Put(ctx, fv, "bench", "benchmark run")
	}
}
