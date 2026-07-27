// SPDX-License-Identifier: MIT
package types_test

import (
	"math"
	"testing"

	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
)

func TestEstimateCost(t *testing.T) {
	cases := []struct {
		model      string
		inputToks  int
		outputToks int
		wantUSD    float64
	}{
		{"gpt-4o", 1_000_000, 0, 2.50},
		{"gpt-4o", 0, 1_000_000, 10.00},
		{"gpt-4o-mini", 1_000_000, 1_000_000, 0.75},
		{"claude-sonnet-4-6", 500_000, 500_000, 9.00},
		{"unknown-model", 1_000_000, 1_000_000, 0.0},
	}
	for _, c := range cases {
		got := types.EstimateCost(c.inputToks, c.outputToks, c.model)
		if math.Abs(got-c.wantUSD) > 0.0001 {
			t.Errorf("EstimateCost(%s, %d, %d) = %.6f, want %.6f",
				c.model, c.inputToks, c.outputToks, got, c.wantUSD)
		}
	}
}

func TestRetryTotalCost(t *testing.T) {
	cases := []struct {
		retries    int
		attemptUSD float64
		wantTotal  float64
	}{
		{0, 0.001, 0.001},
		{1, 0.002, 0.004},
		{3, 0.005, 0.020},
	}
	for _, c := range cases {
		m := types.NewRetryMeta(c.retries, c.attemptUSD, "", "")
		if math.Abs(m.TotalCostUSD-c.wantTotal) > 0.000001 {
			t.Errorf("retries=%d attemptUSD=%.3f: TotalCostUSD=%.6f, want %.6f",
				c.retries, c.attemptUSD, m.TotalCostUSD, c.wantTotal)
		}
	}
}

func TestToolCategoryExhaustive(t *testing.T) {
	if len(types.AllCategories) != 5 {
		t.Errorf("expected 5 ToolCategory constants, got %d", len(types.AllCategories))
	}
	seen := make(map[types.ToolCategory]bool)
	for _, cat := range types.AllCategories {
		if seen[cat] {
			t.Errorf("duplicate category: %s", cat)
		}
		seen[cat] = true
	}
}

func TestToolCallComputedFields(t *testing.T) {
	tc := types.ToolCall{
		Status:   "ERROR",
		HasError: true,
		ErrorMsg: "context deadline exceeded",
	}
	if !tc.HasError {
		t.Error("HasError should be true for ERROR status")
	}
	if tc.ErrorMsg == "" {
		t.Error("ErrorMsg should be populated for error tool calls")
	}
}
