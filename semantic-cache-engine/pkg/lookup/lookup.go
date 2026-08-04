// SPDX-License-Identifier: MIT

// Package lookup is DESIGN.md §1's cache lookup path: given a tenant and
// a prompt, check the exact-dup fast path (pkg/cachestore.Reader.FindExact,
// §2), fall back to embedding the prompt and running a nearest-neighbor
// search (FindNearest) against the tenant's configured similarity
// threshold (pkg/config, §3), and on a hit emit a cache_hit event
// (pkg/lensai, §5). A miss returns Result{Hit: false} so the caller can
// pass the prompt through to inference, per DESIGN.md §1's diagram.
package lookup

import (
	"context"
	"fmt"
	"time"

	"github.com/akshantvats/semantic-cache-engine/pkg/cachestore"
	"github.com/akshantvats/semantic-cache-engine/pkg/config"
	"github.com/akshantvats/semantic-cache-engine/pkg/embedder"
	"github.com/akshantvats/semantic-cache-engine/pkg/prompthash"
)

// EventEmitter is the subset of *lensai.Writer Lookup depends on, so
// tests can inject a fake instead of an httptest.Server. EmitCacheMiss
// exists alongside EmitCacheHit so a Lookup outcome is fully observable
// either way -- an event stream that only ever records hits has no
// denominator to compute a real hit rate from (pkg/analytics, Day 63).
type EventEmitter interface {
	EmitCacheHit(ctx context.Context, tenantID, modelID, matchedPromptHash string, lookupLatency time.Duration) error
	EmitCacheMiss(ctx context.Context, tenantID, modelID string, lookupLatency time.Duration) error
}

// Result is the outcome of a single Lookup call.
type Result struct {
	// Hit is true if either the exact-dup fast path or the semantic
	// search cleared the tenant's threshold. When Hit is false, the
	// caller passes the prompt through to inference (§1).
	Hit bool
	// Response is the cached completion to serve back. Only meaningful
	// when Hit is true.
	Response string
	// MatchedPromptHash is the prompt_hash of the entry that matched.
	MatchedPromptHash string
	// Similarity is the cosine similarity of the match: exactly 1.0 for
	// the exact-dup fast path, or the pgvector-computed value for a
	// semantic match. Meaningless when Hit is false.
	Similarity float64
	// Threshold is the tenant's configured threshold this lookup was
	// evaluated against, returned even on a miss so a caller can log
	// "closest candidate 0.81 vs threshold 0.92" for tuning.
	Threshold float64
	// EmitErr is non-nil when posting this outcome's event (cache_hit on
	// a hit, cache_miss on a miss) to LensAI failed. A failed emission
	// never changes Hit -- serving (or not serving) the cached response
	// is the correctness-critical half of a lookup, the event is an
	// observability side channel, so Lookup always returns the outcome
	// the store and threshold comparison actually produced.
	EmitErr error
}

// ModelID identifies the cache lookup path itself in emitted cache_hit
// events, distinguishing "served from cache" rows from rows carrying a
// real inference model_id in LensAI's ClickHouse table.
const ModelID = "semantic-cache-lookup"

// Lookup runs DESIGN.md §1's cache lookup path for a single prompt.
// emitter may be nil, in which case a hit skips event emission entirely
// (EmitErr stays nil) -- useful for callers that don't yet have a LensAI
// endpoint configured, e.g. local development.
func Lookup(ctx context.Context, tenantID, prompt string, cfg config.Config, emb embedder.Embedder, store cachestore.Reader, emitter EventEmitter) (Result, error) {
	if tenantID == "" {
		return Result{}, fmt.Errorf("lookup: tenant_id is required")
	}
	if prompt == "" {
		return Result{}, fmt.Errorf("lookup: prompt is required")
	}

	start := time.Now()
	threshold := cfg.Threshold(tenantID)
	hash := prompthash.Hash(prompt)

	if exact, ok, err := store.FindExact(ctx, tenantID, hash); err != nil {
		return Result{}, fmt.Errorf("lookup: exact-dup fast path: %w", err)
	} else if ok {
		return finishHit(ctx, tenantID, exact, threshold, start, emitter), nil
	}

	vectors, err := emb.Embed(ctx, []string{prompt})
	if err != nil {
		return Result{}, fmt.Errorf("lookup: embed prompt: %w", err)
	}
	if len(vectors) != 1 {
		return Result{}, fmt.Errorf("lookup: embed prompt: expected 1 vector, got %d", len(vectors))
	}

	nearest, ok, err := store.FindNearest(ctx, tenantID, vectors[0])
	if err != nil {
		return Result{}, fmt.Errorf("lookup: nearest-neighbor search: %w", err)
	}
	if !ok || nearest.Similarity < threshold {
		return finishMiss(ctx, tenantID, threshold, start, emitter), nil
	}

	return finishHit(ctx, tenantID, nearest, threshold, start, emitter), nil
}

// finishHit builds a hit Result and, if an emitter is configured, emits
// the DESIGN.md §5 cache_hit event carrying the lookup's total latency.
func finishHit(ctx context.Context, tenantID string, m cachestore.Match, threshold float64, start time.Time, emitter EventEmitter) Result {
	result := Result{
		Hit:               true,
		Response:          m.Response,
		MatchedPromptHash: m.PromptHash,
		Similarity:        m.Similarity,
		Threshold:         threshold,
	}
	if emitter == nil {
		return result
	}
	if err := emitter.EmitCacheHit(ctx, tenantID, ModelID, m.PromptHash, time.Since(start)); err != nil {
		result.EmitErr = fmt.Errorf("lookup: emit cache_hit event: %w", err)
	}
	return result
}

// finishMiss builds a miss Result and, if an emitter is configured, emits
// a cache_miss event carrying the lookup's total latency -- the
// denominator half of pkg/analytics's hit rate, alongside finishHit's
// numerator half.
func finishMiss(ctx context.Context, tenantID string, threshold float64, start time.Time, emitter EventEmitter) Result {
	result := Result{Hit: false, Threshold: threshold}
	if emitter == nil {
		return result
	}
	if err := emitter.EmitCacheMiss(ctx, tenantID, ModelID, time.Since(start)); err != nil {
		result.EmitErr = fmt.Errorf("lookup: emit cache_miss event: %w", err)
	}
	return result
}
