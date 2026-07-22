// SPDX-License-Identifier: MIT
package traceforge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestStartSpanCreatesRootSpan verifies that StartSpan on a context with no
// active span generates a new trace_id and no parent_span_id.
func TestStartSpanCreatesRootSpan(t *testing.T) {
	ctx := context.Background()
	_, s := StartSpan(ctx, "search_web")
	if s.TraceID == "" {
		t.Fatal("expected trace_id to be set")
	}
	if s.SpanID == "" {
		t.Fatal("expected span_id to be set")
	}
	if s.ParentSpanID != "" {
		t.Fatalf("root span must not have parent_span_id, got %q", s.ParentSpanID)
	}
	if s.ToolName != "search_web" {
		t.Fatalf("expected tool_name=search_web, got %q", s.ToolName)
	}
}

// TestStartSpanInheritsParent verifies that a child span shares trace_id with
// its parent and sets parent_span_id correctly.
func TestStartSpanInheritsParent(t *testing.T) {
	ctx := context.Background()
	parentCtx, parent := StartSpan(ctx, "root_agent")

	childCtx, child := StartSpan(parentCtx, "calculator")
	if child.TraceID != parent.TraceID {
		t.Fatalf("child trace_id %q != parent trace_id %q", child.TraceID, parent.TraceID)
	}
	if child.ParentSpanID != parent.SpanID {
		t.Fatalf("child parent_span_id %q != parent span_id %q", child.ParentSpanID, parent.SpanID)
	}
	_ = childCtx
}

// TestSpanFromContextReturnsActiveSpan checks that SpanFromContext retrieves the
// span stored by StartSpan.
func TestSpanFromContextReturnsActiveSpan(t *testing.T) {
	ctx := context.Background()
	sctx, s := StartSpan(ctx, "tool")
	got := SpanFromContext(sctx)
	if got != s {
		t.Fatal("SpanFromContext did not return the active span")
	}
}

// TestSpanFromContextNilWhenEmpty verifies the nil case.
func TestSpanFromContextNilWhenEmpty(t *testing.T) {
	ctx := context.Background()
	if SpanFromContext(ctx) != nil {
		t.Fatal("expected nil for empty context")
	}
}

// TestEndSpanSetsLatency verifies that EndSpan computes a non-negative latency.
func TestEndSpanSetsLatency(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	os.Setenv("TRACEFORGE_COLLECTOR_URL", srv.URL)
	defer os.Unsetenv("TRACEFORGE_COLLECTOR_URL")

	ctx := context.Background()
	sctx, s := StartSpan(ctx, "tool")
	EndSpan(sctx, s, StatusOK, nil)
	if s.LatencyMs < 0 {
		t.Fatalf("latency_ms must be non-negative, got %f", s.LatencyMs)
	}
	if s.Status != StatusOK {
		t.Fatalf("expected status=ok, got %q", s.Status)
	}
}

// TestEndSpanEmitsHTTP verifies that EndSpan POSTs a valid JSON span to the
// configured collector URL.
func TestEndSpanEmitsHTTP(t *testing.T) {
	received := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	os.Setenv("TRACEFORGE_COLLECTOR_URL", srv.URL)
	defer os.Unsetenv("TRACEFORGE_COLLECTOR_URL")

	ctx := context.Background()
	sctx, s := StartSpan(ctx, "weather_api")
	EndSpan(sctx, s, StatusOK, nil)

	select {
	case body := <-received:
		var got Span
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("collector received invalid JSON: %v — body: %s", err, body)
		}
		if got.ToolName != "weather_api" {
			t.Fatalf("expected tool_name=weather_api, got %q", got.ToolName)
		}
	case <-context.Background().Done():
		t.Fatal("timed out waiting for HTTP emit")
	}
}

// TestEndSpanSetsErrorMessage verifies that an error passed to EndSpan
// is captured in error_msg.
func TestEndSpanSetsErrorMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	os.Setenv("TRACEFORGE_COLLECTOR_URL", srv.URL)
	defer os.Unsetenv("TRACEFORGE_COLLECTOR_URL")

	ctx := context.Background()
	sctx, s := StartSpan(ctx, "failing_tool")
	EndSpan(sctx, s, StatusError, errors.New("connection refused"))
	if s.ErrorMsg != "connection refused" {
		t.Fatalf("expected error_msg='connection refused', got %q", s.ErrorMsg)
	}
	if s.Status != StatusError {
		t.Fatalf("expected status=error, got %q", s.Status)
	}
}

// TestInjectTraceContext verifies that InjectTraceContext writes a valid W3C
// traceparent header from the active span.
func TestInjectTraceContext(t *testing.T) {
	ctx := context.Background()
	sctx, s := StartSpan(ctx, "root")
	header := make(http.Header)
	InjectTraceContext(sctx, header)
	tp := header.Get("traceparent")
	if tp == "" {
		t.Fatal("expected traceparent header to be set")
	}
	parts := strings.Split(tp, "-")
	if len(parts) != 4 {
		t.Fatalf("malformed traceparent: %q", tp)
	}
	if parts[0] != "00" {
		t.Fatalf("expected version=00, got %q", parts[0])
	}
	if !strings.Contains(parts[1], cleanID(s.TraceID)) && parts[1] != cleanID(s.TraceID) {
		t.Fatalf("traceparent trace_id mismatch: expected %q, got %q", cleanID(s.TraceID), parts[1])
	}
}

// TestInjectNoOpWhenNoSpan verifies that InjectTraceContext does nothing when
// no span is active.
func TestInjectNoOpWhenNoSpan(t *testing.T) {
	ctx := context.Background()
	header := make(http.Header)
	InjectTraceContext(ctx, header)
	if header.Get("traceparent") != "" {
		t.Fatal("expected no traceparent header when context has no active span")
	}
}

// TestExtractTraceContext verifies that a valid traceparent header is parsed
// and the resulting context carries the parent IDs.
func TestExtractTraceContext(t *testing.T) {
	header := http.Header{}
	traceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	parentSpanID := "00f067aa0ba902b7"
	header.Set("traceparent", "00-"+traceID+"-"+parentSpanID+"-01")

	ctx := ExtractTraceContext(context.Background(), header)
	s := SpanFromContext(ctx)
	if s == nil {
		t.Fatal("expected span in context after ExtractTraceContext")
	}
	if s.TraceID != traceID {
		t.Fatalf("expected trace_id=%q, got %q", traceID, s.TraceID)
	}
	if s.ParentSpanID != parentSpanID {
		t.Fatalf("expected parent_span_id=%q, got %q", parentSpanID, s.ParentSpanID)
	}
}

// TestExtractIgnoresMalformedHeader verifies that a malformed traceparent
// header leaves the context unchanged.
func TestExtractIgnoresMalformedHeader(t *testing.T) {
	header := http.Header{}
	header.Set("traceparent", "bad-header")
	ctx := ExtractTraceContext(context.Background(), header)
	if SpanFromContext(ctx) != nil {
		t.Fatal("expected nil span for malformed traceparent")
	}
}
