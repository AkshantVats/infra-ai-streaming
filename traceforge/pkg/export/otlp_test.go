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

// TestConvertToResourceSpans_StatusRetry verifies that StatusRetry maps to UNSET status code
// (it is not in the StatusError/StatusTimeout/StatusCancelled group in buildStatus).
func TestConvertToResourceSpans_StatusRetry(t *testing.T) {
	span := schema.Span{
		TraceID:   "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:    "00f067aa0ba902b7",
		ToolName:  "api_call",
		Status:    schema.StatusRetry,
		LatencyMs: 500,
		Ts:        time.Now(),
	}
	rs := export.ConvertToResourceSpans([]schema.Span{span})
	ospan := rs[0].ScopeSpans[0].Spans[0]
	if ospan.Status == nil {
		t.Fatal("status must not be nil")
	}
	// StatusRetry falls to the default case → UNSET (0)
	if int(ospan.Status.Code) != 0 {
		t.Errorf("expected status code 0 (UNSET) for retry, got %d", ospan.Status.Code)
	}
}

// TestConvertToResourceSpans_StatusCancelled verifies StatusCancelled maps to ERROR status.
func TestConvertToResourceSpans_StatusCancelled(t *testing.T) {
	msg := "cancelled by user"
	span := schema.Span{
		TraceID:      "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:       "00f067aa0ba902b7",
		ToolName:     "llm_call",
		Status:       schema.StatusCancelled,
		LatencyMs:    100,
		Ts:           time.Now(),
		ErrorMessage: &msg,
	}
	rs := export.ConvertToResourceSpans([]schema.Span{span})
	ospan := rs[0].ScopeSpans[0].Spans[0]
	if int(ospan.Status.Code) != 2 {
		t.Errorf("expected ERROR code for cancelled, got %d", ospan.Status.Code)
	}
	if ospan.Status.Message != msg {
		t.Errorf("expected message %q, got %q", msg, ospan.Status.Message)
	}
}

// TestConvertToResourceSpans_StatusDefault verifies an unknown status maps to UNSET.
func TestConvertToResourceSpans_StatusUnknown(t *testing.T) {
	span := schema.Span{
		TraceID:   "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:    "00f067aa0ba902b7",
		ToolName:  "llm_call",
		Status:    "unknown_status",
		LatencyMs: 100,
		Ts:        time.Now(),
	}
	rs := export.ConvertToResourceSpans([]schema.Span{span})
	ospan := rs[0].ScopeSpans[0].Spans[0]
	if int(ospan.Status.Code) != 0 {
		t.Errorf("expected status code 0 (UNSET) for unknown status, got %d", ospan.Status.Code)
	}
}

// TestConvertToResourceSpans_ExecutionCategory verifies an execution tool gets INTERNAL kind.
func TestConvertToResourceSpans_ExecutionCategory(t *testing.T) {
	span := schema.Span{
		TraceID:   "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:    "00f067aa0ba902b7",
		ToolName:  "bash_exec", // CategoryExecution
		Status:    schema.StatusOK,
		LatencyMs: 200,
		Ts:        time.Now(),
	}
	rs := export.ConvertToResourceSpans([]schema.Span{span})
	if len(rs[0].ScopeSpans[0].Spans) != 1 {
		t.Fatalf("expected 1 span")
	}
	ospan := rs[0].ScopeSpans[0].Spans[0]
	// SPAN_KIND_INTERNAL = 1
	if int(ospan.Kind) != 1 {
		t.Errorf("expected SPAN_KIND_INTERNAL (1) for execution tool, got %d", ospan.Kind)
	}
}

