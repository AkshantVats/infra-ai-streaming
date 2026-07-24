// SPDX-License-Identifier: MIT
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AkshantVats/infra-ai-streaming/traceforge/pkg/schema"
)

// stubExporter satisfies the exporter interface used in tests.
type stubExporter struct {
	received []schema.Span
	failWith error
}

func (s *stubExporter) Export(_ interface{}, spans []schema.Span) error {
	if s.failWith != nil {
		return s.failWith
	}
	s.received = append(s.received, spans...)
	return nil
}

func validSpan() schema.Span {
	return schema.Span{
		TraceID:   "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:    "00f067aa0ba902b7",
		ToolName:  "llm_call",
		Status:    schema.StatusOK,
		LatencyMs: 300,
		Ts:        time.Now(),
	}
}

func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	healthHandler(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("health: got %d, want 200", w.Code)
	}
}

func TestSpanHandler_InvalidJSON(t *testing.T) {
	handler := makeSpanHandlerWithExporter(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/spans", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid json: got %d, want 400", w.Code)
	}
}

func TestSpanHandler_MissingTraceID(t *testing.T) {
	span := schema.Span{
		SpanID:    "00f067aa0ba902b7",
		ToolName:  "llm_call",
		Status:    schema.StatusOK,
		LatencyMs: 100,
		Ts:        time.Now(),
	}
	body, _ := json.Marshal([]schema.Span{span})
	req := httptest.NewRequest(http.MethodPost, "/v1/spans", bytes.NewReader(body))
	w := httptest.NewRecorder()
	makeSpanHandlerWithExporter(nil)(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("missing trace_id: got %d, want 400", w.Code)
	}
}

func TestSpanHandler_MissingSpanID(t *testing.T) {
	span := schema.Span{
		TraceID:   "4bf92f3577b34da6a3ce929d0e0e4736",
		ToolName:  "llm_call",
		Status:    schema.StatusOK,
		LatencyMs: 100,
		Ts:        time.Now(),
	}
	body, _ := json.Marshal([]schema.Span{span})
	req := httptest.NewRequest(http.MethodPost, "/v1/spans", bytes.NewReader(body))
	w := httptest.NewRecorder()
	makeSpanHandlerWithExporter(nil)(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("missing span_id: got %d, want 400", w.Code)
	}
}

func TestSpanHandler_InvalidStatus(t *testing.T) {
	span := validSpan()
	span.Status = "not_a_status"
	body, _ := json.Marshal([]schema.Span{span})
	req := httptest.NewRequest(http.MethodPost, "/v1/spans", bytes.NewReader(body))
	w := httptest.NewRecorder()
	makeSpanHandlerWithExporter(nil)(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid status: got %d, want 400", w.Code)
	}
}

func TestSpanHandler_ValidBatch(t *testing.T) {
	stub := &stubExporter{}
	spans := []schema.Span{validSpan(), validSpan()}
	spans[1].SpanID = "1234567890abcdef"

	body, _ := json.Marshal(spans)
	req := httptest.NewRequest(http.MethodPost, "/v1/spans", bytes.NewReader(body))
	w := httptest.NewRecorder()
	makeSpanHandlerWithExporter(stub)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("valid batch: got %d, want 200. body: %s", w.Code, w.Body.String())
	}
	if len(stub.received) != 2 {
		t.Errorf("exporter received %d spans, want 2", len(stub.received))
	}
}

// spanExporter is the interface satisfied by both export.SpanExporter and stubExporter.
type spanExporter interface {
	Export(ctx interface{}, spans []schema.Span) error
}

// makeSpanHandlerWithExporter is a test-only factory that injects a stub exporter.
func makeSpanHandlerWithExporter(exp spanExporter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var spans []schema.Span
		if err := json.NewDecoder(r.Body).Decode(&spans); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		for i, s := range spans {
			if err := s.Validate(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				_ = i
				return
			}
		}

		if exp != nil {
			if err := exp.Export(nil, spans); err != nil {
				http.Error(w, "export error", http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
	}
}
