// SPDX-License-Identifier: MIT
package schema_test

import (
	"testing"
	"time"

	"github.com/AkshantVats/infra-ai-streaming/traceforge/pkg/schema"
)

func ptr(s string) *string { return &s }

func validSpan() schema.Span {
	return schema.Span{
		TraceID:   "trace-abc-123",
		SpanID:    "span-def-456",
		ToolName:  "llm_call",
		Model:     "gpt-4o",
		Tokens:    512,
		CostUSD:   0.0025,
		Status:    schema.StatusOK,
		LatencyMs: 340,
		Ts:        time.Now(),
		AgentID:   "research_agent",
		TenantID:  "tenant-1",
		Metadata:  `{}`,
	}
}

func TestSpan_Validate_Valid(t *testing.T) {
	s := validSpan()
	if err := s.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSpan_Validate_MissingTraceID(t *testing.T) {
	s := validSpan()
	s.TraceID = ""
	if err := s.Validate(); err != schema.ErrMissingTraceID {
		t.Fatalf("expected ErrMissingTraceID, got %v", err)
	}
}

func TestSpan_Validate_MissingSpanID(t *testing.T) {
	s := validSpan()
	s.SpanID = ""
	if err := s.Validate(); err != schema.ErrMissingSpanID {
		t.Fatalf("expected ErrMissingSpanID, got %v", err)
	}
}

func TestSpan_Validate_InvalidStatus(t *testing.T) {
	s := validSpan()
	s.Status = "unknown_state"
	if err := s.Validate(); err != schema.ErrInvalidStatus {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestSpan_IsRoot_True(t *testing.T) {
	s := validSpan()
	s.ParentSpanID = nil
	if !s.IsRoot() {
		t.Fatal("expected IsRoot() = true when ParentSpanID is nil")
	}
}

func TestSpan_IsRoot_False(t *testing.T) {
	s := validSpan()
	s.ParentSpanID = ptr("parent-span-id")
	if s.IsRoot() {
		t.Fatal("expected IsRoot() = false when ParentSpanID is set")
	}
}

func TestSpan_IsLLMCall(t *testing.T) {
	s := validSpan()
	if !s.IsLLMCall() {
		t.Fatal("expected IsLLMCall() = true for tool_name=llm_call")
	}
	s.ToolName = "web_search"
	if s.IsLLMCall() {
		t.Fatal("expected IsLLMCall() = false for tool_name=web_search")
	}
}

func TestSpan_Category(t *testing.T) {
	cases := []struct {
		tool string
		want string
	}{
		{"llm_call", schema.CategoryGeneration},
		{"web_search", schema.CategoryRetrieval},
		{"code_interpreter", schema.CategoryExecution},
		{"memory_read", schema.CategoryMemory},
		{"unknown_tool", "unknown"},
	}
	for _, tc := range cases {
		s := validSpan()
		s.ToolName = tc.tool
		if got := s.Category(); got != tc.want {
			t.Errorf("tool=%s: got category %q, want %q", tc.tool, got, tc.want)
		}
	}
}

func TestSpan_AllStatuses(t *testing.T) {
	for _, status := range []string{
		schema.StatusOK,
		schema.StatusError,
		schema.StatusRetry,
		schema.StatusTimeout,
		schema.StatusCancelled,
	} {
		s := validSpan()
		s.Status = status
		if err := s.Validate(); err != nil {
			t.Errorf("status=%s should be valid, got error: %v", status, err)
		}
	}
}
