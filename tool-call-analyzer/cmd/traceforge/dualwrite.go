// SPDX-License-Identifier: MIT
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/AkshantVats/tool-call-analyzer/pkg/lensai"
	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
)

// runDualWrite reads a trace's tool calls back out of ClickHouse and
// dual-writes each one's cost onto LensAI's ingest pipeline, so the unified
// tenant Grafana board (grafana/unified-tenant-cost.json) can show tool
// cost alongside LLM inference cost for the same tenant/trace.
func runDualWrite(args []string) {
	fs := flag.NewFlagSet("dual-write", flag.ExitOnError)
	traceID := fs.String("trace-id", "", "Trace ID to dual-write (required)")
	tenantID := fs.String("tenant-id", "", "Tenant ID to attribute the cost to (required)")
	clickhouseURL := fs.String("clickhouse-url", envOrDefault("CLICKHOUSE_URL", "http://localhost:8123"), "ClickHouse HTTP URL")
	lensaiURL := fs.String("lensai-url", envOrDefault("LENSAI_INGEST_URL", "http://localhost:8080/ingest"), "LensAI ingest URL")
	dryRun := fs.Bool("dry-run", false, "Print the events that would be sent instead of posting them")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *traceID == "" {
		fmt.Fprintln(os.Stderr, "error: --trace-id is required")
		fs.Usage()
		os.Exit(1)
	}
	if *tenantID == "" {
		fmt.Fprintln(os.Stderr, "error: --tenant-id is required")
		fs.Usage()
		os.Exit(1)
	}

	rows, err := fetchToolCalls(*clickhouseURL, *traceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching tool calls: %v\n", err)
		os.Exit(1)
	}

	if len(rows) == 0 {
		fmt.Fprintf(os.Stderr, "no tool calls found for trace-id: %s\n", *traceID)
		os.Exit(1)
	}

	if *dryRun {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		for _, row := range rows {
			evt := lensai.ToEvent(row.toolCall(*traceID), *tenantID, row.HasError == 1, row.Status)
			if err := enc.Encode(evt); err != nil {
				fmt.Fprintf(os.Stderr, "error encoding event: %v\n", err)
				os.Exit(1)
			}
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	writer := lensai.New(*lensaiURL)
	written := 0
	for _, row := range rows {
		tc := row.toolCall(*traceID)
		if err := writer.Insert(ctx, tc, *tenantID, row.HasError == 1, row.Status); err != nil {
			fmt.Fprintf(os.Stderr, "error dual-writing tool_id=%s: %v\n", row.ToolID, err)
			os.Exit(1)
		}
		written++
	}

	fmt.Fprintf(os.Stdout, "dual-wrote %d tool call(s) for trace-id=%s tenant-id=%s to %s\n", written, *traceID, *tenantID, *lensaiURL)
}

// toolCallRow is the ClickHouse JSON row shape for the dual-write query --
// a superset of costRow's columns since LensAI's envelope also needs
// tokens, model name, duration, and outcome.
type toolCallRow struct {
	ToolID       string  `json:"tool_id"`
	ToolName     string  `json:"tool_name"`
	Vendor       string  `json:"vendor"`
	ModelName    string  `json:"model_name"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
	DurationMs   uint64  `json:"duration_ms"`
	HasError     int     `json:"has_error"`
	Status       string  `json:"status"`
}

// toolCall converts a ClickHouse row back into the canonical ToolCall shape
// so it can go through the same lensai.ToEvent conversion the rest of the
// pipeline uses.
func (r toolCallRow) toolCall(traceID string) types.ToolCall {
	return types.ToolCall{
		ID:         r.ToolID,
		TraceID:    traceID,
		Name:       r.ToolName,
		Vendor:     r.Vendor,
		DurationMs: int64(r.DurationMs),
		Cost: types.CostEstimate{
			InputTokens:  r.InputTokens,
			OutputTokens: r.OutputTokens,
			ModelName:    r.ModelName,
			CostUSD:      r.CostUSD,
		},
	}
}

func fetchToolCalls(baseURL, traceID string) ([]toolCallRow, error) {
	query := fmt.Sprintf(
		"SELECT tool_id, tool_name, vendor, model_name, input_tokens, output_tokens, cost_usd, duration_ms, has_error, status "+
			"FROM tool_calls WHERE trace_id = '%s' ORDER BY timestamp ASC FORMAT JSON",
		strings.ReplaceAll(traceID, "'", "''"),
	)

	reqURL := baseURL + "/?query=" + url.QueryEscape(query)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("clickhouse returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []toolCallRow `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return result.Data, nil
}