// TestConvertToResourceSpans_GenerationCategory verifies llm_call gets CLIENT kind.
func TestConvertToResourceSpans_GenerationCategory(t *testing.T) {
	span := schema.Span{
		TraceID:   "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:    "00f067aa0ba902b7",
		ToolName:  "llm_call", // CategoryGeneration
		Status:    schema.StatusOK,
		LatencyMs: 1500,
		Ts:        time.Now(),
	}
	rs := export.ConvertToResourceSpans([]schema.Span{span})
	ospan := rs[0].ScopeSpans[0].Spans[0]
	// SPAN_KIND_CLIENT = 3
	if int(ospan.Kind) != 3 {
		t.Errorf("expected SPAN_KIND_CLIENT (3) for generation tool, got %d", ospan.Kind)
	}
}

// TestConvertToResourceSpans_InvalidSpanID verifies that a malformed span_id causes the span to be skipped.
func TestConvertToResourceSpans_InvalidSpanID(t *testing.T) {
	span := schema.Span{
		TraceID:   "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:    "INVALID", // not valid hex of length 16
		ToolName:  "llm_call",
		Status:    schema.StatusOK,
		LatencyMs: 100,
		Ts:        time.Now(),
	}
	rs := export.ConvertToResourceSpans([]schema.Span{span})
	if len(rs[0].ScopeSpans[0].Spans) != 0 {
		t.Errorf("expected 0 spans for invalid span_id, got %d", len(rs[0].ScopeSpans[0].Spans))
	}
}

// TestConvertToResourceSpans_InvalidParentHex verifies that an invalid parent span ID is silently ignored.
func TestConvertToResourceSpans_InvalidParentHex(t *testing.T) {
	parentID := "ZZZZZZZZZZZZZZZZ" // not valid hex
	span := schema.Span{
		TraceID:      "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:       "00f067aa0ba902b7",
		ParentSpanID: &parentID,
		ToolName:     "llm_call",
		Status:       schema.StatusOK,
		LatencyMs:    100,
		Ts:           time.Now(),
	}
	rs := export.ConvertToResourceSpans([]schema.Span{span})
	ospan := rs[0].ScopeSpans[0].Spans[0]
	// Invalid parent should be ignored (ParentSpanId remains nil/empty).
	if len(ospan.ParentSpanId) != 0 {
		t.Errorf("expected empty ParentSpanId for invalid hex, got %d bytes", len(ospan.ParentSpanId))
	}
}

// TestConvertToResourceSpans_RetrievalCategory verifies a retrieval tool (web_search)
// falls into the default spanKind branch and still produces a valid span.
func TestConvertToResourceSpans_RetrievalCategory(t *testing.T) {
	span := schema.Span{
		TraceID:   "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:    "00f067aa0ba902b7",
		ToolName:  "web_search", // CategoryRetrieval → default INTERNAL kind
		Status:    schema.StatusOK,
		LatencyMs: 300,
		Ts:        time.Now(),
	}
	rs := export.ConvertToResourceSpans([]schema.Span{span})
	if len(rs[0].ScopeSpans[0].Spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(rs[0].ScopeSpans[0].Spans))
	}
}

// TestNew_Close_ErrorPath verifies New can dial and Close releases resources.
// grpc.NewClient is lazy — it does not actually connect until the first RPC.
func TestNew_Close_ErrorPath(t *testing.T) {
	ctx := t.Context()
	exp, err := export.New(ctx, "localhost:0")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := exp.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestExport_ErrorPath verifies Export returns an error when the Collector is unreachable.
func TestExport_ErrorPath(t *testing.T) {
	ctx := t.Context()
	exp, err := export.New(ctx, "localhost:1") // no server on port 1
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer exp.Close() //nolint:errcheck

	spans := []schema.Span{
		{
			TraceID:   "4bf92f3577b34da6a3ce929d0e0e4736",
			SpanID:    "00f067aa0ba902b7",
			ToolName:  "llm_call",
			Status:    schema.StatusOK,
			LatencyMs: 100,
			Ts:        time.Now(),
		},
	}
	if err := exp.Export(ctx, spans); err == nil {
		t.Fatal("expected error when exporting to unreachable server")
	}
}
