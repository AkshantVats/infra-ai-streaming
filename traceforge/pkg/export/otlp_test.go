// SPDX-License-Identifier: MIT
package export_test

import (
	"testing"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"

	"github.com/AkshantVats/infra-ai-streaming/traceforge/pkg/export"
	"github.com/AkshantVats/infra-ai-streaming/traceforge/pkg/schema"
)

func TestConvertToResourceSpans_Empty(t *testing.T) {
	rs := export.ConvertToResourceSpans(nil)
	if len(rs) != 0 {
		t.Fatalf("expected 0 resource spans for nil input, got %d", len(rs))
	}
}

func TestConvertToResourceSpans_SingleSpan(t *testing.T) {
	errMsg := "connection refused"
	span := schema.Span{
		TraceID:      "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:       "00f067aa0ba902b7",
		ToolName:     "llm_call",
		Model:        "claude-sonnet-4-6",
		Tokens:       850,
		CostUSD:      0.00034,
		Status:       schema.StatusOK,
		LatencyMs:    1200,
		Ts:           time.Date(2026, 7, 4, 2, 0, 0, 0, time.UTC),
		AgentID:      "research-agent",
		TenantID:     "tenant-001",
		ErrorMessage: &errMsg,
	}

	rs := export.ConvertToResourceSpans([]schema.Span{span})

	if len(rs) != 1 {
		t.Fatalf("expected 1 ResourceSpans, got %d", len(rs))
	}
	if rs[0].Resource == nil {
		t.Fatal("resource must not be nil")
	}
	if len(rs[0].ScopeSpans) != 1 {
		t.Fatalf("expected 1 ScopeSpans, got %d", len(rs[0].ScopeSpans))
	}

	spans := rs[0].ScopeSpans[0].Spans
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	ospan := spans[0]
	if ospan.Name != "llm_call" {
		t.Errorf("name: got %q, want %q", ospan.Name, "llm_call")
	}
	if len(ospan.TraceId) != 16 {
		t.Errorf("trace_id must be 16 bytes, got %d", len(ospan.TraceId))
	}
	if len(ospan.SpanId) != 8 {
		t.Errorf("span_id must be 8 bytes, got %d", len(ospan.SpanId))
	}

	// Check attributes are present
	attrKeys := make(map[string]bool)
	for _, a := range ospan.Attributes {
		attrKeys[a.Key] = true
		// Verify string attrs have correct type
		if a.Key == "traceforge.model" {
			if _, ok := a.Value.Value.(*commonpb.AnyValue_StringValue); !ok {
				t.Errorf("traceforge.model must be string type")
			}
		}
		if a.Key == "traceforge.tokens.total" {
			if _, ok := a.Value.Value.(*commonpb.AnyValue_IntValue); !ok {
				t.Errorf("traceforge.tokens.total must be int type")
			}
		}
		if a.Key == "traceforge.cost.usd" {
			if _, ok := a.Value.Value.(*commonpb.AnyValue_DoubleValue); !ok {
				t.Errorf("traceforge.cost.usd must be double type")
			}
		}
	}

	if !attrKeys["traceforge.model"] {
		t.Error("attribute traceforge.model missing")
	}
	if !attrKeys["traceforge.tokens.total"] {
		t.Error("attribute traceforge.tokens.total missing")
	}
	if !attrKeys["traceforge.cost.usd"] {
		t.Error("attribute traceforge.cost.usd missing")
	}
}

func TestConvertToResourceSpans_WithParentSpan(t *testing.T) {
	parentID := "1234567890abcdef"
	span := schema.Span{
		TraceID:      "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:       "00f067aa0ba902b7",
		ParentSpanID: &parentID,
		ToolName:     "bash_exec",
		Status:       schema.StatusOK,
		LatencyMs:    450,
		Ts:           time.Now(),
	}

	rs := export.ConvertToResourceSpans([]schema.Span{span})
	if len(rs) == 0 || len(rs[0].ScopeSpans[0].Spans) == 0 {
		t.Fatal("expected at least 1 span")
	}

	ospan := rs[0].ScopeSpans[0].Spans[0]
	if len(ospan.ParentSpanId) != 8 {
		t.Errorf("parent_span_id must be 8 bytes, got %d", len(ospan.ParentSpanId))
	}
}

func TestConvertToResourceSpans_InvalidHex_Skipped(t *testing.T) {
	span := schema.Span{
		TraceID:   "INVALID_TRACE_ID_NOT_HEX_0000000",
		SpanID:    "00f067aa0ba902b7",
		ToolName:  "web_search",
		Status:    schema.StatusOK,
		LatencyMs: 100,
		Ts:        time.Now(),
	}

	rs := export.ConvertToResourceSpans([]schema.Span{span})
	// Invalid spans are skipped, but ResourceSpans is still returned.
	if len(rs) == 0 {
		t.Fatal("expected ResourceSpans even for skipped spans")
	}
	// The invalid span should have been skipped.
	if len(rs[0].ScopeSpans[0].Spans) != 0 {
		t.Errorf("expected 0 valid spans, got %d", len(rs[0].ScopeSpans[0].Spans))
	}
}

func TestConvertToResourceSpans_ErrorStatus(t *testing.T) {
	msg := "timeout after 30s"
	span := schema.Span{
		TraceID:      "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:       "00f067aa0ba902b7",
		ToolName:     "api_call",
		Status:       schema.StatusError,
		LatencyMs:    30000,
		Ts:           time.Now(),
		ErrorMessage: &msg,
	}

	rs := export.ConvertToResourceSpans([]schema.Span{span})
	ospan := rs[0].ScopeSpans[0].Spans[0]

	if ospan.Status == nil {
		t.Fatal("status must not be nil")
	}

	// Import tracepb to check status code — use raw integer 2 = STATUS_CODE_ERROR
	if int(ospan.Status.Code) != 2 {
		t.Errorf("expected status code 2 (ERROR), got %d", ospan.Status.Code)
	}
}

func TestConvertToResourceSpans_Timestamps(t *testing.T) {
	ts := time.Date(2026, 7, 4, 2, 0, 0, 0, time.UTC)
	latency := uint32(2500)

	span := schema.Span{
		TraceID:   "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:    "00f067aa0ba902b7",
		ToolName:  "llm_call",
		Status:    schema.StatusOK,
		LatencyMs: latency,
		Ts:        ts,
	}

	rs := export.ConvertToResourceSpans([]schema.Span{span})
	ospan := rs[0].ScopeSpans[0].Spans[0]

	expectedStart := uint64(ts.UnixNano())
	expectedEnd := uint64(ts.Add(time.Duration(latency) * time.Millisecond).UnixNano())

	if ospan.StartTimeUnixNano != expectedStart {
		t.Errorf("start time: got %d, want %d", ospan.StartTimeUnixNano, expectedStart)
	}
	if ospan.EndTimeUnixNano != expectedEnd {
		t.Errorf("end time: got %d, want %d", ospan.EndTimeUnixNano, expectedEnd)
	}
}
