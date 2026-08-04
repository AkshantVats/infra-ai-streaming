// SPDX-License-Identifier: MIT

// Command cachelookup is the CLI entry point for semantic-cache-engine's
// Day 62 cache lookup path (pkg/lookup): it reads pending queries from a
// JSON Lines file, runs each through the exact-dup fast path and the
// pgvector nearest-neighbor search, and prints a hit/miss line per query.
// This is the read-side counterpart to Day 61's cmd/embedworker, which
// populates the same table this command reads from.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/akshantvats/semantic-cache-engine/pkg/cachestore"
	"github.com/akshantvats/semantic-cache-engine/pkg/config"
	"github.com/akshantvats/semantic-cache-engine/pkg/embedder"
	"github.com/akshantvats/semantic-cache-engine/pkg/lensai"
	"github.com/akshantvats/semantic-cache-engine/pkg/lookup"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run implements the CLI and returns the process exit code, kept separate
// from main for testability without exec'ing a built binary (same shape
// as cmd/embedworker/main.go::run).
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cachelookup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	inputPath := fs.String("input", "", "path to a JSON Lines file of queries: {\"tenant_id\",\"prompt\"} per line")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *inputPath == "" {
		_, _ = fmt.Fprintln(stderr, "cachelookup: --input is required")
		return 2
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		_, _ = fmt.Fprintln(stderr, "cachelookup: OPENAI_API_KEY is required")
		return 2
	}
	dsn := os.Getenv("PGVECTOR_DSN")
	if dsn == "" {
		_, _ = fmt.Fprintln(stderr, "cachelookup: PGVECTOR_DSN is required")
		return 2
	}

	cfg := config.Config{}
	if path := os.Getenv("CACHE_CONFIG_PATH"); path != "" {
		loaded, err := config.Load(path)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "cachelookup: %v\n", err)
			return 1
		}
		cfg = loaded
	}

	var emitter lookup.EventEmitter
	if ingestURL := os.Getenv("LENSAI_INGEST_URL"); ingestURL != "" {
		emitter = lensai.New(ingestURL)
	}

	f, err := os.Open(*inputPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "cachelookup: open %s: %v\n", *inputPath, err)
		return 1
	}
	defer f.Close()

	queries, err := parseQueries(f)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "cachelookup: parse %s: %v\n", *inputPath, err)
		return 1
	}

	ctx := context.Background()

	emb, err := embedder.New(apiKey)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "cachelookup: %v\n", err)
		return 1
	}

	store, err := cachestore.NewPostgresWriter(ctx, dsn)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "cachelookup: %v\n", err)
		return 1
	}
	defer store.Close()

	hits, misses := 0, 0
	for _, q := range queries {
		result, err := lookup.Lookup(ctx, q.TenantID, q.Prompt, cfg, emb, store, emitter)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "cachelookup: lookup for tenant %s: %v\n", q.TenantID, err)
			return 1
		}
		if result.Hit {
			hits++
			_, _ = fmt.Fprintf(stdout, "HIT  tenant=%s similarity=%.4f threshold=%.4f matched=%s\n",
				q.TenantID, result.Similarity, result.Threshold, result.MatchedPromptHash)
			if result.EmitErr != nil {
				_, _ = fmt.Fprintf(stderr, "cachelookup: cache_hit event not emitted for tenant %s: %v\n", q.TenantID, result.EmitErr)
			}
		} else {
			misses++
			_, _ = fmt.Fprintf(stdout, "MISS tenant=%s threshold=%.4f\n", q.TenantID, result.Threshold)
		}
	}

	_, _ = fmt.Fprintf(stdout, "received=%d hits=%d misses=%d\n", len(queries), hits, misses)
	return 0
}

// queryLine is one JSON Lines record in the --input file.
type queryLine struct {
	TenantID string `json:"tenant_id"`
	Prompt   string `json:"prompt"`
}

func parseQueries(r io.Reader) ([]queryLine, error) {
	var out []queryLine
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}
		var ql queryLine
		if err := json.Unmarshal([]byte(line), &ql); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
		if ql.TenantID == "" || ql.Prompt == "" {
			return nil, fmt.Errorf("line %d: tenant_id and prompt are required", lineNum)
		}
		out = append(out, ql)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
