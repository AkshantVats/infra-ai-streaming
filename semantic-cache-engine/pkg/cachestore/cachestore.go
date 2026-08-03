// SPDX-License-Identifier: MIT

// Package cachestore persists embedded prompts to the pgvector-backed
// semantic_cache_entries table DESIGN.md §2 defines (schema/001_semantic_cache_entries.sql
// in this module). Persistence is an injected Writer interface, the same
// shape agent-benchmark-runner/pkg/store uses for its ClickHouse writer,
// so pkg/worker's batching and idempotency logic is testable without a
// live Postgres instance.
package cachestore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/akshantvats/semantic-cache-engine/pkg/embedder"
)

// Entry is one row of semantic_cache_entries, matching DESIGN.md §2's
// schema column for column.
type Entry struct {
	TenantID   string
	PromptHash string
	Embedding  []float32
	Response   string
	CreatedAt  time.Time
}

// Writer persists Entries. It is an interface so callers that only need
// to exercise batching/idempotency (pkg/worker's tests) never have to
// stand up Postgres, mirroring agent-benchmark-runner/pkg/store.Writer.
type Writer interface {
	WriteEntries(ctx context.Context, entries []Entry) error
}

// PostgresWriter writes Entries to a pgvector-enabled Postgres database
// over github.com/jackc/pgx/v5.
type PostgresWriter struct {
	pool *pgxpool.Pool
}

// NewPostgresWriter dials dsn and pings it before returning, so connection
// failures surface at construction time rather than on the first
// WriteEntries call -- the same fail-fast shape as
// agent-benchmark-runner/pkg/store.NewClickHouseWriter.
func NewPostgresWriter(ctx context.Context, dsn string) (*PostgresWriter, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("cachestore: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("cachestore: ping: %w", err)
	}
	return &PostgresWriter{pool: pool}, nil
}

// Close releases the underlying connection pool.
func (w *PostgresWriter) Close() {
	w.pool.Close()
}

const insertEntrySQL = `INSERT INTO semantic_cache_entries (tenant_id, prompt_hash, embedding, response, created_at, last_hit_at)
VALUES ($1, $2, $3, $4, $5, $5)
ON CONFLICT (tenant_id, prompt_hash) DO NOTHING`

// WriteEntries upserts entries into semantic_cache_entries. The insert is
// ON CONFLICT ... DO NOTHING on (tenant_id, prompt_hash) -- the same
// primary key DESIGN.md §2 defines -- so re-embedding an already-cached
// prompt is a true no-op rather than overwriting created_at or the stored
// response. This is what makes the embedding worker (pkg/worker)
// idempotent on prompt_hash: replaying the same input twice writes the
// row once.
//
// Entries within a single call are written inside one transaction so a
// batch either lands completely or not at all; a partial batch write
// would leave prompt_hash entries embedded but not queryable, silently
// defeating the exact-dup fast path DESIGN.md §2 describes.
func (w *PostgresWriter) WriteEntries(ctx context.Context, entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("cachestore: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after a successful Commit

	for _, e := range entries {
		if len(e.Embedding) != embedder.Dimension {
			return fmt.Errorf("cachestore: entry for %s/%s has %d dims, want %d", e.TenantID, e.PromptHash, len(e.Embedding), embedder.Dimension)
		}
		createdAt := e.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		if _, err := tx.Exec(ctx, insertEntrySQL, e.TenantID, e.PromptHash, vectorLiteral(e.Embedding), e.Response, createdAt); err != nil {
			return fmt.Errorf("cachestore: insert %s/%s: %w", e.TenantID, e.PromptHash, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("cachestore: commit tx: %w", err)
	}
	return nil
}

// vectorLiteral renders an embedding as pgvector's text input format,
// e.g. "[0.1,0.2,0.3]". Building the literal here instead of depending on
// a pgvector-specific driver extension keeps this package's only
// dependency github.com/jackc/pgx/v5, matching the rest of this repo's
// preference for the plain driver (ClickHouseWriter, the Redis client in
// ingestion) over ORM-style wrappers.
func vectorLiteral(v []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%g", f)
	}
	b.WriteByte(']')
	return b.String()
}
