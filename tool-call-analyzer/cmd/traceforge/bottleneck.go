// SPDX-License-Identifier: MIT
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/AkshantVats/tool-call-analyzer/pkg/graph"
)

func runBottleneck(args []string) {
	fs := flag.NewFlagSet("bottleneck", flag.ExitOnError)
	traceID := fs.String("trace-id", "", "Trace ID to analyze (required)")
	top := fs.Int("top", 5, "Number of ranked spans to print")
	format := fs.String("format", "text", "Output format: text or json")
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
	results := graph.ComputeExclusiveTimes(g)
	if *top > 0 && *top < len(results) {
		results = results[:*top]
	}

	switch strings.ToLower(*format) {
	case "json":
		printBottleneckJSON(os.Stdout, *traceID, results)
	default:
		printBottleneckText(os.Stdout, *traceID, len(g.Nodes), results)
	}
}

// bottleneckJSONEntry is one ranked entry in the JSON output.
type bottleneckJSONEntry struct {
	Rank            int    `json:"rank"`
	SpanID          string `json:"span_id"`
	ToolName        string `json:"tool_name"`
	Vendor          string `json:"vendor"`
	ExclusiveTimeMs uint64 `json:"exclusive_time_ms"`
	TotalDurationMs uint64 `json:"total_duration_ms"`
}

func printBottleneckJSON(w io.Writer, traceID string, results []graph.ExclusiveTimeResult) {
	entries := make([]bottleneckJSONEntry, len(results))
	for i, r := range results {
		entries[i] = bottleneckJSONEntry{
			Rank:            i + 1,
			SpanID:          r.SpanID,
			ToolName:        r.ToolName,
			Vendor:          r.Vendor,
			ExclusiveTimeMs: r.ExclusiveTimeMs,
			TotalDurationMs: r.TotalDurationMs,
		}
	}

	out := struct {
		TraceID    string                `json:"trace_id"`
		Bottleneck *bottleneckJSONEntry  `json:"bottleneck,omitempty"`
		Ranked     []bottleneckJSONEntry `json:"ranked"`
	}{
		TraceID: traceID,
		Ranked:  entries,
	}
	if len(entries) > 0 {
		out.Bottleneck = &entries[0]
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func printBottleneckText(w io.Writer, traceID string, spanCount int, results []graph.ExclusiveTimeResult) {
	var b strings.Builder
	fmt.Fprintf(&b, "=== TraceForge Bottleneck Report ===\n")
	fmt.Fprintf(&b, "Trace ID  : %s\n", traceID)
	fmt.Fprintf(&b, "Spans     : %d\n\n", spanCount)
	fmt.Fprintf(&b, "Rank  SpanID      Tool             Vendor     Excl. Time   Total Time\n")
	for i, r := range results {
		fmt.Fprintf(&b, "%-5d %-11s %-16s %-10s %8dms   %6dms\n",
			i+1, truncate(r.SpanID, 11), r.ToolName, r.Vendor, r.ExclusiveTimeMs, r.TotalDurationMs)
	}
	if len(results) > 0 {
		top := results[0]
		fmt.Fprintf(&b, "\nBottleneck: %s (%s) — %dms exclusive time\n", top.ToolName, top.Vendor, top.ExclusiveTimeMs)
	}
	_, _ = io.WriteString(w, b.String())
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
