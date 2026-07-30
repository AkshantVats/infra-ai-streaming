// SPDX-License-Identifier: MIT
// Package waterfall aggregates per-span cost into a Grafana-compatible
// stacked bar payload — the numeric equivalent of a cost waterfall chart.
package waterfall

import "slices"

// SpanCost is the input to the waterfall builder.
type SpanCost struct {
	SpanID   string
	ToolName string
	Vendor   string
	CostUSD  float64
}

// WaterfallEntry is one bar in the Grafana stacked bar chart.
type WaterfallEntry struct {
	ToolName string  `json:"tool_name"`
	Vendor   string  `json:"vendor"`
	CostUSD  float64 `json:"cost_usd"`
}

// WaterfallPayload is the JSON output (Grafana Simple JSON datasource shape).
type WaterfallPayload struct {
	TraceID string           `json:"trace_id"`
	Data    []WaterfallEntry `json:"data"` // sorted by CostUSD descending
}

// Build aggregates SpanCost records by (tool_name, vendor), sorts descending
// by total CostUSD, and returns a WaterfallPayload.
func Build(traceID string, spans []SpanCost) WaterfallPayload {
	type key struct {
		toolName string
		vendor   string
	}

	totals := make(map[key]float64)
	order := make([]key, 0)
	for _, s := range spans {
		k := key{toolName: s.ToolName, vendor: s.Vendor}
		if _, seen := totals[k]; !seen {
			order = append(order, k)
		}
		totals[k] += s.CostUSD
	}

	entries := make([]WaterfallEntry, 0, len(order))
	for _, k := range order {
		entries = append(entries, WaterfallEntry{
			ToolName: k.toolName,
			Vendor:   k.vendor,
			CostUSD:  totals[k],
		})
	}

	slices.SortFunc(entries, func(a, b WaterfallEntry) int {
		if a.CostUSD != b.CostUSD {
			if a.CostUSD > b.CostUSD {
				return -1
			}
			return 1
		}
		if a.ToolName != b.ToolName {
			if a.ToolName < b.ToolName {
				return -1
			}
			return 1
		}
		if a.Vendor < b.Vendor {
			return -1
		}
		if a.Vendor > b.Vendor {
			return 1
		}
		return 0
	})

	return WaterfallPayload{
		TraceID: traceID,
		Data:    entries,
	}
}
