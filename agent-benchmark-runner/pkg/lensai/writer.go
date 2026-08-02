// SPDX-License-Identifier: MIT

// Package lensai dual-writes a benchmark batch's completion onto LensAI's
// ingestion wire format (see ingestion/src/handlers/event.rs::InferenceEvent
// in the infra-ai-streaming repo root) so a single Grafana board can join
// inference cost and benchmark outcome per tenant.
//
// This mirrors tool-call-analyzer/pkg/lensai's dual-write exactly, one
// module over: pkg/store's benchmark_runs table (ClickHouse) stays the
// source of truth for per-repetition benchmark data, while this package
// additionally posts one event per completed batch onto LensAI's existing
// /ingest pipeline (WAL + Kafka + ClickHouse) using LensAI's own envelope,
// discriminated by source so a batch-completion event and a tool-call cost
// event land in the same query face without being confused for each other.
package lensai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/akshantvats/agent-benchmark-runner/pkg/orchestrator"
)

// SourceBenchmarkRun discriminates benchmark-batch-completion events from
// LensAI's native LLM inference cost events, and from
// tool-call-analyzer/pkg/lensai's SourceToolCall events, on the shared
// ingest pipeline. LensAI's InferenceEvent struct does not use
// #[serde(deny_unknown_fields)], so this additive field is safely ignored
// by callers that don't know about it yet, while downstream consumers
// (ClickHouse JSONExtract, Grafana panels) can filter on it.
const SourceBenchmarkRun = "benchmark_run"

// Event is the LensAI ingest wire envelope for a single event. Field names
// and JSON tags mirror ingestion/src/handlers/event.rs::InferenceEvent
// exactly, plus the same additive TraceID/Source fields
// tool-call-analyzer/pkg/lensai introduced, so a Go HTTP client can POST
// straight to LensAI's existing /ingest endpoint.
type Event struct {
	EventID          string  `json:"event_id,omitempty"`
	TenantID         string  `json:"tenant_id"`
	ModelID          string  `json:"model_id"`
	TraceID          string  `json:"trace_id,omitempty"`
	TimestampUnixMs  uint64  `json:"timestamp_unix_ms"`
	LatencyMs        uint32  `json:"latency_ms"`
	PromptTokens     uint32  `json:"prompt_tokens"`
	CompletionTokens uint32  `json:"completion_tokens"`
	CostUSD          float64 `json:"cost_usd"`
	Status           string  `json:"status,omitempty"`
	RequestID        string  `json:"request_id,omitempty"`
	Source           string  `json:"source,omitempty"`
}

// ingestRequest mirrors ingestion/src/handlers/event.rs::IngestRequest --
// LensAI's /ingest endpoint always expects a batch, even for a single event.
type ingestRequest struct {
	Events []Event `json:"events"`
}

// tenantHeader is the header LensAI's ingest handler reads the tenant from
// when the batch itself doesn't carry one out of band (see
// ingestion/src/handlers/ingest.rs::TENANT_HEADER).
const tenantHeader = "X-Tenant-ID"

// Writer dual-writes benchmark-batch-completion events to a LensAI ingest
// endpoint over HTTP, following the same NewWithClient injection pattern as
// tool-call-analyzer/pkg/lensai.Writer so it stays httptest-friendly.
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

// NewWithClient creates a Writer with an injected HTTP client (for testing).
func NewWithClient(ingestURL string, client *http.Client) *Writer {
	return &Writer{ingestURL: ingestURL, httpClient: client}
}

// BatchParams supplies the fields orchestrator.Summary doesn't carry itself
// but a LensAI event needs: which task and agent produced this batch, which
// tenant to bill it to, an identifier for this specific batch invocation,
// how long the batch took wall-clock, and when it completed. Every field is
// required except BatchID, the same way tool-call-analyzer/pkg/lensai.Insert
// requires its caller to supply tenantID explicitly rather than guessing it.
type BatchParams struct {
	TaskID      string
	AgentName   string
	TenantID    string
	BatchID     string
	Duration    time.Duration
	CompletedAt time.Time
}

// Insert dual-writes summary onto the LensAI ingest pipeline as a single
// benchmark_run-sourced event describing the whole batch, not one event per
// repetition -- see DESIGN.md's "One Event per Batch, Not per Repetition"
// for why. params.TenantID is required the same way tool-call-analyzer's
// Insert requires it: neither orchestrator.Summary nor BatchParams' other
// fields carry a tenant on their own.
func (w *Writer) Insert(ctx context.Context, summary orchestrator.Summary, params BatchParams) error {
	if params.TenantID == "" {
		return fmt.Errorf("lensai: tenant_id is required")
	}

	evt := ToEvent(summary, params)

	payload, err := json.Marshal(ingestRequest{Events: []Event{evt}})
	if err != nil {
		return fmt.Errorf("lensai: marshal failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.ingestURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("lensai: build request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(tenantHeader, params.TenantID)

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

// ToEvent converts a batch's Summary into a LensAI ingest Event. Exported so
// callers can build the envelope without an HTTP round trip -- useful for
// dry-run output, the same reason tool-call-analyzer/pkg/lensai.ToEvent is
// exported.
//
// CostUSD is always 0.0: orchestrator.Summary carries no cost data, and the
// only cost data this module has (pkg/report.CostReport, Day 54) is scoped
// to two-agent pkg/compare output, not the single-agent orchestrator.Run
// path this event is built from. See DESIGN.md's "CostUSD Stays Zero" for
// why estimating one here would double-count cost already dual-written
// elsewhere.
func ToEvent(summary orchestrator.Summary, params BatchParams) Event {
	var latencyMs uint32
	if params.Duration > 0 {
		latencyMs = uint32(params.Duration.Milliseconds())
	}

	completedAt := params.CompletedAt
	if completedAt.IsZero() {
		completedAt = time.Now()
	}

	return Event{
		EventID:          params.BatchID,
		TenantID:         params.TenantID,
		ModelID:          params.AgentName,
		TraceID:          params.TaskID,
		TimestampUnixMs:  uint64(completedAt.UnixMilli()),
		LatencyMs:        latencyMs,
		PromptTokens:     0,
		CompletionTokens: 0,
		CostUSD:          0.0,
		Status:           batchStatus(summary),
		RequestID:        params.BatchID,
		Source:           SourceBenchmarkRun,
	}
}

// batchStatus derives an ingest-friendly status from a batch Summary: no
// completed repetitions maps to "error" (the batch itself never produced a
// signal), a perfect pass rate maps to "pass", anything else maps to
// "fail". This generalizes tool-call-analyzer/pkg/lensai.ToEvent's
// hasError/status derivation from one call's boolean outcome to a batch's
// aggregate completion state.
func batchStatus(summary orchestrator.Summary) string {
	if summary.Completed == 0 {
		return "error"
	}
	if summary.PassRate == 1.0 {
		return "pass"
	}
	return "fail"
}
