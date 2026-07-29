// SPDX-License-Identifier: MIT
package traceforge

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// StartSpan begins a new span for toolName. It inherits trace_id and
// parent_span_id from any active span in ctx.
// Returns a new context carrying the span and the span itself.
func StartSpan(ctx context.Context, toolName string) (context.Context, *Span) {
	parent := spanFromContext(ctx)
	s := &Span{
		SpanID:   newID(),
		ToolName: toolName,
		StartNs:  time.Now().UnixNano(),
		Tags:     make(map[string]string),
	}
	if parent != nil {
		s.TraceID = parent.TraceID
		s.ParentSpanID = parent.SpanID
	} else {
		s.TraceID = newID()
	}
	return withSpan(ctx, s), s
}

// EndSpan finalises span with the given status and optional error.
// It emits the span to the configured backend (HTTP + Kafka).
func EndSpan(ctx context.Context, span *Span, status Status, err error) {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	span.finish(status, errMsg)
	emit(ctx, span)
}

// newID generates a URL-safe random 128-bit identifier.
func newID() string {
	return uuid.New().String()
}
