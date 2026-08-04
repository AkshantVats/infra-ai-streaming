// SPDX-License-Identifier: MIT

// Command loadtest is the CLI entry point for semantic-cache-engine's
// Day 64 load-test harness (pkg/loadtest): it drives a target QPS of
// cachestore.Reader.FindNearest calls for a fixed duration and reports
// p50/p95/p99 latency plus achieved throughput.
//
// With PGVECTOR_DSN (or --dsn) set, it connects to a real
// Postgres+pgvector instance -- e.g. this module's docker-compose.yml --
// seeds one synthetic entry, and measures the real hnsw index query
// path. With neither set, it falls back to pkg/loadtest.MemStore, an
// in-memory simulated-latency stand-in, so `go run ./cmd/loadtest` still
// works without Docker -- see README.md's "Load test (Day 64)" section
// and DESIGN.md §10 for why the fallback exists and what it does and
// doesn't measure.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/akshantvats/semantic-cache-engine/pkg/cachestore"
	"github.com/akshantvats/semantic-cache-engine/pkg/embedder"
	"github.com/akshantvats/semantic-cache-engine/pkg/loadtest"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run implements the CLI and returns the process exit code, kept
// separate from main for testability without exec'ing a built binary
// (same shape as cmd/embedworker/main.go::run).
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("loadtest", flag.ContinueOnError)
	fs.SetOutput(stderr)
	qps := fs.Int("qps", 1000, "target requests/sec")
	duration := fs.Duration("duration", 10*time.Second, "how long to generate load for")
	concurrency := fs.Int("concurrency", 64, "max in-flight FindNearest calls")
	tenant := fs.String("tenant", "loadtest-tenant", "tenant ID every simulated lookup queries")
	dsn := fs.String("dsn", os.Getenv("PGVECTOR_DSN"), "Postgres DSN; falls back to an in-memory simulated store if empty")
	simLatency := fs.Duration("sim-latency", 2*time.Millisecond, "simulated round-trip latency, only used when --dsn is empty")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ctx := context.Background()
	cfg := loadtest.Config{
		QPS:         *qps,
		Duration:    *duration,
		Concurrency: *concurrency,
		TenantID:    *tenant,
		Embedding:   syntheticEmbedding(),
	}

	var store cachestore.Reader
	if *dsn != "" {
		writer, err := cachestore.NewPostgresWriter(ctx, *dsn)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "loadtest: %v\n", err)
			return 1
		}
		defer writer.Close()

		seed := cachestore.Entry{
			TenantID:   *tenant,
			PromptHash: "loadtest-seed",
			Embedding:  cfg.Embedding,
			Response:   "loadtest cached response",
		}
		if err := writer.WriteEntries(ctx, []cachestore.Entry{seed}); err != nil {
			_, _ = fmt.Fprintf(stderr, "loadtest: seed entry: %v\n", err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout, "loadtest: running against live Postgres+pgvector at %s\n", redactDSN(*dsn))
		store = writer
	} else {
		mem := loadtest.NewMemStore(*simLatency)
		mem.Seed(*tenant, cachestore.Match{
			PromptHash: "loadtest-seed",
			Response:   "loadtest cached response",
			Similarity: 0.97,
		})
		_, _ = fmt.Fprintf(stdout, "loadtest: PGVECTOR_DSN not set -- running against an in-memory simulated store (sim-latency=%s), NOT a live pgvector instance\n", *simLatency)
		store = mem
	}

	result, err := loadtest.Run(ctx, cfg, store)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "loadtest: %v\n", err)
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "requests=%d errors=%d hits=%d misses=%d achieved_qps=%.1f\n",
		result.Requests, result.Errors, result.Hits, result.Misses, result.Achieved)
	_, _ = fmt.Fprintf(stdout, "p50=%s p95=%s p99=%s\n", result.P50, result.P95, result.P99)

	return 0
}

// syntheticEmbedding returns a deterministic embedder.Dimension-length
// vector, long enough to satisfy cachestore.WriteEntries's dimension
// check when running against a real Postgres instance. The values
// themselves are arbitrary -- the load test measures the index's query
// latency, not recall, so any well-formed vector exercises the same
// code path a real embedding would.
func syntheticEmbedding() []float32 {
	v := make([]float32, embedder.Dimension)
	for i := range v {
		v[i] = float32(i%1000) / 1000.0
	}
	return v
}

// redactDSN hides a DSN's password (if any) before printing it to stdout,
// so a load test run's output never leaks credentials into logs.
func redactDSN(dsn string) string {
	schemeEnd := strings.Index(dsn, "://")
	if schemeEnd < 0 {
		return dsn
	}
	userinfoStart := schemeEnd + len("://")
	at := strings.Index(dsn[userinfoStart:], "@")
	if at < 0 {
		return dsn
	}
	at += userinfoStart
	colon := strings.Index(dsn[userinfoStart:at], ":")
	if colon < 0 {
		return dsn
	}
	colon += userinfoStart
	return dsn[:colon+1] + "***" + dsn[at:]
}
