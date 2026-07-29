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

	"github.com/AkshantVats/tool-call-analyzer/pkg/graph"
)

func runGraph(args []string) {
	fs := flag.NewFlagSet("graph", flag.ExitOnError)
	traceID := fs.String("trace-id", "", "Trace ID to analyze (required)")
	minN1 := fs.Int("min-n1-count", 3, "Minimum tool call repetitions to flag as N+1")
	format := fs.String("format", "text", "Output format: text or dot")
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

	spans, err := fetchSpans(*clickhouseURL, *traceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching spans: %v\n", err)
		os.Exit(1)
	}

	if len(spans) == 0 {
		fmt.Fprintf(os.Stderr, "no spans found for trace-id: %s\n", *traceID)
		os.Exit(1)
	}

	g := graph.Build(*traceID, spans)

	switch strings.ToLower(*format) {
	case "dot":
		fmt.Print(g.ToDOT())
	default:
		printTextReport(os.Stdout, g, *minN1)
	}
}

// chRow is the ClickHouse JSON row shape returned by the HTTP API.
type chRow struct {
	TraceID         string `json:"trace_id"`
	ToolID          string `json:"tool_id"`
	ToolName        string `json:"tool_name"`
	Vendor          string `json:"vendor"`
	DurationMs      uint64 `json:"duration_ms"`
	TraceDurationMs uint64 `json:"trace_duration_ms"`
	HasError        int    `json:"has_error"`
}

func fetchSpans(baseURL, traceID string) ([]graph.SpanRecord, error) {
	query := fmt.Sprintf(
		"SELECT trace_id, tool_id, tool_name, vendor, duration_ms, trace_duration_ms, has_error FROM tool_calls WHERE trace_id = '%s' FORMAT JSON",
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("clickhouse returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []chRow `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	spans := make([]graph.SpanRecord, len(result.Data))
	for i, row := range result.Data {
		spans[i] = graph.SpanRecord{
			SpanID:          row.ToolID,
			ParentSpanID:    "", // tool_calls table doesn't store parent_span_id; all spans are siblings
			ToolName:        row.ToolName,
			Vendor:          row.Vendor,
			DurationMs:      row.DurationMs,
			TraceDurationMs: row.TraceDurationMs,
			HasError:        row.HasError != 0,
		}
	}
	return spans, nil
}

func printTextReport(w io.Writer, g *graph.DependencyGraph, minN1 int) {
	fmt.Fprintf(w, "=== TraceForge Graph Report ===\n")
	fmt.Fprintf(w, "Trace ID : %s\n", g.TraceID)
	fmt.Fprintf(w, "Nodes    : %d\n", len(g.Nodes))

	hasCycle := g.HasCycle()
	fmt.Fprintf(w, "Cycle    : %v\n\n", hasCycle)

	if !hasCycle {
		order, err := g.TopologicalSort()
		if err == nil {
			fmt.Fprintf(w, "Execution order:\n")
			for i, id := range order {
				node := g.Nodes[id]
				fmt.Fprintf(w, "  %d. [%s] %s (%s) %dms\n",
					i+1, id[:min8(len(id))], node.ToolName, node.Vendor, node.DurationMs)
			}
			fmt.Fprintln(w)
		}
	}

	findings := g.DetectN1(minN1)
	if len(findings) == 0 {
		fmt.Fprintf(w, "N+1 check: clean (no tool called >= %d times)\n", minN1)
	} else {
		fmt.Fprintf(w, "N+1 alerts (threshold=%d):\n", minN1)
		for _, f := range findings {
			fmt.Fprintf(w, "  ⚠  %s called %d times — possible N+1 pattern\n", f.ToolName, f.Count)
		}
	}
}

func min8(n int) int {
	if n < 8 {
		return n
	}
	return 8
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
