// SPDX-License-Identifier: MIT

// Package lensai dual-writes cost-budget-enforcer's gateway outcomes onto
// LensAI's existing /ingest pipeline, the same envelope and injection shape
// semantic-cache-engine/pkg/lensai and tool-call-analyzer/pkg/lensai already
// established for their own sources. Day 68's DESIGN.md §6 adds three new
// source values this repo didn't previously produce: a real spend record
// (source=SourceGatewayInference, cost_usd is the model call's actual
// price) and two zero-cost records for the two ways a request can leave
// pkg/gateway without spending anything (cache hit, budget block). Keeping
// all three on the shared infra_ai.inference_events table, distinguished
// only by source, is the same "one clearinghouse ledger" choice
// semantic-cache-engine's DESIGN.md §5 already made for its own hit/miss
// events.
package lensai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SourceGatewayInference is the InferenceEvent.source value for a request
// that reached the model — the only path with a nonzero cost_usd. Wiring
// this value onto the same field the rest of the fleet already reports
// spend on is the entire point of Day 68: without it, cost-budget-enforcer
// could reject or degrade a request, but nobody watching LensAI's cost_usd
// stream would see either the spend that did happen or the spend a block
// prevented.
const SourceGatewayInference = "gateway_inference"

// SourceGatewayCacheHit is the InferenceEvent.source value for a request
// pkg/gateway served from the semantic cache instead of calling the model.
// cost_usd is always 0 here: a cache hit is, by construction, the one path
// through the gateway that spends nothing, and recording that as an
// explicit zero (rather than not emitting an event at all) is what lets a
// cost dashboard show "spend avoided by cache" as a real number instead of
// an absence.
const SourceGatewayCacheHit = "gateway_cache_hit"

// SourceGatewayBlocked is the InferenceEvent.source value for a request
// enforcer.Block rejected before pkg/gateway touched the cache or the
// model. This is the event that makes Day 68's AI Learning hook —
// "order matters, wrong order leaks spend" — checkable after the fact: a
// tenant hitting their hard limit should show up in the stream as a
// zero-cost block, never as a gateway_inference row with a real charge.
const SourceGatewayBlocked = "gateway_blocked"

// StatusBlocked is the Event.Status value EmitBlocked sets, mirroring how
// SourceGatewayInference and SourceGatewayCacheHit both use "ok" — a
// blocked request isn't a failure of the gateway, it is the gateway
// working, so it gets its own status rather than being folded into an
// error-shaped one.
const StatusBlocked = "blocked"

// Event mirrors ingestion/src/handlers/event.rs::InferenceEvent's wire
// format, matching semantic-cache-engine/pkg/lensai.Event's field set so a
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

// Writer posts gateway outcome events to a LensAI ingest endpoint over
// HTTP.
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

// EmitSpend posts a single gateway_inference event for a request that
// reached modelID and cost costUSD. This is Day 68's core wiring: the
// first place in cost-budget-enforcer a real dollar amount, sourced from
// the model call itself rather than an estimate, reaches LensAI's
// cost_usd stream.
func (w *Writer) EmitSpend(ctx context.Context, tenantID, modelID string, costUSD float64, latency time.Duration) error {
	if tenantID == "" {
		return fmt.Errorf("lensai: tenant_id is required")
	}
	if modelID == "" {
		return fmt.Errorf("lensai: model_id is required")
	}

	evt := Event{
		TenantID:        tenantID,
		ModelID:         modelID,
		TimestampUnixMs: uint64(time.Now().UnixMilli()),
		LatencyMs:       uint32(latency.Milliseconds()),
		CostUSD:         costUSD,
		Status:          "ok",
		Source:          SourceGatewayInference,
	}
	return w.post(ctx, tenantID, evt)
}

// EmitCacheHit posts a single gateway_cache_hit event for a request
// pkg/gateway served from the cache at matchedPromptHash, at cost_usd=0.
func (w *Writer) EmitCacheHit(ctx context.Context, tenantID, modelID, matchedPromptHash string, latency time.Duration) error {
	if tenantID == "" {
		return fmt.Errorf("lensai: tenant_id is required")
	}

	evt := Event{
		TenantID:        tenantID,
		ModelID:         modelID,
		TraceID:         matchedPromptHash,
		TimestampUnixMs: uint64(time.Now().UnixMilli()),
		LatencyMs:       uint32(latency.Milliseconds()),
		CostUSD:         0,
		Status:          "ok",
		Source:          SourceGatewayCacheHit,
	}
	return w.post(ctx, tenantID, evt)
}

// EmitBlocked posts a single gateway_blocked event for a request
// enforcer.Block rejected before any cache lookup or model call happened,
// at cost_usd=0.
func (w *Writer) EmitBlocked(ctx context.Context, tenantID string) error {
	if tenantID == "" {
		return fmt.Errorf("lensai: tenant_id is required")
	}

	evt := Event{
		TenantID:        tenantID,
		TimestampUnixMs: uint64(time.Now().UnixMilli()),
		CostUSD:         0,
		Status:          StatusBlocked,
		Source:          SourceGatewayBlocked,
	}
	return w.post(ctx, tenantID, evt)
}

func (w *Writer) post(ctx context.Context, tenantID string, evt Event) error {
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("lensai: unexpected status %d", resp.StatusCode)
	}
	return nil
}
