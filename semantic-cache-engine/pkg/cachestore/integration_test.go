// SPDX-License-Identifier: MIT

//go:build integration

package cachestore

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/akshantvats/semantic-cache-engine/pkg/embedder"
)

func postgresDSN(t *testing.T) string {
	t.Helper()
	v := os.Getenv("PGVECTOR_DSN")
	if v == "" {
		t.Skip("set PGVECTOR_DSN to run pgvector integration tests (requires the vector extension, see schema/001_semantic_cache_entries.sql)")
	}
	return v
}

// TestNewPostgresWriterConnects verifies that NewPostgresWriter dials and
// pings a live database.
func TestNewPostgresWriterConnects(t *testing.T) {
	dsn := postgresDSN(t)
	ctx := context.Background()

	w, err := NewPostgresWriter(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPostgresWriter: %v", err)
	}
	defer w.Close()
}

// TestWriteEntriesIsIdempotentOnPromptHash writes the same (tenant_id,
// prompt_hash) entry twice and verifies the second write is a no-op --
// the property pkg/worker's idempotency guarantee ultimately rests on.
func TestWriteEntriesIsIdempotentOnPromptHash(t *testing.T) {
	dsn := postgresDSN(t)
	ctx := context.Background()

	w, err := NewPostgresWriter(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPostgresWriter: %v", err)
	}
	defer w.Close()

	entry := Entry{
		TenantID:   "test-tenant",
		PromptHash: "integration-test-hash",
		Embedding:  make([]float32, embedder.Dimension),
		Response:   "first write",
		CreatedAt:  time.Now().UTC(),
	}

	if err := w.WriteEntries(ctx, []Entry{entry}); err != nil {
		t.Fatalf("first WriteEntries: %v", err)
	}
	entry.Response = "second write, should not land"
	if err := w.WriteEntries(ctx, []Entry{entry}); err != nil {
		t.Fatalf("second WriteEntries: %v", err)
	}

	var count int
	if err := w.pool.QueryRow(ctx, `SELECT count(*) FROM semantic_cache_entries WHERE tenant_id = $1 AND prompt_hash = $2`,
		entry.TenantID, entry.PromptHash).Scan(&count); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 row after two writes of the same prompt_hash, got %d", count)
	}
}
