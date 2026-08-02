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
	"time"

	"github.com/akshantvats/agent-benchmark-runner/pkg/lensai"
	"github.com/akshantvats/agent-benchmark-runner/pkg/orchestrator"
)

func sampleParams() lensai.BatchParams {
	return lensai.BatchParams{
		TaskID:      "checkout-happy-path",
		AgentName:   "agent-a",
		TenantID:    "acme",
		BatchID:     "batch-001",
		Duration:    2500 * time.Millisecond,
		CompletedAt: time.UnixMilli(1_715_000_000_000),
	}
}

func passingSummary() orchestrator.Summary {
	return orchestrator.Summary{
		N:           10,
		Completed:   10,
		Passed:      10,
		PassRate:    1.0,
		CILow:       0.72,
		CIHigh:      1.0,
		MedianSteps: 4,
		P95Steps:    6,
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
	params := sampleParams()

	if err := writer.Insert(context.Background(), passingSummary(), params); err != nil {
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
	if evt.ModelID != "agent-a" {
		t.Errorf("expected model_id=agent-a, got %q", evt.ModelID)
	}
	if evt.TraceID != "checkout-happy-path" {
		t.Errorf("expected trace_id=checkout-happy-path, got %q", evt.TraceID)
	}
	if evt.CostUSD != 0.0 {
		t.Errorf("expected cost_usd=0.0, got %v", evt.CostUSD)
	}
	if evt.Source != lensai.SourceBenchmarkRun {
		t.Errorf("expected source=%q, got %q", lensai.SourceBenchmarkRun, evt.Source)
	}
	if evt.Status != "pass" {
		t.Errorf("expected status=pass, got %q", evt.Status)
	}
	if evt.LatencyMs != 2500 {
		t.Errorf("expected latency_ms=2500, got %d", evt.LatencyMs)
	}
	if evt.EventID != "batch-001" || evt.RequestID != "batch-001" {
		t.Errorf("expected event_id/request_id=batch-001, got %q/%q", evt.EventID, evt.RequestID)
	}
}

func TestWriterInsertFailingBatch(t *testing.T) {
	var receivedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	writer := lensai.NewWithClient(srv.URL, srv.Client())
	summary := passingSummary()
	summary.Passed = 7
	summary.PassRate = 0.7

	if err := writer.Insert(context.Background(), summary, sampleParams()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(receivedBody, `"status":"fail"`) {
		t.Errorf("expected status=fail in body, got: %s", receivedBody)
	}
}

func TestWriterInsertMissingTenantID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called when tenant_id is empty")
	}))
	defer srv.Close()

	writer := lensai.NewWithClient(srv.URL, srv.Client())
	params := sampleParams()
	params.TenantID = ""

	err := writer.Insert(context.Background(), passingSummary(), params)
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

	err := writer.Insert(context.Background(), passingSummary(), sampleParams())
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

	if err := writer.Insert(context.Background(), passingSummary(), sampleParams()); err != nil {
		t.Fatalf("unexpected error for HTTP 202: %v", err)
	}
}

func TestToEventZeroCompletedMapsToError(t *testing.T) {
	summary := orchestrator.Summary{N: 5, Completed: 0, Passed: 0}
	evt := lensai.ToEvent(summary, sampleParams())

	if evt.Status != "error" {
		t.Errorf("expected status=error for zero completed repetitions, got %q", evt.Status)
	}
	if evt.CostUSD != 0.0 {
		t.Errorf("expected cost_usd=0.0, got %v", evt.CostUSD)
	}
}

func TestToEventPartialPassRateMapsToFail(t *testing.T) {
	summary := orchestrator.Summary{N: 10, Completed: 10, Passed: 3, PassRate: 0.3}
	evt := lensai.ToEvent(summary, sampleParams())

	if evt.Status != "fail" {
		t.Errorf("expected status=fail for partial pass rate, got %q", evt.Status)
	}
}

func TestToEventZeroDurationClampsLatencyToZero(t *testing.T) {
	params := sampleParams()
	params.Duration = 0
	evt := lensai.ToEvent(passingSummary(), params)

	if evt.LatencyMs != 0 {
		t.Errorf("expected latency_ms=0 for zero duration, got %d", evt.LatencyMs)
	}
}

func TestToEventFallsBackToWallClockTimestamp(t *testing.T) {
	params := sampleParams()
	params.CompletedAt = time.Time{}
	evt := lensai.ToEvent(passingSummary(), params)

	if evt.TimestampUnixMs == 0 {
		t.Error("expected non-zero timestamp_unix_ms when CompletedAt is unset")
	}
}

func TestToEventOmitsEmptyTraceIDFromJSON(t *testing.T) {
	params := sampleParams()
	params.TaskID = ""
	evt := lensai.ToEvent(passingSummary(), params)

	payload, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if strings.Contains(string(payload), `"trace_id"`) {
		t.Errorf("expected trace_id to be omitted from JSON when empty, got: %s", payload)
	}
}
