// SPDX-License-Identifier: MIT

// Package embedder implements DESIGN.md §1's "pluggable interface, not a
// hard dependency" boundary: an Embedder interface any embedding source
// can satisfy, plus OpenAIEmbedder, a concrete implementation over
// OpenAI's /v1/embeddings endpoint using the text-embedding-3-small model
// DESIGN.md §1 names as the assumed N=1536 dimension source.
package embedder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Dimension is the embedding vector length DESIGN.md §1 and §2 fix as a
// build-time constant (matching text-embedding-3-small's default output
// size), so semantic_cache_entries.embedding's vector(1536) column and
// every Embedder implementation agree without either side guessing the
// other's size at runtime.
const Dimension = 1536

// Embedder turns a batch of prompt texts into one embedding vector per
// text, in the same order as the input. It is an interface, not a
// concrete OpenAI type, so pkg/worker's batching and idempotency logic can
// be tested without an API key or network access -- the same reason
// pkg/store.Writer is an interface in agent-benchmark-runner.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// defaultModel is OpenAI's small embedding model, chosen in DESIGN.md §1
// as the concrete default for the assumed N=1536 dimension.
const defaultModel = "text-embedding-3-small"

const defaultBaseURL = "https://api.openai.com/v1"

// OpenAIEmbedder calls OpenAI's /v1/embeddings endpoint. baseURL and
// httpClient are overridable (NewWithClient) so tests run against an
// httptest.Server instead of the real API, the same injection pattern
// agent-benchmark-runner/pkg/lensai.Writer uses for its ingest HTTP calls.
type OpenAIEmbedder struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// New creates an OpenAIEmbedder targeting the real OpenAI API with
// defaultModel. apiKey must be non-empty -- OpenAI rejects unauthenticated
// requests, and failing fast at construction is cheaper than failing on
// the first batch.
func New(apiKey string) (*OpenAIEmbedder, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("embedder: API key is required")
	}
	return &OpenAIEmbedder{
		apiKey:     apiKey,
		model:      defaultModel,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// NewWithClient creates an OpenAIEmbedder against an overridden baseURL
// and http.Client, for testing against an httptest.Server without hitting
// OpenAI or spending API budget.
func NewWithClient(apiKey, model, baseURL string, client *http.Client) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		apiKey:     apiKey,
		model:      model,
		baseURL:    baseURL,
		httpClient: client,
	}
}

type embeddingsRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingsResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Embed posts texts to OpenAI's /v1/embeddings endpoint in a single
// request and returns one vector per text, reordered by the response's
// Index field so the result always matches the input order even if the
// API were to return entries out of order.
//
// Callers are responsible for batching (pkg/worker splits into groups of
// 32 per DESIGN.md's batch size) -- this method makes exactly one HTTP
// call per invocation, so it never silently re-batches a caller's batch.
func (e *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	payload, err := json.Marshal(embeddingsRequest{Model: e.model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("embedder: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("embedder: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedder: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("embedder: read response: %w", err)
	}

	var parsed embeddingsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("embedder: decode response (status %d): %w", resp.StatusCode, err)
	}

	if resp.StatusCode != http.StatusOK {
		if parsed.Error != nil {
			return nil, fmt.Errorf("embedder: OpenAI error (status %d): %s", resp.StatusCode, parsed.Error.Message)
		}
		return nil, fmt.Errorf("embedder: unexpected status %d", resp.StatusCode)
	}

	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("embedder: expected %d embeddings, got %d", len(texts), len(parsed.Data))
	}

	out := make([][]float32, len(texts))
	for _, d := range parsed.Data {
		if d.Index < 0 || d.Index >= len(out) {
			return nil, fmt.Errorf("embedder: response index %d out of range for %d inputs", d.Index, len(texts))
		}
		out[d.Index] = d.Embedding
	}
	for i, v := range out {
		if v == nil {
			return nil, fmt.Errorf("embedder: no embedding returned for input index %d", i)
		}
	}
	return out, nil
}
