// SPDX-License-Identifier: MIT
package lensai_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AkshantVats/tool-call-analyzer/pkg/lensai"
	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
)

func sampleToolCall() types.ToolCall {
	return types.ToolCall{
		ID:           "tcall-001",
		TraceID:      "trace-abc",
		SpanID:       "span-001",
		Name:         "search_web",
		Vendor:       "openai",
		Category:     types.CategoryHTTP,
		FinishedAtNs: 1_715_000_000_000_000_000,
		DurationMs:   120,
		Cost: types.CostEstimate{
			InputTokens:  512,
			OutputTokens: 64,
			ModelName:    "gpt-4o",
			CostUSD:      0.00192,
		},
	}
}

func TestWriterInsertSuccess(t *testing.T) {
	var receivedBody string
	var receivedHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		receivedHeader = r.Header.Get("X-Tenant-ID")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	writer := lensai.NewWithClient(srv.URL, srv.Client())
	tc := sampleToolCall()

	if err := writer.Insert(context.Background(), tc, "acme", false, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedHeader != "acme" {
		t.Errorf("expected X-Tenant-ID header 'acme', got: %q", receivedHeader)
	}

	var req struct {
		Events []lensai.Event `json:"events"`
	}
	if err := json.Unmarshal([]byte(receivedBody), &req); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if len(req.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(req.Events))
	}

	evt := req.Events[0]
	if evt.TenantID != "acme" {
		t.Errorf("expected tenant_id=acme, got %q", evt.TenantID)
	}
	if evt.TraceID != "trace-abc" {
		t.Errorf("expected trace_id=trace-abc, got %q", evt.TraceID)
	}
	if evt.ModelID != "gpt-4o" {
		t.Errorf("expected model_id=gpt-4o, got %q", evt.ModelID)
	}
	if evt.CostUSD != 0.00192 {
		t.Errorf("expected cost_usd=0.00192, got %v", evt.CostUSD)
	}
	if evt.Source != lensai.SourceToolCall {
		t.Errorf("expected source=%q, got %q", lensai.SourceToolCall, evt.Source)
	}
	if evt.Status != "success" {
		t.Errorf("expected status=success, got %q", evt.Status)
	}
	if evt.LatencyMs != 120 {
		t.Errorf("expected latency_ms=120, got %d", evt.LatencyMs)
	}
	if evt.PromptTokens != 512 || evt.CompletionTokens != 64 {
		t.Errorf("expected prompt_tokens=512 completion_tokens=64, got %d/%d", evt.PromptTokens, evt.CompletionTokens)
	}
}

func TestWriterInsertErrorCall(t *testing.T) {
	var receivedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	writer := lensai.NewWithClient(srv.URL, srv.Client())
	tc := sampleToolCall()

	if err := writer.Insert(context.Background(), tc, "acme", true, "ERROR"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(receivedBody, `"status":"ERROR"`) {
		t.Errorf("expected status=ERROR in body, got: %s", receivedBody)
	}
	if !strings.Contains(receivedBody, `"source":"tool_call"`) {
		t.Errorf("expected source=tool_call in body, got: %s", receivedBody)
	}
}

func TestWriterInsertDerivesErrorStatusWhenUnset(t *testing.T) {
	var receivedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	writer := lensai.NewWithClient(srv.URL, srv.Client())
	tc := sampleToolCall()

	if err := writer.Insert(context.Background(), tc, "acme", true, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(receivedBody, `"status":"error"`) {
		t.Errorf("expected derived status=error in body, got: %s", receivedBody)
	}
}

func TestWriterInsertMissingTenantID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called when tenant_id is empty")
	}))
	defer srv.Close()

	writer := lensai.NewWithClient(srv.URL, srv.Client())
	tc := sampleToolCall()

	err := writer.Insert(context.Background(), tc, "", false, "")
	if err == nil {
		t.Fatal("expected error for empty tenant_id, got nil")
	}
	if !strings.Contains(err.Error(), "tenant_id") {
		t.Errorf("expected tenant_id in error message, got: %v", err)
	}
}

func TestWriterInsertHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	writer := lensai.NewWithClient(srv.URL, srv.Client())
	tc := sampleToolCall()

	err := writer.Insert(context.Background(), tc, "acme", false, "")
	if err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 in error message, got: %v", err)
	}
}

func TestWriterInsertAcceptsHTTP202(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	writer := lensai.NewWithClient(srv.URL, srv.Client())
	tc := sampleToolCall()

	if err := writer.Insert(context.Background(), tc, "acme", false, ""); err != nil {
		t.Fatalf("unexpected error for HTTP 202: %v", err)
	}
}

func TestToEventZeroCost(t *testing.T) {
	tc := types.ToolCall{ID: "tcall-zero", TraceID: "trace-zero", Name: "noop"}
	evt := lensai.ToEvent(tc, "acme", false, "")

	if evt.CostUSD != 0.0 {
		t.Errorf("expected cost_usd=0.0, got %v", evt.CostUSD)
	}
	if evt.ModelID != "noop" {
		t.Errorf("expected model_id to fall back to tool name 'noop', got %q", evt.ModelID)
	}
	if evt.Status != "success" {
		t.Errorf("expected default status=success, got %q", evt.Status)
	}
	if evt.Source != lensai.SourceToolCall {
		t.Errorf("expected source=%q, got %q", lensai.SourceToolCall, evt.Source)
	}
}

func TestToEventMissingTraceID(t *testing.T) {
	tc := types.ToolCall{ID: "tcall-notrace", Name: "search_web"}
	evt := lensai.ToEvent(tc, "acme", false, "")

	if evt.TraceID != "" {
		t.Errorf("expected empty trace_id to pass through, got %q", evt.TraceID)
	}

	payload, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if strings.Contains(string(payload), `"trace_id"`) {
		t.Errorf("expected trace_id to be omitted from JSON when empty, got: %s", payload)
	}
}

func TestToEventEmptyTenantID(t *testing.T) {
	tc := sampleToolCall()
	evt := lensai.ToEvent(tc, "", false, "")

	if evt.TenantID != "" {
		t.Errorf("expected TenantID to pass through as empty (Insert is what rejects it), got %q", evt.TenantID)
	}
}

func TestToEventNegativeDuration(t *testing.T) {
	tc := sampleToolCall()
	tc.DurationMs = -50
	evt := lensai.ToEvent(tc, "acme", false, "")

	if evt.LatencyMs != 0 {
		t.Errorf("expected negative duration to clamp latency_ms to 0, got %d", evt.LatencyMs)
	}
}

func TestToEventFallsBackToWallClockTimestamp(t *testing.T) {
	tc := types.ToolCall{ID: "tcall-notime", TraceID: "trace-x", Name: "search_web"}
	evt := lensai.ToEvent(tc, "acme", false, "")

	if evt.TimestampUnixMs == 0 {
		t.Error("expected non-zero timestamp_unix_ms when FinishedAtNs is unset")
	}
}
