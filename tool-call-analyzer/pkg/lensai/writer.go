// SPDX-License-Identifier: MIT
// Package lensai dual-writes tool-call cost into LensAI's ingestion wire
// format (see ingestion/src/handlers/event.rs::InferenceEvent in the
// infra-ai-streaming repo root) so a single Grafana board can join LLM
// inference cost and tool-call cost per tenant.
//
// This is hot/cold tiering for observability: the ClickHouse tool_calls
// table (pkg/clickhouse) stays the source of truth for tool-call analytics,
// while this package additionally posts the same cost event onto LensAI's
// existing /ingest pipeline (WAL + Kafka + ClickHouse) using LensAI's own
// envelope so tool cost and inference cost land in one query face.
package lensai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
)

// SourceToolCall discriminates tool-call cost events from LensAI's native
// LLM inference cost events on the shared ingest pipeline. LensAI's own
// InferenceEvent struct does not use `#[serde(deny_unknown_fields)]`, so
// this additive field is safely ignored by callers that don't know about it
// yet, while downstream consumers (ClickHouse JSONExtract, Grafana panels)
// can filter on it.
const SourceToolCall = "tool_call"

// Event is the LensAI ingest wire envelope for a single event. Field names
// and JSON tags mirror ingestion/src/handlers/event.rs::InferenceEvent
// exactly so a Go HTTP client can POST straight to LensAI's existing
// /ingest endpoint. TraceID and Source are additive fields layered on top
// of the native schema: TraceID is what lets a Grafana board join tool cost
// to inference cost for the same trace, and Source distinguishes the two
// event kinds once they're in the same table.
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

// Writer dual-writes tool-call cost events to a LensAI ingest endpoint over
// HTTP, following the same NewWithClient injection pattern as
// pkg/clickhouse.Writer so it stays httptest-friendly.
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

// Insert dual-writes tc's cost onto the LensAI ingest pipeline as a
// tool_call-sourced event. tenantID is required -- ToolCall itself carries
// no tenant, so the caller (the span processor that already knows which
// tenant the trace belongs to) supplies it explicitly, the same way
// pkg/clickhouse.Writer.Insert takes duration/status from its caller rather
// than from ToolCall. hasError and status describe the outcome of the call.
func (w *Writer) Insert(ctx context.Context, tc types.ToolCall, tenantID string, hasError bool, status string) error {
	if tenantID == "" {
		return fmt.Errorf("lensai: tenant_id is required")
	}

	evt := ToEvent(tc, tenantID, hasError, status)

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

// ToEvent converts a ToolCall into a LensAI ingest Event. tenantID and
// hasError/status are supplied by the caller for the same reason described
// on Insert. Exported so callers (e.g. the traceforge CLI) can build the
// envelope without needing an HTTP round trip -- useful for dry-run output
// and for constructing an ingestRequest batch of more than one event.
func ToEvent(tc types.ToolCall, tenantID string, hasError bool, status string) Event {
	modelID := tc.Cost.ModelName
	if modelID == "" {
		modelID = tc.Name
	}

	timestampUnixMs := uint64(0)
	if tc.FinishedAtNs > 0 {
		timestampUnixMs = uint64(tc.FinishedAtNs / int64(time.Millisecond))
	} else {
		timestampUnixMs = uint64(time.Now().UnixMilli())
	}

	var latencyMs uint32
	if tc.DurationMs > 0 {
		latencyMs = uint32(tc.DurationMs)
	}

	if status == "" {
		if hasError {
			status = "error"
		} else {
			status = "success"
		}
	}

	return Event{
		EventID:          tc.ID,
		TenantID:         tenantID,
		ModelID:          modelID,
		TraceID:          tc.TraceID,
		TimestampUnixMs:  timestampUnixMs,
		LatencyMs:        latencyMs,
		PromptTokens:     uint32(tc.Cost.InputTokens),
		CompletionTokens: uint32(tc.Cost.OutputTokens),
		CostUSD:          tc.Cost.CostUSD,
		Status:           status,
		RequestID:        tc.SpanID,
		Source:           SourceToolCall,
	}
}
