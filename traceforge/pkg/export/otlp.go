// SPDX-License-Identifier: MIT
// Package export converts canonical TraceForge spans to OTLP format and ships
// them to a downstream OpenTelemetry Collector via gRPC.
package export

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	colpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/AkshantVats/infra-ai-streaming/traceforge/pkg/schema"
)

// SpanExporter sends TraceForge spans to an OTLP Collector over gRPC.
type SpanExporter struct {
	conn   *grpc.ClientConn
	client colpb.TraceServiceClient
}

// New dials the OTel Collector at addr and returns a ready exporter.
func New(ctx context.Context, addr string) (*SpanExporter, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	return &SpanExporter{
		conn:   conn,
		client: colpb.NewTraceServiceClient(conn),
	}, nil
}

// Close shuts down the underlying gRPC connection.
func (e *SpanExporter) Close() error {
	return e.conn.Close()
}

// Export converts spans to OTLP ResourceSpans and ships them via gRPC.
func (e *SpanExporter) Export(ctx context.Context, spans []schema.Span) error {
	req := &colpb.ExportTraceServiceRequest{
		ResourceSpans: ConvertToResourceSpans(spans),
	}
	_, err := e.client.Export(ctx, req)
	return err
}

// ConvertToResourceSpans converts a slice of TraceForge spans to OTLP
// ResourceSpans. Exported so tests can inspect the conversion without a live
// Collector.
func ConvertToResourceSpans(spans []schema.Span) []*tracepb.ResourceSpans {
	if len(spans) == 0 {
		return nil
	}

	rs := &tracepb.ResourceSpans{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				strAttr("service.name", "agent-trace-collector"),
				strAttr("traceforge.version", "0.1.0"),
			},
		},
		ScopeSpans: []*tracepb.ScopeSpans{
			{
				Scope: &commonpb.InstrumentationScope{
					Name:    "traceforge",
					Version: "0.1.0",
				},
				Spans: make([]*tracepb.Span, 0, len(spans)),
			},
		},
	}

	ss := rs.ScopeSpans[0]
	for i := range spans {
		s := &spans[i]
		ospan, err := convertSpan(s)
		if err != nil {
			// Log but skip malformed spans rather than dropping the whole batch.
			continue
		}
		ss.Spans = append(ss.Spans, ospan)
	}

	return []*tracepb.ResourceSpans{rs}
}

func convertSpan(s *schema.Span) (*tracepb.Span, error) {
	traceID, err := parseHex(s.TraceID, 32)
	if err != nil {
		return nil, fmt.Errorf("trace_id: %w", err)
	}
	spanID, err := parseHex(s.SpanID, 16)
	if err != nil {
		return nil, fmt.Errorf("span_id: %w", err)
	}

	ospan := &tracepb.Span{
		TraceId:           traceID,
		SpanId:            spanID,
		Name:              s.ToolName,
		Kind:              spanKindFor(s),
		StartTimeUnixNano: uint64(s.Ts.UnixNano()),
		EndTimeUnixNano:   uint64(s.Ts.Add(time.Duration(s.LatencyMs) * time.Millisecond).UnixNano()),
		Attributes:        buildAttributes(s),
		Status:            buildStatus(s),
	}

	if s.ParentSpanID != nil && *s.ParentSpanID != "" {
		parentID, err := parseHex(*s.ParentSpanID, 16)
		if err == nil {
			ospan.ParentSpanId = parentID
		}
	}

	return ospan, nil
}

func buildAttributes(s *schema.Span) []*commonpb.KeyValue {
	attrs := []*commonpb.KeyValue{
		strAttr("traceforge.tool.name", s.ToolName),
		strAttr("traceforge.tool.category", s.Category()),
		strAttr("traceforge.status", s.Status),
		intAttr("traceforge.latency.ms", int64(s.LatencyMs)),
	}

	if s.Model != "" {
		attrs = append(attrs, strAttr("traceforge.model", s.Model))
		attrs = append(attrs, strAttr("gen_ai.request.model", s.Model))
	}
	if s.Tokens > 0 {
		attrs = append(attrs, intAttr("traceforge.tokens.total", int64(s.Tokens)))
		attrs = append(attrs, intAttr("gen_ai.usage.total_tokens", int64(s.Tokens)))
	}
	if s.CostUSD > 0 {
		attrs = append(attrs, dblAttr("traceforge.cost.usd", s.CostUSD))
	}
	if s.AgentID != "" {
		attrs = append(attrs, strAttr("traceforge.agent.id", s.AgentID))
	}
	if s.TenantID != "" {
		attrs = append(attrs, strAttr("traceforge.tenant.id", s.TenantID))
	}
	if s.ErrorMessage != nil {
		attrs = append(attrs, strAttr("traceforge.error.message", *s.ErrorMessage))
	}

	return attrs
}

func buildStatus(s *schema.Span) *tracepb.Status {
	switch s.Status {
	case schema.StatusOK:
		return &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK}
	case schema.StatusError, schema.StatusTimeout, schema.StatusCancelled:
		msg := ""
		if s.ErrorMessage != nil {
			msg = *s.ErrorMessage
		}
		return &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR, Message: msg}
	default:
		return &tracepb.Status{Code: tracepb.Status_STATUS_CODE_UNSET}
	}
}

func spanKindFor(s *schema.Span) tracepb.Span_SpanKind {
	switch s.Category() {
	case schema.CategoryGeneration:
		return tracepb.Span_SPAN_KIND_CLIENT
	case schema.CategoryExecution:
		return tracepb.Span_SPAN_KIND_INTERNAL
	default:
		return tracepb.Span_SPAN_KIND_INTERNAL
	}
}

// parseHex decodes a hex string into a byte slice, validating expected length.
func parseHex(s string, expectedLen int) ([]byte, error) {
	if len(s) != expectedLen {
		return nil, fmt.Errorf("expected %d hex chars, got %d", expectedLen, len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func strAttr(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   k,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}},
	}
}

func intAttr(k string, v int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   k,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: v}},
	}
}

func dblAttr(k string, v float64) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   k,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: v}},
	}
}
