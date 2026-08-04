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

// TestFindExactAndFindNearest writes one entry, then exercises both read
// paths against a live pgvector instance: the exact prompt_hash fast path
// (§2), and the hnsw nearest-neighbor search (§1) using the same
// embedding, which should report similarity 1.0 -- the identical vector
// compared against itself.
func TestFindExactAndFindNearest(t *testing.T) {
	dsn := postgresDSN(t)
	ctx := context.Background()

	w, err := NewPostgresWriter(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPostgresWriter: %v", err)
	}
	defer w.Close()

	embedding := make([]float32, embedder.Dimension)
	embedding[0] = 1.0

	entry := Entry{
		TenantID:   "test-tenant-lookup",
		PromptHash: "lookup-test-hash",
		Embedding:  embedding,
		Response:   "the cached response",
		CreatedAt:  time.Now().UTC(),
	}
	if err := w.WriteEntries(ctx, []Entry{entry}); err != nil {
		t.Fatalf("WriteEntries: %v", err)
	}

	exact, ok, err := w.FindExact(ctx, entry.TenantID, entry.PromptHash)
	if err != nil {
		t.Fatalf("FindExact: %v", err)
	}
	if !ok {
		t.Fatal("FindExact: expected a hit, got none")
	}
	if exact.Response != entry.Response || exact.Similarity != 1.0 {
		t.Errorf("FindExact = %+v, want Response=%q Similarity=1.0", exact, entry.Response)
	}

	nearest, ok, err := w.FindNearest(ctx, entry.TenantID, embedding)
	if err != nil {
		t.Fatalf("FindNearest: %v", err)
	}
	if !ok {
		t.Fatal("FindNearest: expected a hit, got none")
	}
	if nearest.PromptHash != entry.PromptHash {
		t.Errorf("FindNearest.PromptHash = %q, want %q", nearest.PromptHash, entry.PromptHash)
	}
	if nearest.Similarity < 0.999 {
		t.Errorf("FindNearest.Similarity = %v, want ~1.0 for an identical vector", nearest.Similarity)
	}

	_, ok, err = w.FindNearest(ctx, "tenant-with-no-entries", embedding)
	if err != nil {
		t.Fatalf("FindNearest (empty tenant): %v", err)
	}
	if ok {
		t.Error("FindNearest: expected no hit for a tenant with no cached entries")
	}
}
