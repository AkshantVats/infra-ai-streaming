// SPDX-License-Identifier: MIT
package traceforge

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

const traceParentHeader = "traceparent"

// InjectTraceContext writes the W3C traceparent header from the active span in ctx.
// If there is no active span, the header is not written.
func InjectTraceContext(ctx context.Context, h http.Header) {
	s := spanFromContext(ctx)
	if s == nil {
		return
	}
	// traceparent = 00-<trace_id>-<span_id>-01
	h.Set(traceParentHeader, fmt.Sprintf("00-%s-%s-01", cleanID(s.TraceID), cleanID(s.SpanID)))
}

// ExtractTraceContext reads the W3C traceparent header and creates a new span
// in ctx that is a child of the remote span. Returns ctx unchanged if the
// header is absent or malformed.
func ExtractTraceContext(ctx context.Context, h http.Header) context.Context {
	raw := h.Get(traceParentHeader)
	if raw == "" {
		return ctx
	}
	parts := strings.Split(raw, "-")
	if len(parts) != 4 || parts[0] != "00" {
		return ctx
	}
	traceID, parentSpanID := parts[1], parts[2]
	if len(traceID) != 32 || len(parentSpanID) != 16 {
		// UUID format used internally has dashes — accept both forms
		if len(strings.ReplaceAll(traceID, "-", "")) != 32 {
			return ctx
		}
	}
	s := &Span{
		TraceID:      traceID,
		ParentSpanID: parentSpanID,
	}
	return withSpan(ctx, s)
}

// cleanID removes dashes from a UUID so it fits the 32-hex traceparent field.
func cleanID(id string) string {
	return strings.ReplaceAll(id, "-", "")
}
