// SPDX-License-Identifier: MIT

package worker

import (
	"context"
	"fmt"
	"testing"

	"github.com/akshantvats/semantic-cache-engine/pkg/cachestore"
	"github.com/akshantvats/semantic-cache-engine/pkg/embedder"
)

// fakeEmbedder returns a fixed-length zero vector per input and counts how
// many times Embed was called, so tests can assert on batching without an
// API key.
type fakeEmbedder struct {
	calls    int
	texts    [][]string
	failFrom int // if > 0, Embed fails once total calls reaches this count
}

func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	f.calls++
	f.texts = append(f.texts, append([]string{}, texts...))
	if f.failFrom > 0 && f.calls >= f.failFrom {
		return nil, fmt.Errorf("simulated embed failure")
	}
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = make([]float32, embedder.Dimension)
	}
	return out, nil
}

// fakeStore records every WriteEntries call in memory.
type fakeStore struct {
	written []cachestore.Entry
	failOn  int // if > 0, WriteEntries fails on this call number
	calls   int
}

func (f *fakeStore) WriteEntries(_ context.Context, entries []cachestore.Entry) error {
	f.calls++
	if f.failOn > 0 && f.calls == f.failOn {
		return fmt.Errorf("simulated write failure")
	}
	f.written = append(f.written, entries...)
	return nil
}

func TestRunSplitsIntoBatchesOf32(t *testing.T) {
	prompts := make([]PendingPrompt, 70)
	for i := range prompts {
		prompts[i] = PendingPrompt{TenantID: "t1", Prompt: fmt.Sprintf("prompt %d", i), Response: "r"}
	}

	emb := &fakeEmbedder{}
	store := &fakeStore{}

	result, err := Run(context.Background(), prompts, emb, store)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if emb.calls != 3 {
		t.Fatalf("expected 3 embed calls (32+32+6), got %d", emb.calls)
	}
	if len(emb.texts[0]) != 32 || len(emb.texts[1]) != 32 || len(emb.texts[2]) != 6 {
		t.Fatalf("unexpected batch sizes: %v", []int{len(emb.texts[0]), len(emb.texts[1]), len(emb.texts[2])})
	}
	if result.Written != 70 {
		t.Fatalf("expected 70 written, got %d", result.Written)
	}
}

func TestRunDedupsSamePromptWithinRun(t *testing.T) {
	prompts := []PendingPrompt{
		{TenantID: "t1", Prompt: "summarize this doc", Response: "r1"},
		{TenantID: "t1", Prompt: "Summarize   this doc", Response: "r2"}, // normalizes to the same hash
		{TenantID: "t2", Prompt: "summarize this doc", Response: "r3"},   // different tenant, not a dup
	}

	emb := &fakeEmbedder{}
	store := &fakeStore{}

	result, err := Run(context.Background(), prompts, emb, store)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Duplicates != 1 {
		t.Fatalf("expected 1 duplicate, got %d", result.Duplicates)
	}
	if result.Written != 2 {
		t.Fatalf("expected 2 distinct entries written, got %d", result.Written)
	}
	if emb.calls != 1 {
		t.Fatalf("expected the embedder to be called once (batch of 2 unique prompts), got %d calls", emb.calls)
	}
}

func TestRunPropagatesEmbedError(t *testing.T) {
	prompts := []PendingPrompt{{TenantID: "t1", Prompt: "a", Response: "r"}}
	emb := &fakeEmbedder{failFrom: 1}
	store := &fakeStore{}

	_, err := Run(context.Background(), prompts, emb, store)
	if err == nil {
		t.Fatal("expected an error when the embedder fails")
	}
	if len(store.written) != 0 {
		t.Fatalf("expected nothing written after an embed failure, got %d", len(store.written))
	}
}

func TestRunPreservesEarlierBatchesOnLaterFailure(t *testing.T) {
	prompts := make([]PendingPrompt, 40)
	for i := range prompts {
		prompts[i] = PendingPrompt{TenantID: "t1", Prompt: fmt.Sprintf("prompt %d", i), Response: "r"}
	}
	emb := &fakeEmbedder{}
	store := &fakeStore{failOn: 2} // second batch's write fails

	result, err := Run(context.Background(), prompts, emb, store)
	if err == nil {
		t.Fatal("expected an error from the second batch write")
	}
	if result.Written != 32 {
		t.Fatalf("expected the first batch's 32 entries to have been written before the failure, got %d", result.Written)
	}
	if len(store.written) != 32 {
		t.Fatalf("expected the store to retain the first batch's 32 entries, got %d", len(store.written))
	}
}

func TestRunEmptyInput(t *testing.T) {
	emb := &fakeEmbedder{}
	store := &fakeStore{}

	result, err := Run(context.Background(), nil, emb, store)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Written != 0 || emb.calls != 0 {
		t.Fatalf("expected no work for empty input, got written=%d calls=%d", result.Written, emb.calls)
	}
}
