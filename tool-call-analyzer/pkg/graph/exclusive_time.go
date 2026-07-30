// SPDX-License-Identifier: MIT
package graph

import "slices"

// ExclusiveTimeResult holds the exclusive time for a single span.
type ExclusiveTimeResult struct {
	SpanID          string
	ToolName        string
	Vendor          string
	TotalDurationMs uint64
	ExclusiveTimeMs uint64 // TotalDurationMs - Σ(direct child durations)
	ChildCount      int
}

// ComputeExclusiveTimes returns an ExclusiveTimeResult for every node in g,
// sorted descending by ExclusiveTimeMs (bottleneck first). Exclusive time is
// the portion of a span's duration not accounted for by its direct children —
// the same subtraction a flame graph does visually, done numerically instead.
func ComputeExclusiveTimes(g *DependencyGraph) []ExclusiveTimeResult {
	results := make([]ExclusiveTimeResult, 0, len(g.Nodes))

	for id, node := range g.Nodes {
		var childSum uint64
		for _, childID := range g.Children[id] {
			if child, ok := g.Nodes[childID]; ok {
				childSum += child.DurationMs
			}
		}

		var exclusive uint64
		if node.DurationMs > childSum {
			exclusive = node.DurationMs - childSum
		}

		results = append(results, ExclusiveTimeResult{
			SpanID:          node.SpanID,
			ToolName:        node.ToolName,
			Vendor:          node.Vendor,
			TotalDurationMs: node.DurationMs,
			ExclusiveTimeMs: exclusive,
			ChildCount:      len(g.Children[id]),
		})
	}

	slices.SortFunc(results, func(a, b ExclusiveTimeResult) int {
		if a.ExclusiveTimeMs != b.ExclusiveTimeMs {
			if a.ExclusiveTimeMs > b.ExclusiveTimeMs {
				return -1
			}
			return 1
		}
		if a.SpanID < b.SpanID {
			return -1
		}
		if a.SpanID > b.SpanID {
			return 1
		}
		return 0
	})

	return results
}
