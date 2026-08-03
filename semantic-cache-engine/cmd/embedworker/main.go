// SPDX-License-Identifier: MIT

// Command embedworker is the CLI entry point for semantic-cache-engine's
// Day 61 embedding worker: it reads pending prompts from a JSON Lines
// file, embeds them via OpenAI (pkg/embedder), and writes the results to
// a pgvector-backed Postgres database (pkg/cachestore) through
// pkg/worker's batching and idempotency logic.
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
	"github.com/akshantvats/semantic-cache-engine/pkg/embedder"
	"github.com/akshantvats/semantic-cache-engine/pkg/worker"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run implements the CLI and returns the process exit code, kept separate
// from main so it is testable without exec'ing a built binary (same shape
// as agent-replay-engine/cmd/traceforge).
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("embedworker", flag.ContinueOnError)
	fs.SetOutput(stderr)
	inputPath := fs.String("input", "", "path to a JSON Lines file of pending prompts: {\"tenant_id\",\"prompt\",\"response\"} per line")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *inputPath == "" {
		_, _ = fmt.Fprintln(stderr, "embedworker: --input is required")
		return 2
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		_, _ = fmt.Fprintln(stderr, "embedworker: OPENAI_API_KEY is required")
		return 2
	}
	dsn := os.Getenv("PGVECTOR_DSN")
	if dsn == "" {
		_, _ = fmt.Fprintln(stderr, "embedworker: PGVECTOR_DSN is required")
		return 2
	}

	f, err := os.Open(*inputPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "embedworker: open %s: %v\n", *inputPath, err)
		return 1
	}
	defer f.Close()

	prompts, err := parsePrompts(f)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "embedworker: parse %s: %v\n", *inputPath, err)
		return 1
	}

	ctx := context.Background()

	emb, err := embedder.New(apiKey)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "embedworker: %v\n", err)
		return 1
	}

	store, err := cachestore.NewPostgresWriter(ctx, dsn)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "embedworker: %v\n", err)
		return 1
	}
	defer store.Close()

	result, err := worker.Run(ctx, prompts, emb, store)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "embedworker: %v\n", err)
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "received=%d duplicates=%d written=%d\n", result.Received, result.Duplicates, result.Written)
	return 0
}

// promptLine is one JSON Lines record in the --input file.
type promptLine struct {
	TenantID string `json:"tenant_id"`
	Prompt   string `json:"prompt"`
	Response string `json:"response"`
}

// parsePrompts reads newline-delimited JSON records into worker.PendingPrompt,
// rejecting any line missing tenant_id or prompt -- both are required for
// the entry's primary key (cachestore.Entry.TenantID, prompt_hash).
func parsePrompts(r io.Reader) ([]worker.PendingPrompt, error) {
	var out []worker.PendingPrompt
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}
		var pl promptLine
		if err := json.Unmarshal([]byte(line), &pl); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
		if pl.TenantID == "" || pl.Prompt == "" {
			return nil, fmt.Errorf("line %d: tenant_id and prompt are required", lineNum)
		}
		out = append(out, worker.PendingPrompt{TenantID: pl.TenantID, Prompt: pl.Prompt, Response: pl.Response})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
