// SPDX-License-Identifier: MIT
// Package clickhouse provides a lightweight HTTP writer for the ClickHouse HTTP API.
// Uses INSERT INTO ... FORMAT JSONEachRow -- no CGO, no native protocol required.
package clickhouse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/AkshantVats/tool-call-analyzer/pkg/kafka"
	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
)

// defaultFallbackDeadline bounds how long Insert waits on the ClickHouse
// HTTP write once a fallback producer is wired via SetFallback, before
// treating the write as "too slow" and buffering the span to Kafka instead.
// It only applies when a fallback is set -- without one, Insert uses the
// caller's own context exactly as it did before Day 43, so existing callers
// that don't opt into Kafka buffering see no behaviour change.
const defaultFallbackDeadline = 100 * time.Millisecond

// row is the ClickHouse-compatible flat representation of a ToolCall.
// Column names match the DDL exactly (snake_case).
type row struct {
	TraceID         string  `json:"trace_id"`
	ToolID          string  `json:"tool_id"`
	ToolName        string  `json:"tool_name"`
	Vendor          string  `json:"vendor"`
	Category        string  `json:"category"`
	ModelName       string  `json:"model_name"`
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	CostUSD         float64 `json:"cost_usd"`
	DurationMs      uint64  `json:"duration_ms"`
	TraceDurationMs uint64  `json:"trace_duration_ms"`
	HasError        uint8   `json:"has_error"`
	Status          string  `json:"status"`
	Timestamp       string  `json:"timestamp"` // "2006-01-02 15:04:05.000"
}

// Writer inserts ToolCall structs into the ClickHouse tool_calls table via HTTP JSONEachRow.
type Writer struct {
	baseURL    string
	table      string
	httpClient *http.Client

	// fallback, when wired via SetFallback, receives spans that Insert could
	// not write to ClickHouse within fallbackDeadline (or that returned an
	// HTTP error), instead of Insert returning a hard error. Nil means
	// "opted out" -- the pre-Day-43 behaviour, where a ClickHouse error is
	// simply returned to the caller.
	fallback         *kafka.FallbackProducer
	fallbackDeadline time.Duration
}

// New creates a Writer targeting baseURL (e.g. "http://localhost:8123").
func New(baseURL string) *Writer {
	return &Writer{
		baseURL:    baseURL,
		table:      "tool_calls",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// NewWithClient creates a Writer with an injected HTTP client (for testing).
func NewWithClient(baseURL string, client *http.Client) *Writer {
	return &Writer{baseURL: baseURL, table: "tool_calls", httpClient: client}
}

// SetFallback wires a Kafka fallback producer into the Writer. Once set,
// Insert buffers spans to fb instead of returning a hard error when the
// ClickHouse write fails or exceeds its write deadline (see
// SetFallbackDeadline). Passing nil disables fallback buffering again.
func (w *Writer) SetFallback(fb *kafka.FallbackProducer) {
	w.fallback = fb
}

// SetFallbackDeadline overrides defaultFallbackDeadline, the time budget
// Insert gives the ClickHouse HTTP write once a fallback producer is wired
// before treating it as "too slow" and buffering to Kafka instead. It has
// no effect when no fallback is set.
func (w *Writer) SetFallbackDeadline(d time.Duration) {
	w.fallbackDeadline = d
}

// Insert serializes tc as a JSONEachRow row and POSTs it to ClickHouse.
// durationMs is the tool span duration; traceDurationMs is the trace root span duration.
// hasError and status describe the outcome of the call -- ToolCall itself only carries
// the normalized request/response payload, so the caller (the span processor that already
// knows whether the tool returned an error) supplies them explicitly, the same way it
// already supplies the two duration values.
//
// When a fallback producer is wired (SetFallback), a ClickHouse write that
// errors or exceeds the fallback deadline is buffered to Kafka instead of
// being dropped -- Insert then returns nil, since the span was not lost,
// just diverted. Without a fallback producer, Insert behaves exactly as it
// did before Day 43: the caller's own context bounds the write, and a
// ClickHouse error is returned as-is.
func (w *Writer) Insert(ctx context.Context, tc types.ToolCall, durationMs, traceDurationMs uint64, hasError bool, status string) error {
	r := row{
		TraceID:         tc.TraceID,
		ToolID:          tc.ID,
		ToolName:        tc.Name,
		Vendor:          tc.Vendor,
		Category:        string(tc.Category),
		ModelName:       tc.Cost.ModelName,
		InputTokens:     tc.Cost.InputTokens,
		OutputTokens:    tc.Cost.OutputTokens,
		CostUSD:         tc.Cost.CostUSD,
		DurationMs:      durationMs,
		TraceDurationMs: traceDurationMs,
		HasError:        boolToUint8(hasError),
		Status:          status,
		Timestamp:       time.Now().UTC().Format("2006-01-02 15:04:05.000"),
	}

	payload, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("clickhouse: marshal failed: %w", err)
	}

	if w.fallback == nil {
		return w.doInsert(ctx, payload)
	}

	deadline := w.fallbackDeadline
	if deadline <= 0 {
		deadline = defaultFallbackDeadline
	}
	cctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	chErr := w.doInsert(cctx, payload)
	if chErr == nil {
		return nil
	}

	reason := "clickhouse_error"
	if cctx.Err() != nil {
		reason = "clickhouse_timeout"
	}

	// Use a fresh, un-timed-out context for the fallback send itself -- the
	// span already missed its ClickHouse budget, but that's not a reason to
	// also fail the Kafka buffer attempt.
	if fbErr := w.fallback.Send(context.Background(), rowToSpanEvent(r, reason)); fbErr != nil {
		return fmt.Errorf("clickhouse: %w; kafka fallback also failed: %v", chErr, fbErr)
	}
	return nil // buffered to Kafka -- not dropped
}

// doInsert performs the actual ClickHouse HTTP write. Split out of Insert so
// the fallback-deadline wrapping in Insert has a single call site to bound.
func (w *Writer) doInsert(ctx context.Context, payload []byte) error {
	url := fmt.Sprintf("%s/?query=INSERT%%20INTO%%20%s%%20FORMAT%%20JSONEachRow", w.baseURL, w.table)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("clickhouse: build request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("clickhouse: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("clickhouse: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// rowToSpanEvent converts a ClickHouse row into the Kafka fallback envelope,
// stamping it with why it was buffered.
func rowToSpanEvent(r row, reason string) kafka.SpanEvent {
	return kafka.SpanEvent{
		TraceID:          r.TraceID,
		ToolID:           r.ToolID,
		ToolName:         r.ToolName,
		Vendor:           r.Vendor,
		Category:         r.Category,
		ModelName:        r.ModelName,
		InputTokens:      r.InputTokens,
		OutputTokens:     r.OutputTokens,
		CostUSD:          r.CostUSD,
		DurationMs:       r.DurationMs,
		TraceDurationMs:  r.TraceDurationMs,
		HasError:         r.HasError,
		Status:           r.Status,
		Timestamp:        r.Timestamp,
		BufferedAtUnixMs: time.Now().UnixMilli(),
		BufferReason:     reason,
	}
}

func boolToUint8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
