// SPDX-License-Identifier: MIT
// Package schema defines the agent execution span model for TraceForge.
package schema

import "time"

// Status values for Span.Status.
const (
	StatusOK        = "ok"
	StatusError     = "error"
	StatusRetry     = "retry"
	StatusTimeout   = "timeout"
	StatusCancelled = "cancelled"
)

// ToolCategory groups tool names by function.
const (
	CategoryRetrieval  = "retrieval"
	CategoryExecution  = "execution"
	CategoryMemory     = "memory"
	CategoryGeneration = "generation"
)

// ToolCategory maps each tool name to its category.
var ToolCategory = map[string]string{
	"web_search":          CategoryRetrieval,
	"vector_search":       CategoryRetrieval,
	"document_read":       CategoryRetrieval,
	"db_query":            CategoryRetrieval,
	"code_interpreter":    CategoryExecution,
	"bash_exec":           CategoryExecution,
	"api_call":            CategoryExecution,
	"http_request":        CategoryExecution,
	"memory_read":         CategoryMemory,
	"memory_write":        CategoryMemory,
	"context_window_read": CategoryMemory,
	"llm_call":            CategoryGeneration,
	"image_gen":           CategoryGeneration,
	"embedding_gen":       CategoryGeneration,
}

// Span represents a single agent execution step collected by TraceForge.
// One Span per tool call; a complete agent run is a DAG of Spans sharing
// the same TraceID.
type Span struct {
	// TraceID is a UUID v4 shared by all spans in a single agent run.
	TraceID string `json:"trace_id"`

	// SpanID is a UUID v4 unique to this span.
	SpanID string `json:"span_id"`

	// ParentSpanID is nil for the root span; otherwise the SpanID of the
	// calling span.
	ParentSpanID *string `json:"parent_span_id,omitempty"`

	// ToolName identifies the logical tool that executed (e.g. "llm_call",
	// "web_search", "code_interpreter"). See ToolCategory for the full
	// taxonomy.
	ToolName string `json:"tool_name"`

	// Model is the LLM model identifier (e.g. "gpt-4o", "claude-3-5-sonnet").
	// Empty string for non-LLM spans.
	Model string `json:"model"`

	// Tokens is the total token count (input + output) for llm_call spans.
	// Zero for all other tool types.
	Tokens uint32 `json:"tokens"`

	// CostUSD is the inferred cost in US dollars for this span.
	// Zero for non-LLM spans.
	CostUSD float64 `json:"cost_usd"`

	// Status is one of: ok, error, retry, timeout, cancelled.
	Status string `json:"status"`

	// LatencyMs is the wall-clock duration from span start to span end.
	LatencyMs uint32 `json:"latency_ms"`

	// Ts is the span start timestamp.
	Ts time.Time `json:"ts"`

	// AgentID identifies the agent type or class (e.g. "research_agent").
	AgentID string `json:"agent_id"`

	// TenantID scopes the span to a tenant or workspace.
	TenantID string `json:"tenant_id"`

	// ErrorMessage holds the error description when Status == StatusError.
	ErrorMessage *string `json:"error_message,omitempty"`

	// Metadata is a free-form JSON blob for span-specific attributes such
	// as prompt hash or tool input fingerprint.
	Metadata string `json:"metadata"`
}

// IsRoot returns true when this span has no parent — i.e. it is the root
// span of an agent run.
func (s *Span) IsRoot() bool {
	return s.ParentSpanID == nil
}

// IsLLMCall returns true when this span represents a model inference call.
func (s *Span) IsLLMCall() bool {
	return s.ToolName == "llm_call"
}

// Category returns the tool category for this span's ToolName.
func (s *Span) Category() string {
	if cat, ok := ToolCategory[s.ToolName]; ok {
		return cat
	}
	return "unknown"
}

// Validate checks that required fields are non-empty and that Status is a
// known value.
func (s *Span) Validate() error {
	if s.TraceID == "" {
		return ErrMissingTraceID
	}
	if s.SpanID == "" {
		return ErrMissingSpanID
	}
	if s.ToolName == "" {
		return ErrMissingToolName
	}
	switch s.Status {
	case StatusOK, StatusError, StatusRetry, StatusTimeout, StatusCancelled:
		// valid
	default:
		return ErrInvalidStatus
	}
	return nil
}
