// SPDX-License-Identifier: MIT

// Package lensai posts prompt-fingerprinter's exact-match cache hits onto
// LensAI's existing /ingest pipeline — the same envelope
// cost-budget-enforcer/pkg/lensai and semantic-cache-engine/pkg/lensai
// already established for their own sources, and the same path that
// reaches LensAI's Kafka topic on the Rust ingestion side. DESIGN.md §4
// reserved SourceCacheHitExact on Day 70 without ever populating it; Day
// 76 is what finally gives that source value a writer.
package lensai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SourceCacheHitExact is the Event.Source value for a request
// prompt-fingerprinter's Stack served from its L1 exact-match tier —
// DESIGN.md §4's reserved value, distinct from semantic-cache-engine's
// cache_hit/gateway_cache_hit so a dashboard can tell "how much traffic
// is a literal duplicate" apart from "how much is merely similar."
// cost_usd is always 0: an L1 hit is, by construction, the cheapest path
// through this stack.
const SourceCacheHitExact = "cache_hit_exact"

// Event mirrors ingestion/src/handlers/event.rs::InferenceEvent's wire
// format, matching cost-budget-enforcer/pkg/lensai.Event's field set so
// this Go HTTP client can POST to the same /ingest endpoint without a
// separate envelope type per producer.
type Event struct {
	EventID         string  `json:"event_id,omitempty"`
	TenantID        string  `json:"tenant_id"`
	ModelID         string  `json:"model_id,omitempty"`
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

// Writer posts prompt-fingerprinter cache-hit events to a LensAI ingest
// endpoint over HTTP.
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

// EmitExactHit posts a single cache_hit_exact event for a request Stack
// served from L1 at the given fingerprint key, at cost_usd=0.
func (w *Writer) EmitExactHit(ctx context.Context, tenantID, fingerprintKey string, latency time.Duration) error {
	if tenantID == "" {
		return fmt.Errorf("lensai: tenant_id is required")
	}

	evt := Event{
		TenantID:        tenantID,
		TraceID:         fingerprintKey,
		TimestampUnixMs: uint64(time.Now().UnixMilli()),
		LatencyMs:       uint32(latency.Milliseconds()),
		CostUSD:         0,
		Status:          "ok",
		Source:          SourceCacheHitExact,
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
