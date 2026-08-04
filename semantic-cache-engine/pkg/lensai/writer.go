// SPDX-License-Identifier: MIT

// Package lensai dual-writes cache hits onto LensAI's existing /ingest
// pipeline as DESIGN.md §5 specifies: source="cache_hit", trace_id set to
// the matched entry's prompt_hash, latency_ms set to the cache lookup
// latency (not the original inference's), and cost_usd=0 since a hit
// performs no model call. This follows the same envelope and injection
// shape tool-call-analyzer/pkg/lensai.Writer already established for its
// own source="tool_call" dual-write, so cache_hit is additive to a
// pattern this repo has two producers of already (DESIGN.md §5).
package lensai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SourceCacheHit is the InferenceEvent.source value DESIGN.md §5 adds for
// semantic-cache-engine hits, alongside the already-live "inference",
// "benchmark_run", and "tool_call" values on the same field.
const SourceCacheHit = "cache_hit"

// Event mirrors ingestion/src/handlers/event.rs::InferenceEvent's wire
// format, matching tool-call-analyzer/pkg/lensai.Event's field set so a
// Go HTTP client can POST straight to LensAI's existing /ingest endpoint
// without a separate envelope type per producer.
type Event struct {
	EventID         string  `json:"event_id,omitempty"`
	TenantID        string  `json:"tenant_id"`
	ModelID         string  `json:"model_id"`
	TraceID         string  `json:"trace_id,omitempty"`
	TimestampUnixMs uint64  `json:"timestamp_unix_ms"`
	LatencyMs       uint32  `json:"latency_ms"`
	CostUSD         float64 `json:"cost_usd"`
	Status          string  `json:"status,omitempty"`
	Source          string  `json:"source,omitempty"`
}

type ingestRequest struct {
	Events []Event `json:"events"`
}

// tenantHeader is the header LensAI's ingest handler reads the tenant
// from when the batch itself doesn't carry one out of band (see
// ingestion/src/handlers/ingest.rs::TENANT_HEADER).
const tenantHeader = "X-Tenant-ID"

// Writer posts cache_hit events to a LensAI ingest endpoint over HTTP.
type Writer struct {
	ingestURL  string
	httpClient *http.Client
}

// New creates a Writer targeting ingestURL (e.g. "http://localhost:8080/ingest").
func New(ingestURL string) *Writer {
	return &Writer{
		ingestURL:  ingestURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// NewWithClient creates a Writer with an injected HTTP client, so tests
// run against an httptest.Server instead of a real ingest endpoint.
func NewWithClient(ingestURL string, client *http.Client) *Writer {
	return &Writer{ingestURL: ingestURL, httpClient: client}
}

// EmitCacheHit posts a single cache_hit event for a lookup that matched
// matchedPromptHash, took lookupLatency, and belongs to tenantID.
// modelID identifies the cache lookup path itself (not the original
// inference's model) so the event is attributable in a model-scoped
// Grafana panel; DESIGN.md §5 does not fix a value for this field, so
// callers pass whatever value distinguishes "served from cache" rows.
func (w *Writer) EmitCacheHit(ctx context.Context, tenantID, modelID, matchedPromptHash string, lookupLatency time.Duration) error {
	if tenantID == "" {
		return fmt.Errorf("lensai: tenant_id is required")
	}
	if matchedPromptHash == "" {
		return fmt.Errorf("lensai: matched prompt hash is required")
	}

	evt := Event{
		TenantID:        tenantID,
		ModelID:         modelID,
		TraceID:         matchedPromptHash,
		TimestampUnixMs: uint64(time.Now().UnixMilli()),
		LatencyMs:       uint32(lookupLatency.Milliseconds()),
		CostUSD:         0,
		Status:          "ok",
		Source:          SourceCacheHit,
	}

	payload, err := json.Marshal(ingestRequest{Events: []Event{evt}})
	if err != nil {
		return fmt.Errorf("lensai: marshal failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.ingestURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("lensai: build request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(tenantHeader, tenantID)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("lensai: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("lensai: unexpected status %d", resp.StatusCode)
	}
	return nil
}
