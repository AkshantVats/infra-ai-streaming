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

// SourceCacheFeedback is the InferenceEvent.source value DESIGN.md §8's
// Day 63 implementation notes add: a user-reported thumbs-down on a
// specific cache hit, dual-written onto the same infra_ai.inference_events
// table cache_hit already uses (DESIGN.md §5's "one clearinghouse ledger"
// principle) rather than a second table that would need its own dashboard
// and its own join back to tenant/model data.
const SourceCacheFeedback = "cache_feedback"

// SourceCacheMiss is the InferenceEvent.source value DESIGN.md §8's Day 63
// implementation notes add: a lookup that found no candidate above the
// tenant's threshold. Without this value, hit rate (pkg/analytics) has no
// denominator -- counting only cache_hit events makes "hit rate" trend
// toward 100% by construction, since the event stream would only ever
// record successes. A miss costs no model call by itself (the caller
// falls through to a real inference, logged separately with its own
// source), so cache_miss exists purely to make lookup outcomes fully
// observable, not to represent spend.
const SourceCacheMiss = "cache_miss"

// StatusThumbsDown is the Event.Status value EmitCacheFeedback sets. It is
// the only feedback status this module emits today -- DESIGN.md §4's full
// design calls for a sampled human/LLM-judge review pass that could label
// both correct and incorrect hits, but the webhook this status backs
// (pkg/feedback) is deliberately the minimal real signal available without
// that pipeline: a user flagging a wrong answer, nothing more.
const StatusThumbsDown = "thumbs_down"

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

// EmitCacheMiss posts a single cache_miss event for a lookup that found no
// candidate above tenantID's threshold. It carries no trace_id -- there is
// no matched entry to trace back to -- and lookupLatency is the same
// lookup-timing value EmitCacheHit records, so hit and miss events are
// comparable on latency in a Grafana panel, not just on outcome.
func (w *Writer) EmitCacheMiss(ctx context.Context, tenantID, modelID string, lookupLatency time.Duration) error {
	if tenantID == "" {
		return fmt.Errorf("lensai: tenant_id is required")
	}

	evt := Event{
		TenantID:        tenantID,
		ModelID:         modelID,
		TimestampUnixMs: uint64(time.Now().UnixMilli()),
		LatencyMs:       uint32(lookupLatency.Milliseconds()),
		CostUSD:         0,
		Status:          "ok",
		Source:          SourceCacheMiss,
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

// EmitCacheFeedback posts a single cache_feedback event recording a
// thumbs-down on the cache hit that matched matchedPromptHash for
// tenantID. modelID identifies the lookup path the same way EmitCacheHit's
// modelID parameter does (callers pass pkg/lookup.ModelID), so a
// cache_feedback row and the cache_hit row it's flagging share the same
// model_id and are attributable to the same lookup path in a Grafana
// panel. It shares EmitCacheHit's envelope and injection shape -- same
// tenant header, same ingest endpoint, same error handling -- so a caller
// (pkg/feedback's HTTP handler) never has to reason about two different
// failure modes for what is, from LensAI's point of view, the same event
// pipeline with a different source value.
func (w *Writer) EmitCacheFeedback(ctx context.Context, tenantID, modelID, matchedPromptHash string) error {
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
		CostUSD:         0,
		Status:          StatusThumbsDown,
		Source:          SourceCacheFeedback,
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
