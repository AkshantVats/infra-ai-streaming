// SPDX-License-Identifier: MIT

// Package worker is the embedding worker DESIGN.md §1 places ahead of the
// cache lookup: it takes pending prompts, computes each one's prompt_hash
// (pkg/prompthash), embeds them in batches of BatchSize (pkg/embedder),
// and persists the results (pkg/cachestore). Idempotency has two layers:
// this package dedups by prompt_hash within a single Run so a duplicate
// prompt in the same input never triggers a second embedding API call,
// and cachestore.Writer's ON CONFLICT DO NOTHING makes a duplicate across
// separate Run invocations a no-op at the database.
package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/akshantvats/semantic-cache-engine/pkg/cachestore"
	"github.com/akshantvats/semantic-cache-engine/pkg/embedder"
	"github.com/akshantvats/semantic-cache-engine/pkg/prompthash"
)

// BatchSize is the embedding API batch size DESIGN.md's Day 61 plan item
// fixes at 32.
const BatchSize = 32

// PendingPrompt is one (tenant, prompt, response) tuple awaiting
// embedding -- the response a prompt produced, which is what a cache hit
// later serves back per DESIGN.md §1's "Serve cached response" path.
type PendingPrompt struct {
	TenantID string
	Prompt   string
	Response string
}

// Result summarizes one Run call for logging/observability: how many
// prompts came in, how many were skipped as in-run duplicates, and how
// many distinct entries were actually embedded and written.
type Result struct {
	Received   int
	Duplicates int
	Written    int
}

// Run dedups prompts by (tenant_id, prompt_hash), embeds the remaining
// ones in groups of BatchSize, and writes each batch's entries before
// moving to the next -- so a failure partway through a large input still
// leaves every already-completed batch durably cached instead of losing
// all progress.
func Run(ctx context.Context, prompts []PendingPrompt, emb embedder.Embedder, store cachestore.Writer) (Result, error) {
	type key struct{ tenant, hash string }

	seen := make(map[key]bool, len(prompts))
	type deduped struct {
		PendingPrompt
		hash string
	}
	unique := make([]deduped, 0, len(prompts))
	duplicates := 0

	for _, p := range prompts {
		h := prompthash.Hash(p.Prompt)
		k := key{p.TenantID, h}
		if seen[k] {
			duplicates++
			continue
		}
		seen[k] = true
		unique = append(unique, deduped{p, h})
	}

	written := 0
	now := time.Now().UTC()

	for start := 0; start < len(unique); start += BatchSize {
		end := start + BatchSize
		if end > len(unique) {
			end = len(unique)
		}
		batch := unique[start:end]

		texts := make([]string, len(batch))
		for i, d := range batch {
			texts[i] = d.Prompt
		}

		vectors, err := emb.Embed(ctx, texts)
		if err != nil {
			return Result{Received: len(prompts), Duplicates: duplicates, Written: written}, fmt.Errorf("worker: embed batch [%d:%d]: %w", start, end, err)
		}
		if len(vectors) != len(batch) {
			return Result{Received: len(prompts), Duplicates: duplicates, Written: written}, fmt.Errorf("worker: embed batch [%d:%d]: got %d vectors for %d prompts", start, end, len(vectors), len(batch))
		}

		entries := make([]cachestore.Entry, len(batch))
		for i, d := range batch {
			entries[i] = cachestore.Entry{
				TenantID:   d.TenantID,
				PromptHash: d.hash,
				Embedding:  vectors[i],
				Response:   d.Response,
				CreatedAt:  now,
			}
		}

		if err := store.WriteEntries(ctx, entries); err != nil {
			return Result{Received: len(prompts), Duplicates: duplicates, Written: written}, fmt.Errorf("worker: write batch [%d:%d]: %w", start, end, err)
		}
		written += len(entries)
	}

	return Result{Received: len(prompts), Duplicates: duplicates, Written: written}, nil
}
