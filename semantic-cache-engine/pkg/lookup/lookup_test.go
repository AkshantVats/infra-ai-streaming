// SPDX-License-Identifier: MIT

package lookup

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/akshantvats/semantic-cache-engine/pkg/cachestore"
	"github.com/akshantvats/semantic-cache-engine/pkg/config"
	"github.com/akshantvats/semantic-cache-engine/pkg/prompthash"
)

// fakeStore is an in-memory cachestore.Reader keyed by (tenant, hash),
// plus a single fixed "nearest" candidate per tenant for the semantic
// path, so tests can drive FindExact and FindNearest independently
// without a live Postgres+pgvector instance.
type fakeStore struct {
	exact   map[string]cachestore.Match // key: tenant + "/" + hash
	nearest map[string]cachestore.Match // key: tenant
}

func (f *fakeStore) FindExact(_ context.Context, tenantID, promptHash string) (cachestore.Match, bool, error) {
	m, ok := f.exact[tenantID+"/"+promptHash]
	return m, ok, nil
}

func (f *fakeStore) FindNearest(_ context.Context, tenantID string, _ []float32) (cachestore.Match, bool, error) {
	m, ok := f.nearest[tenantID]
	return m, ok, nil
}

type fakeEmbedder struct {
	err error
}

func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{0.1, 0.2, 0.3}
	}
	return out, nil
}

type fakeEmitter struct {
	calls int
	err   error
}

func (f *fakeEmitter) EmitCacheHit(_ context.Context, _, _, _ string, _ time.Duration) error {
	f.calls++
	return f.err
}

func TestLookupExactFastPathSkipsEmbedAndEmits(t *testing.T) {
	prompt := "Summarize this document"
	hash := prompthash.Hash(prompt)
	store := &fakeStore{exact: map[string]cachestore.Match{
		"tenant-a/" + hash: {PromptHash: hash, Response: "cached answer", Similarity: 1.0},
	}}
	emb := &fakeEmbedder{err: errors.New("embed should not be called on the exact fast path")}
	emitter := &fakeEmitter{}

	result, err := Lookup(context.Background(), "tenant-a", prompt, config.Config{}, emb, store, emitter)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !result.Hit || result.Response != "cached answer" {
		t.Fatalf("Lookup = %+v, want a hit with the cached answer", result)
	}
	if result.Similarity != 1.0 {
		t.Errorf("Similarity = %v, want 1.0 for exact match", result.Similarity)
	}
	if emitter.calls != 1 {
		t.Errorf("emitter called %d times, want 1", emitter.calls)
	}
}

func TestLookupSemanticHitAboveThreshold(t *testing.T) {
	store := &fakeStore{
		exact:   map[string]cachestore.Match{},
		nearest: map[string]cachestore.Match{"tenant-a": {PromptHash: "h2", Response: "semantic answer", Similarity: 0.95}},
	}
	cfg := config.Config{Default: config.TenantConfig{SimilarityThreshold: 0.92}}

	result, err := Lookup(context.Background(), "tenant-a", "some prompt", cfg, &fakeEmbedder{}, store, nil)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !result.Hit || result.Response != "semantic answer" {
		t.Fatalf("Lookup = %+v, want a semantic hit", result)
	}
	if result.EmitErr != nil {
		t.Errorf("EmitErr = %v, want nil when emitter is nil", result.EmitErr)
	}
}

func TestLookupSemanticMissBelowThreshold(t *testing.T) {
	store := &fakeStore{
		exact:   map[string]cachestore.Match{},
		nearest: map[string]cachestore.Match{"tenant-a": {PromptHash: "h2", Response: "semantic answer", Similarity: 0.80}},
	}
	cfg := config.Config{Default: config.TenantConfig{SimilarityThreshold: 0.92}}

	result, err := Lookup(context.Background(), "tenant-a", "some prompt", cfg, &fakeEmbedder{}, store, nil)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if result.Hit {
		t.Fatalf("Lookup = %+v, want a miss for similarity 0.80 < threshold 0.92", result)
	}
	if result.Threshold != 0.92 {
		t.Errorf("Threshold = %v, want 0.92", result.Threshold)
	}
}

func TestLookupMissWithNoCachedEntries(t *testing.T) {
	store := &fakeStore{exact: map[string]cachestore.Match{}, nearest: map[string]cachestore.Match{}}
	result, err := Lookup(context.Background(), "tenant-a", "some prompt", config.Config{}, &fakeEmbedder{}, store, nil)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if result.Hit {
		t.Fatalf("Lookup = %+v, want a miss when the tenant has no cached entries", result)
	}
}

func TestLookupHitSurvivesEmitFailure(t *testing.T) {
	store := &fakeStore{
		exact:   map[string]cachestore.Match{},
		nearest: map[string]cachestore.Match{"tenant-a": {PromptHash: "h2", Response: "semantic answer", Similarity: 0.95}},
	}
	cfg := config.Config{Default: config.TenantConfig{SimilarityThreshold: 0.92}}
	emitter := &fakeEmitter{err: errors.New("ingest endpoint unreachable")}

	result, err := Lookup(context.Background(), "tenant-a", "some prompt", cfg, &fakeEmbedder{}, store, emitter)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !result.Hit || result.Response != "semantic answer" {
		t.Fatalf("Lookup = %+v, want the hit to still be served despite the emit failure", result)
	}
	if result.EmitErr == nil {
		t.Error("EmitErr = nil, want the emitter's error to be surfaced")
	}
}

func TestLookupRequiresTenantAndPrompt(t *testing.T) {
	store := &fakeStore{}
	if _, err := Lookup(context.Background(), "", "prompt", config.Config{}, &fakeEmbedder{}, store, nil); err == nil {
		t.Error("expected error for missing tenant_id, got nil")
	}
	if _, err := Lookup(context.Background(), "tenant-a", "", config.Config{}, &fakeEmbedder{}, store, nil); err == nil {
		t.Error("expected error for missing prompt, got nil")
	}
}
