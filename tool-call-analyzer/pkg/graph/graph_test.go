// SPDX-License-Identifier: MIT
package graph_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/AkshantVats/tool-call-analyzer/pkg/graph"
)

// helpers

func span(spanID, parentID, toolName, vendor string, durationMs uint64) graph.SpanRecord {
	return graph.SpanRecord{
		SpanID:       spanID,
		ParentSpanID: parentID,
		ToolName:     toolName,
		Vendor:       vendor,
		DurationMs:   durationMs,
	}
}

// Build tests

func TestBuildEmptySpans(t *testing.T) {
	g := graph.Build("trace-1", nil)
	if len(g.Nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(g.Nodes))
	}
}

func TestBuildSingleRootSpan(t *testing.T) {
	g := graph.Build("trace-1", []graph.SpanRecord{
		span("span-a", "", "search_web", "openai", 100),
	})
	if len(g.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(g.Nodes))
	}
	if len(g.Children["span-a"]) != 0 {
		t.Errorf("root span should have no children")
	}
	if len(g.Parents["span-a"]) != 0 {
		t.Errorf("root span should have no parents")
	}
}

func TestBuildParentChild(t *testing.T) {
	g := graph.Build("trace-2", []graph.SpanRecord{
		span("parent", "", "search_web", "openai", 200),
		span("child", "parent", "bash", "anthropic", 50),
	})
	if len(g.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(g.Nodes))
	}
	children := g.Children["parent"]
	if len(children) != 1 || children[0] != "child" {
		t.Errorf("parent should have child [child], got %v", children)
	}
	parents := g.Parents["child"]
	if len(parents) != 1 || parents[0] != "parent" {
		t.Errorf("child should have parent [parent], got %v", parents)
	}
}

func TestBuildLinearChain(t *testing.T) {
	g := graph.Build("trace-3", []graph.SpanRecord{
		span("A", "", "step1", "openai", 10),
		span("B", "A", "step2", "openai", 20),
		span("C", "B", "step3", "openai", 30),
	})
	if len(g.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(g.Nodes))
	}
	if len(g.Children["A"]) != 1 || g.Children["A"][0] != "B" {
		t.Errorf("A→B edge missing")
	}
	if len(g.Children["B"]) != 1 || g.Children["B"][0] != "C" {
		t.Errorf("B→C edge missing")
	}
}

func TestBuildMissingParentDropped(t *testing.T) {
	// Span references a parent not in the span list — edge is silently dropped.
	g := graph.Build("trace-4", []graph.SpanRecord{
		span("child", "ghost-parent", "tool_x", "openai", 100),
	})
	if len(g.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(g.Nodes))
	}
	if len(g.Parents["child"]) != 0 {
		t.Errorf("child should have no parents when parent is missing from span list")
	}
}

// Cycle detection tests

func TestHasCycleAcyclic(t *testing.T) {
	g := graph.Build("trace-5", []graph.SpanRecord{
		span("A", "", "s1", "openai", 10),
		span("B", "A", "s2", "openai", 20),
		span("C", "B", "s3", "openai", 30),
	})
	if g.HasCycle() {
		t.Error("expected no cycle in linear chain")
	}
}

func TestHasCycleCyclic(t *testing.T) {
	// Manually build a graph with a cycle A→B→C→A by bypassing Build
	g := &graph.DependencyGraph{
		TraceID: "trace-cycle",
		Nodes: map[string]*graph.Node{
			"A": {SpanID: "A", ToolName: "tool_a"},
			"B": {SpanID: "B", ToolName: "tool_b"},
			"C": {SpanID: "C", ToolName: "tool_c"},
		},
		Children: map[string][]string{
			"A": {"B"},
			"B": {"C"},
			"C": {"A"}, // cycle back to A
		},
		Parents: map[string][]string{
			"B": {"A"},
			"C": {"B"},
			"A": {"C"},
		},
	}
	if !g.HasCycle() {
		t.Error("expected cycle to be detected")
	}
}

// Topological sort tests

func TestTopologicalSort(t *testing.T) {
	g := graph.Build("trace-6", []graph.SpanRecord{
		span("A", "", "root_tool", "openai", 10),
		span("B", "A", "child_tool", "openai", 20),
		span("C", "A", "child_tool2", "openai", 15),
	})
	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 nodes in order, got %d", len(order))
	}
	// A must come before B and C
	posA, posB, posC := -1, -1, -1
	for i, id := range order {
		switch id {
		case "A":
			posA = i
		case "B":
			posB = i
		case "C":
			posC = i
		}
	}
	if posA > posB || posA > posC {
		t.Errorf("A must appear before B and C in topo sort, got order %v", order)
	}
}

func TestTopologicalSortCyclic(t *testing.T) {
	g := &graph.DependencyGraph{
		TraceID: "cycle-trace",
		Nodes: map[string]*graph.Node{
			"X": {SpanID: "X", ToolName: "tool_x"},
			"Y": {SpanID: "Y", ToolName: "tool_y"},
		},
		Children: map[string][]string{"X": {"Y"}, "Y": {"X"}},
		Parents:  map[string][]string{"Y": {"X"}, "X": {"Y"}},
	}
	_, err := g.TopologicalSort()
	if err == nil {
		t.Error("expected error for cyclic graph, got nil")
	}
}

// N+1 detection tests

func TestDetectN1EmptyGraph(t *testing.T) {
	g := graph.Build("trace-7", nil)
	findings := g.DetectN1(3)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for empty graph, got %d", len(findings))
	}
}

func TestDetectN1Found(t *testing.T) {
	spans := make([]graph.SpanRecord, 3)
	for i := range spans {
		spans[i] = span(fmt.Sprintf("s%d", i), "", "search_web", "openai", 100)
	}
	g := graph.Build("trace-8", spans)
	findings := g.DetectN1(3)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].ToolName != "search_web" || findings[0].Count != 3 {
		t.Errorf("unexpected finding: %+v", findings[0])
	}
}

func TestDetectN1NotFound(t *testing.T) {
	// Only 2 occurrences, threshold is 3 — should not trigger
	g := graph.Build("trace-9", []graph.SpanRecord{
		span("s1", "", "search_web", "openai", 100),
		span("s2", "s1", "search_web", "openai", 80),
	})
	findings := g.DetectN1(3)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for 2 occurrences at threshold 3, got %d", len(findings))
	}
}

func TestDetectN1MultipleTools(t *testing.T) {
	// Two tools each appearing 4 times — both should trigger
	spans := []graph.SpanRecord{}
	for i := 0; i < 4; i++ {
		spans = append(spans, span(fmt.Sprintf("web-%d", i), "", "search_web", "openai", 100))
		spans = append(spans, span(fmt.Sprintf("bash-%d", i), "", "bash", "anthropic", 50))
	}
	g := graph.Build("trace-10", spans)
	findings := g.DetectN1(3)
	if len(findings) != 2 {
		t.Errorf("expected 2 findings, got %d: %+v", len(findings), findings)
	}
}

// DOT output test

func TestToDOT(t *testing.T) {
	g := graph.Build("trace-dot", []graph.SpanRecord{
		span("parent", "", "search_web", "openai", 200),
		span("child", "parent", "bash", "anthropic", 50),
	})
	dot := g.ToDOT()
	if !strings.Contains(dot, "->") {
		t.Error("expected edge arrow in DOT output")
	}
	if !strings.Contains(dot, "parent") {
		t.Error("expected parent node in DOT output")
	}
	if !strings.Contains(dot, "child") {
		t.Error("expected child node in DOT output")
	}
	if !strings.Contains(dot, "search_web") {
		t.Error("expected tool name in DOT output")
	}
}
