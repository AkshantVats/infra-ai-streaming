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

	"github.com/AkshantVats/tool-call-analyzer/pkg/waterfall"
)

func runWaterfall(args []string) {
	fs := flag.NewFlagSet("waterfall", flag.ExitOnError)
	traceID := fs.String("trace-id", "", "Trace ID to analyze (required)")
	clickhouseURL := fs.String("clickhouse-url", envOrDefault("CLICKHOUSE_URL", "http://localhost:8123"), "ClickHouse HTTP URL")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *traceID == "" {
		fmt.Fprintln(os.Stderr, "error: --trace-id is required")
		fs.Usage()
		os.Exit(1)
	}

	spans, err := fetchSpanCosts(*clickhouseURL, *traceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching span costs: %v\n", err)
		os.Exit(1)
	}

	payload := waterfall.Build(*traceID, spans)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		fmt.Fprintf(os.Stderr, "error encoding payload: %v\n", err)
		os.Exit(1)
	}
}

// costRow is the ClickHouse JSON row shape for the waterfall query.
type costRow struct {
	ToolID   string  `json:"tool_id"`
	ToolName string  `json:"tool_name"`
	Vendor   string  `json:"vendor"`
	CostUSD  float64 `json:"cost_usd"`
}

func fetchSpanCosts(baseURL, traceID string) ([]waterfall.SpanCost, error) {
	query := fmt.Sprintf(
		"SELECT tool_id, tool_name, vendor, cost_usd FROM tool_calls WHERE trace_id = '%s' AND cost_usd > 0 ORDER BY cost_usd DESC FORMAT JSON",
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
		Data []costRow `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	spans := make([]waterfall.SpanCost, len(result.Data))
	for i, row := range result.Data {
		spans[i] = waterfall.SpanCost{
			SpanID:   row.ToolID,
			ToolName: row.ToolName,
			Vendor:   row.Vendor,
			CostUSD:  row.CostUSD,
		}
	}
	return spans, nil
}
