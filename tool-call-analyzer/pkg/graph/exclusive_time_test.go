// SPDX-License-Identifier: MIT
package graph_test

import (
	"testing"

	"github.com/AkshantVats/tool-call-analyzer/pkg/graph"
)

func TestSingleSpan(t *testing.T) {
	g := graph.Build("trace-1", []graph.SpanRecord{
		span("a", "", "search_web", "openai", 100),
	})
	results := graph.ComputeExclusiveTimes(g)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ExclusiveTimeMs != 100 {
		t.Errorf("expected exclusive=100, got %d", results[0].ExclusiveTimeMs)
	}
	if results[0].ExclusiveTimeMs != results[0].TotalDurationMs {
		t.Errorf("leaf-only span should have exclusive == total")
	}
}

func TestSequentialChain(t *testing.T) {
	g := graph.Build("trace-2", []graph.SpanRecord{
		span("A", "", "step1", "openai", 100),
		span("B", "A", "step2", "openai", 100),
		span("C", "B", "step3", "openai", 100),
	})
	results := byID(graph.ComputeExclusiveTimes(g))
	if results["A"].ExclusiveTimeMs != 0 {
		t.Errorf("A: expected excl=0, got %d", results["A"].ExclusiveTimeMs)
	}
	if results["B"].ExclusiveTimeMs != 0 {
		t.Errorf("B: expected excl=0, got %d", results["B"].ExclusiveTimeMs)
	}
	if results["C"].ExclusiveTimeMs != 100 {
		t.Errorf("C: expected excl=100, got %d", results["C"].ExclusiveTimeMs)
	}
}

func TestParallelChildren(t *testing.T) {
	g := graph.Build("trace-3", []graph.SpanRecord{
		span("root", "", "plan_task", "openai", 500),
		span("B", "root", "search_web", "openai", 200),
		span("C", "root", "retrieve_doc", "openai", 250),
	})
	results := byID(graph.ComputeExclusiveTimes(g))
	if results["root"].ExclusiveTimeMs != 50 {
		t.Errorf("root: expected excl=50, got %d", results["root"].ExclusiveTimeMs)
	}
}

func TestLeafNodes(t *testing.T) {
	g := graph.Build("trace-4", []graph.SpanRecord{
		span("root", "", "plan_task", "openai", 100),
		span("B", "root", "search_web", "openai", 40),
		span("C", "root", "retrieve_doc", "openai", 60),
	})
	results := byID(graph.ComputeExclusiveTimes(g))
	if results["B"].ExclusiveTimeMs != 40 {
		t.Errorf("B: expected excl=40, got %d", results["B"].ExclusiveTimeMs)
	}
	if results["C"].ExclusiveTimeMs != 60 {
		t.Errorf("C: expected excl=60, got %d", results["C"].ExclusiveTimeMs)
	}
}

func TestPartialChildren(t *testing.T) {
	g := graph.Build("trace-5", []graph.SpanRecord{
		span("root", "", "plan_task", "openai", 500),
		span("B", "root", "search_web", "openai", 200),
		// second child intentionally missing from the trace
	})
	results := byID(graph.ComputeExclusiveTimes(g))
	if results["root"].ExclusiveTimeMs != 300 {
		t.Errorf("root: expected excl=300, got %d", results["root"].ExclusiveTimeMs)
	}
}

func TestZeroDurationRoot(t *testing.T) {
	g := graph.Build("trace-6", []graph.SpanRecord{
		span("root", "", "plan_task", "openai", 0),
		span("child", "root", "search_web", "openai", 100),
	})
	results := byID(graph.ComputeExclusiveTimes(g))
	if results["root"].ExclusiveTimeMs != 0 {
		t.Errorf("root: expected clamped excl=0, got %d", results["root"].ExclusiveTimeMs)
	}
}

func TestExclusiveTimeSortOrder(t *testing.T) {
	g := graph.Build("trace-7", []graph.SpanRecord{
		span("a", "", "tool_a", "openai", 10),
		span("b", "", "tool_b", "openai", 40),
		span("c", "", "tool_c", "openai", 30),
		span("d", "", "tool_d", "openai", 20),
	})
	results := graph.ComputeExclusiveTimes(g)
	want := []uint64{40, 30, 20, 10}
	for i, w := range want {
		if results[i].ExclusiveTimeMs != w {
			t.Errorf("position %d: expected %d, got %d", i, w, results[i].ExclusiveTimeMs)
		}
	}
}

func TestExclusiveTimeEmptyGraph(t *testing.T) {
	g := graph.Build("trace-8", nil)
	results := graph.ComputeExclusiveTimes(g)
	if len(results) != 0 {
		t.Errorf("expected empty slice, got %d results", len(results))
	}
}

func TestDeepChain(t *testing.T) {
	g := graph.Build("trace-9", []graph.SpanRecord{
		span("A", "", "step1", "openai", 100),
		span("B", "A", "step2", "openai", 100),
		span("C", "B", "step3", "openai", 100),
		span("D", "C", "step4", "openai", 100),
	})
	results := byID(graph.ComputeExclusiveTimes(g))
	for _, id := range []string{"A", "B", "C"} {
		if results[id].ExclusiveTimeMs != 0 {
			t.Errorf("%s: expected excl=0, got %d", id, results[id].ExclusiveTimeMs)
		}
	}
	if results["D"].ExclusiveTimeMs != 100 {
		t.Errorf("D: expected excl=100, got %d", results["D"].ExclusiveTimeMs)
	}
}

func TestMixedTopology(t *testing.T) {
	// root -> [A, B]; A -> C
	g := graph.Build("trace-10", []graph.SpanRecord{
		span("root", "", "plan_task", "openai", 50),
		span("A", "root", "search_web", "openai", 300),
		span("B", "root", "summarize", "anthropic", 20),
		span("C", "A", "retrieve_doc", "openai", 100),
	})
	results := byID(graph.ComputeExclusiveTimes(g))
	if results["A"].ExclusiveTimeMs != 200 {
		t.Errorf("A: expected excl=200, got %d", results["A"].ExclusiveTimeMs)
	}
	bottleneck := graph.ComputeExclusiveTimes(g)[0]
	if bottleneck.SpanID != "A" {
		t.Errorf("expected bottleneck A, got %s (excl=%d)", bottleneck.SpanID, bottleneck.ExclusiveTimeMs)
	}
}

func byID(results []graph.ExclusiveTimeResult) map[string]graph.ExclusiveTimeResult {
	m := make(map[string]graph.ExclusiveTimeResult, len(results))
	for _, r := range results {
		m[r.SpanID] = r
	}
	return m
}
