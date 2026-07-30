// SPDX-License-Identifier: MIT
// Package graph builds a directed dependency graph from agent trace spans and
// provides cycle detection, topological sort, N+1 detection, and DOT output.
package graph

import (
	"fmt"
	"sort"
	"strings"
)

// Node is a single tool call span in the dependency graph.
type Node struct {
	SpanID     string
	ToolName   string
	Vendor     string
	DurationMs uint64
}

// N1Finding represents a tool that was called more times in a trace than the N+1 threshold.
type N1Finding struct {
	ToolName string
	Count    int
	TraceID  string
}

// DependencyGraph is a directed graph where nodes are spans and edges are parent→child
// relationships derived from ParentSpanID fields.
type DependencyGraph struct {
	TraceID  string
	Nodes    map[string]*Node    // keyed by SpanID
	Children map[string][]string // SpanID → []child SpanID
	Parents  map[string][]string // SpanID → []parent SpanID
}

// Build constructs a DependencyGraph from a slice of SpanRecords.
// Spans with empty ParentSpanID are root nodes.
// Spans referencing a ParentSpanID not present in spans are still added as nodes;
// the missing parent edge is silently dropped.
func Build(traceID string, spans []SpanRecord) *DependencyGraph {
	g := &DependencyGraph{
		TraceID:  traceID,
		Nodes:    make(map[string]*Node),
		Children: make(map[string][]string),
		Parents:  make(map[string][]string),
	}
	for i := range spans {
		s := &spans[i]
		g.Nodes[s.SpanID] = &Node{
			SpanID:     s.SpanID,
			ToolName:   s.ToolName,
			Vendor:     s.Vendor,
			DurationMs: s.DurationMs,
		}
	}
	for i := range spans {
		s := &spans[i]
		if s.ParentSpanID == "" {
			continue
		}
		if _, parentExists := g.Nodes[s.ParentSpanID]; !parentExists {
			continue
		}
		g.Children[s.ParentSpanID] = append(g.Children[s.ParentSpanID], s.SpanID)
		g.Parents[s.SpanID] = append(g.Parents[s.SpanID], s.ParentSpanID)
	}
	return g
}

// HasCycle returns true if the graph contains at least one directed cycle.
// Uses iterative DFS with an explicit in-stack set to avoid goroutine-stack overflow
// on deep traces.
func (g *DependencyGraph) HasCycle() bool {
	visited := make(map[string]bool, len(g.Nodes))
	inStack := make(map[string]bool, len(g.Nodes))

	var dfs func(id string) bool
	dfs = func(id string) bool {
		visited[id] = true
		inStack[id] = true
		for _, child := range g.Children[id] {
			if !visited[child] {
				if dfs(child) {
					return true
				}
			} else if inStack[child] {
				return true
			}
		}
		inStack[id] = false
		return false
	}

	for id := range g.Nodes {
		if !visited[id] {
			if dfs(id) {
				return true
			}
		}
	}
	return false
}

// TopologicalSort returns node IDs in topological order (parents before children).
// Returns an error if the graph contains a cycle.
// Uses Kahn's algorithm (in-degree queue) for deterministic output.
func (g *DependencyGraph) TopologicalSort() ([]string, error) {
	if g.HasCycle() {
		return nil, fmt.Errorf("graph: topological sort failed — cycle detected in trace %s", g.TraceID)
	}

	inDegree := make(map[string]int, len(g.Nodes))
	for id := range g.Nodes {
		inDegree[id] = len(g.Parents[id])
	}

	queue := make([]string, 0, len(g.Nodes))
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue) // deterministic ordering of roots

	result := make([]string, 0, len(g.Nodes))
	for len(queue) > 0 {
		// pop front
		cur := queue[0]
		queue = queue[1:]
		result = append(result, cur)

		children := make([]string, len(g.Children[cur]))
		copy(children, g.Children[cur])
		sort.Strings(children)
		for _, child := range children {
			inDegree[child]--
			if inDegree[child] == 0 {
				queue = append(queue, child)
			}
		}
	}
	return result, nil
}

// DetectN1 returns findings for any tool_name that appears in at least minCount
// distinct spans within this trace. threshold=3 catches the classic N+1 pattern.
func (g *DependencyGraph) DetectN1(minCount int) []N1Finding {
	counts := make(map[string]int, len(g.Nodes))
	for _, node := range g.Nodes {
		counts[node.ToolName]++
	}

	var findings []N1Finding
	for toolName, count := range counts {
		if count >= minCount {
			findings = append(findings, N1Finding{
				ToolName: toolName,
				Count:    count,
				TraceID:  g.TraceID,
			})
		}
	}
	// Sort for deterministic output in tests and CLI
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Count != findings[j].Count {
			return findings[i].Count > findings[j].Count
		}
		return findings[i].ToolName < findings[j].ToolName
	})
	return findings
}

// ToDOT renders the graph as a Graphviz DOT string.
// Node labels include tool_name and duration_ms.
func (g *DependencyGraph) ToDOT() string {
	var b strings.Builder
	fmt.Fprintf(&b, "digraph trace_%s {\n", sanitizeDOTID(g.TraceID))
	fmt.Fprintf(&b, "  label=\"Trace %s\";\n", g.TraceID)
	fmt.Fprintf(&b, "  rankdir=TB;\n")

	ids := make([]string, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		node := g.Nodes[id]
		fmt.Fprintf(&b, "  %q [label=\"%s\\n%s\\n%dms\"];\n",
			id, node.ToolName, node.Vendor, node.DurationMs)
	}

	for _, parentID := range ids {
		children := make([]string, len(g.Children[parentID]))
		copy(children, g.Children[parentID])
		sort.Strings(children)
		for _, childID := range children {
			fmt.Fprintf(&b, "  %q -> %q;\n", parentID, childID)
		}
	}
	fmt.Fprintf(&b, "}\n")
	return b.String()
}

func sanitizeDOTID(s string) string {
	return strings.NewReplacer("-", "_", ".", "_").Replace(s)
}
