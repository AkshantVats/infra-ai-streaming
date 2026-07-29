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

	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
)

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

// Insert serializes tc as a JSONEachRow row and POSTs it to ClickHouse.
// durationMs is the tool span duration; traceDurationMs is the trace root span duration.
// hasError and status describe the outcome of the call -- ToolCall itself only carries
// the normalized request/response payload, so the caller (the span processor that already
// knows whether the tool returned an error) supplies them explicitly, the same way it
// already supplies the two duration values.
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

func boolToUint8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
