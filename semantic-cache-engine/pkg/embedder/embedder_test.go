// SPDX-License-Identifier: MIT

package embedder

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbedReordersByResponseIndex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req embeddingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(req.Input) != 2 {
			t.Fatalf("expected 2 inputs, got %d", len(req.Input))
		}
		// Respond with entries out of order to exercise the Index-based reorder.
		resp := embeddingsResponse{}
		resp.Data = []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		}{
			{Embedding: []float32{0.2}, Index: 1},
			{Embedding: []float32{0.1}, Index: 0},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	e := NewWithClient("test-key", "test-model", srv.URL, srv.Client())
	got, err := e.Embed(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(got) != 2 || got[0][0] != 0.1 || got[1][0] != 0.2 {
		t.Fatalf("unexpected order: %v", got)
	}
}

func TestEmbedEmptyInputIsNoop(t *testing.T) {
	e := NewWithClient("test-key", "test-model", "http://unused.invalid", http.DefaultClient)
	got, err := e.Embed(context.Background(), nil)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil result for empty input, got %v", got)
	}
}

func TestEmbedPropagatesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "invalid api key"},
		})
	}))
	defer srv.Close()

	e := NewWithClient("bad-key", "test-model", srv.URL, srv.Client())
	_, err := e.Embed(context.Background(), []string{"x"})
	if err == nil {
		t.Fatal("expected an error for a 401 response, got nil")
	}
}

func TestEmbedMismatchedResponseCountErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := embeddingsResponse{}
		resp.Data = []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		}{{Embedding: []float32{0.1}, Index: 0}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	e := NewWithClient("test-key", "test-model", srv.URL, srv.Client())
	_, err := e.Embed(context.Background(), []string{"a", "b"})
	if err == nil {
		t.Fatal("expected an error when the API returns fewer embeddings than requested")
	}
}

func TestNewRequiresAPIKey(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("expected an error for an empty API key")
	}
}
