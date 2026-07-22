// SPDX-License-Identifier: MIT
package traceforge

import (
	"context"
	"encoding/json"
	"time"
)

type contextKey struct{}

// Status represents the outcome of a traced operation.
type Status string

const (
	StatusOK    Status = "ok"
	StatusError Status = "error"
)

// Span captures one tool call or sub-agent invocation.
type Span struct {
	TraceID      string            `json:"trace_id"`
	SpanID       string            `json:"span_id"`
	ParentSpanID string            `json:"parent_span_id,omitempty"`
	ToolName     string            `json:"tool_name"`
	StartNs      int64             `json:"start_ns"`
	EndNs        int64             `json:"end_ns,omitempty"`
	LatencyMs    float64           `json:"latency_ms,omitempty"`
	Status       Status            `json:"status,omitempty"`
	ErrorMsg     string            `json:"error_msg,omitempty"`
	Model        string            `json:"model,omitempty"`
	InputTokens  int               `json:"input_tokens,omitempty"`
	OutputTokens int               `json:"output_tokens,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
}

// spanFromContext returns the active span stored in ctx, or nil.
func spanFromContext(ctx context.Context) *Span {
	v := ctx.Value(contextKey{})
	if v == nil {
		return nil
	}
	s, _ := v.(*Span)
	return s
}

// SpanFromContext returns the active span or nil (exported for tests).
func SpanFromContext(ctx context.Context) *Span {
	return spanFromContext(ctx)
}

// withSpan stores span in ctx.
func withSpan(ctx context.Context, s *Span) context.Context {
	return context.WithValue(ctx, contextKey{}, s)
}

// MarshalJSON serialises the span for emission.
func (s *Span) MarshalJSON() ([]byte, error) {
	type alias Span
	return json.Marshal((*alias)(s))
}

// finish records end time and latency.
func (s *Span) finish(status Status, errMsg string) {
	s.EndNs = time.Now().UnixNano()
	s.LatencyMs = float64(s.EndNs-s.StartNs) / 1e6
	s.Status = status
	s.ErrorMsg = errMsg
}
