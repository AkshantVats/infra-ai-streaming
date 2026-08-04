// SPDX-License-Identifier: MIT

// This file adds the read side of semantic_cache_entries that DESIGN.md §1's
// cache lookup path needs: an exact prompt_hash fast path (skips the
// embedding call entirely for a byte-identical repeat prompt) and a
// nearest-neighbor semantic search over the hnsw index
// schema/001_semantic_cache_entries.sql already creates. Both live
// alongside cachestore.go's write path because they share the same
// pgxpool.Pool and Entry-shaped rows.
package cachestore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Match is a single cache lookup candidate: a stored response, the
// prompt_hash it was stored under, and how similar the query was to it.
// FindExact always reports Similarity 1.0 (byte-identical after
// normalization); FindNearest reports the pgvector cosine similarity
// (1 - cosine distance).
type Match struct {
	PromptHash string
	Response   string
	Similarity float64
}

// Reader looks up cache entries for a tenant. It is an interface,
// mirroring Writer, so pkg/lookup's threshold and event-emission logic
// is testable with a fake instead of a live Postgres+pgvector instance.
type Reader interface {
	FindExact(ctx context.Context, tenantID, promptHash string) (Match, bool, error)
	FindNearest(ctx context.Context, tenantID string, embedding []float32) (Match, bool, error)
}

const findExactSQL = `SELECT prompt_hash, response FROM semantic_cache_entries WHERE tenant_id = $1 AND prompt_hash = $2`

// FindExact looks up an entry by its exact prompt_hash -- DESIGN.md §2's
// "exact-dup fast path" -- so a byte-identical repeat prompt (after
// prompthash.Normalize) can hit without spending an embedding API call.
// A hit updates last_hit_at; a miss returns (Match{}, false, nil), not an
// error, since "not cached yet" is an expected outcome, not a failure.
func (w *PostgresWriter) FindExact(ctx context.Context, tenantID, promptHash string) (Match, bool, error) {
	var m Match
	err := w.pool.QueryRow(ctx, findExactSQL, tenantID, promptHash).Scan(&m.PromptHash, &m.Response)
	if errors.Is(err, pgx.ErrNoRows) {
		return Match{}, false, nil
	}
	if err != nil {
		return Match{}, false, fmt.Errorf("cachestore: find exact %s/%s: %w", tenantID, promptHash, err)
	}
	m.Similarity = 1.0
	if err := w.touchLastHit(ctx, tenantID, m.PromptHash); err != nil {
		return Match{}, false, fmt.Errorf("cachestore: touch last_hit_at for %s/%s: %w", tenantID, m.PromptHash, err)
	}
	return m, true, nil
}

// findNearestSQL orders by pgvector's cosine distance operator (<=>) and
// takes the single closest row within the tenant, using the hnsw index
// schema/001_semantic_cache_entries.sql defines on (embedding). Cosine
// similarity is 1 - cosine distance.
const findNearestSQL = `SELECT prompt_hash, response, 1 - (embedding <=> $2) AS similarity
FROM semantic_cache_entries
WHERE tenant_id = $1
ORDER BY embedding <=> $2
LIMIT 1`

// FindNearest returns the single closest entry to embedding within
// tenantID, regardless of how similar it is -- threshold comparison is
// the caller's job (pkg/lookup), not this method's, so a caller that
// wants to log "closest miss was 0.81" for tuning can still see it.
// A tenant with no cached entries yet returns (Match{}, false, nil).
func (w *PostgresWriter) FindNearest(ctx context.Context, tenantID string, embedding []float32) (Match, bool, error) {
	if len(embedding) == 0 {
		return Match{}, false, fmt.Errorf("cachestore: find nearest: embedding is empty")
	}
	var m Match
	err := w.pool.QueryRow(ctx, findNearestSQL, tenantID, vectorLiteral(embedding)).Scan(&m.PromptHash, &m.Response, &m.Similarity)
	if errors.Is(err, pgx.ErrNoRows) {
		return Match{}, false, nil
	}
	if err != nil {
		return Match{}, false, fmt.Errorf("cachestore: find nearest for tenant %s: %w", tenantID, err)
	}
	return m, true, nil
}

const touchLastHitSQL = `UPDATE semantic_cache_entries SET last_hit_at = now() WHERE tenant_id = $1 AND prompt_hash = $2`

// touchLastHit bumps last_hit_at on a cache hit. DESIGN.md §6 notes
// last_hit_at "does not extend the hard [TTL] ceiling" -- it exists for
// observability of hot entries, not for freshness -- so this update is
// best-effort bookkeeping alongside the read, not a correctness
// requirement of the lookup itself.
func (w *PostgresWriter) touchLastHit(ctx context.Context, tenantID, promptHash string) error {
	_, err := w.pool.Exec(ctx, touchLastHitSQL, tenantID, promptHash)
	return err
}
