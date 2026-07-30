// SPDX-License-Identifier: MIT
package graph

// SpanRecord is the minimal representation of a tool call span read from ClickHouse.
// ParentSpanID is empty for root spans.
type SpanRecord struct {
	SpanID          string
	ParentSpanID    string
	ToolName        string
	Vendor          string
	DurationMs      uint64
	TraceDurationMs uint64
	HasError        bool
}
