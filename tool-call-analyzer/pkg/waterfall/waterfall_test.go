// SPDX-License-Identifier: MIT
package waterfall_test

import (
	"encoding/json"
	"testing"

	"github.com/AkshantVats/tool-call-analyzer/pkg/waterfall"
)

func TestEmptySpans(t *testing.T) {
	payload := waterfall.Build("trace-1", nil)
	if len(payload.Data) != 0 {
		t.Errorf("expected empty data, got %d entries", len(payload.Data))
	}
	if payload.Data == nil {
		t.Errorf("expected non-nil empty slice, got nil")
	}
}

func TestWaterfallSingleSpan(t *testing.T) {
	payload := waterfall.Build("trace-2", []waterfall.SpanCost{
		{SpanID: "a", ToolName: "search_web", Vendor: "openai", CostUSD: 0.005},
	})
	if len(payload.Data) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(payload.Data))
	}
	if payload.Data[0].CostUSD != 0.005 {
		t.Errorf("expected cost=0.005, got %f", payload.Data[0].CostUSD)
	}
}

func TestAggregation(t *testing.T) {
	payload := waterfall.Build("trace-3", []waterfall.SpanCost{
		{SpanID: "a", ToolName: "search_web", Vendor: "openai", CostUSD: 0.003},
		{SpanID: "b", ToolName: "search_web", Vendor: "openai", CostUSD: 0.002},
	})
	if len(payload.Data) != 1 {
		t.Fatalf("expected 1 aggregated entry, got %d", len(payload.Data))
	}
	if payload.Data[0].CostUSD != 0.005 {
		t.Errorf("expected summed cost=0.005, got %f", payload.Data[0].CostUSD)
	}
}

func TestWaterfallSort(t *testing.T) {
	payload := waterfall.Build("trace-4", []waterfall.SpanCost{
		{SpanID: "a", ToolName: "tool_a", Vendor: "openai", CostUSD: 0.001},
		{SpanID: "b", ToolName: "tool_b", Vendor: "openai", CostUSD: 0.009},
		{SpanID: "c", ToolName: "tool_c", Vendor: "openai", CostUSD: 0.004},
	})
	want := []float64{0.009, 0.004, 0.001}
	for i, w := range want {
		if payload.Data[i].CostUSD != w {
			t.Errorf("position %d: expected %f, got %f", i, w, payload.Data[i].CostUSD)
		}
	}
}

func TestVendorSplit(t *testing.T) {
	payload := waterfall.Build("trace-5", []waterfall.SpanCost{
		{SpanID: "a", ToolName: "search_web", Vendor: "openai", CostUSD: 0.002},
		{SpanID: "b", ToolName: "search_web", Vendor: "anthropic", CostUSD: 0.003},
	})
	if len(payload.Data) != 2 {
		t.Fatalf("expected 2 separate entries by vendor, got %d", len(payload.Data))
	}
}

func TestZeroCost(t *testing.T) {
	payload := waterfall.Build("trace-6", []waterfall.SpanCost{
		{SpanID: "a", ToolName: "search_web", Vendor: "openai", CostUSD: 0},
	})
	if len(payload.Data) != 1 {
		t.Fatalf("expected zero-cost tool to still be included, got %d entries", len(payload.Data))
	}
	if payload.Data[0].CostUSD != 0 {
		t.Errorf("expected cost=0, got %f", payload.Data[0].CostUSD)
	}
}

func TestWaterfallJSONShape(t *testing.T) {
	payload := waterfall.Build("trace-7", []waterfall.SpanCost{
		{SpanID: "a", ToolName: "search_web", Vendor: "openai", CostUSD: 0.005},
	})
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded["trace_id"] != "trace-7" {
		t.Errorf("expected trace_id=trace-7, got %v", decoded["trace_id"])
	}
	if decoded["data"] == nil {
		t.Errorf("expected data array to be present, got nil")
	}
}

func TestLargeTrace(t *testing.T) {
	spans := make([]waterfall.SpanCost, 0, 100)
	toolCosts := make(map[string]float64)
	for i := 0; i < 100; i++ {
		toolName := []string{"t0", "t1", "t2", "t3", "t4", "t5", "t6", "t7", "t8", "t9"}[i%10]
		cost := 0.001 * float64(i%10+1)
		spans = append(spans, waterfall.SpanCost{
			SpanID:   toolName,
			ToolName: toolName,
			Vendor:   "openai",
			CostUSD:  cost,
		})
		toolCosts[toolName] += cost
	}

	payload := waterfall.Build("trace-8", spans)
	if len(payload.Data) != 10 {
		t.Fatalf("expected 10 distinct tool entries, got %d", len(payload.Data))
	}

	got := make(map[string]float64, len(payload.Data))
	for _, e := range payload.Data {
		got[e.ToolName] = e.CostUSD
	}
	for tool, want := range toolCosts {
		if diff := got[tool] - want; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("tool %s: expected cost %f, got %f", tool, want, got[tool])
		}
	}
}
