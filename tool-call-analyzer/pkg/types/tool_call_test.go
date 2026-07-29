// SPDX-License-Identifier: MIT
package types_test

import (
	"testing"

	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
)

func TestEstimateCost_KnownModel(t *testing.T) {
	cost := types.EstimateCost(1000, 500, "gpt-4o")
	// 1000/1M * $2.50 + 500/1M * $10.00 = $0.0025 + $0.005 = $0.0075
	if cost < 0.007 || cost > 0.008 {
		t.Errorf("unexpected cost for gpt-4o: %f", cost)
	}
}

func TestEstimateCost_UnknownModel(t *testing.T) {
	cost := types.EstimateCost(1000, 500, "unknown-model-xyz")
	if cost != 0.0 {
		t.Errorf("expected 0.0 for unknown model, got %f", cost)
	}
}

func TestNewRetryMeta_TotalCost(t *testing.T) {
	rm := types.NewRetryMeta(2, 0.01, "timeout", "timeout")
	// Total = 0.01 * (2 + 1) = 0.03
	if rm.TotalCostUSD != 0.03 {
		t.Errorf("expected TotalCostUSD 0.03, got %f", rm.TotalCostUSD)
	}
	if rm.RetryCount != 2 {
		t.Errorf("expected RetryCount 2, got %d", rm.RetryCount)
	}
}

func TestAllCategoriesExhaustive(t *testing.T) {
	expected := []types.ToolCategory{
		types.CategoryHTTP,
		types.CategoryDB,
		types.CategoryCode,
		types.CategoryFile,
		types.CategoryAgent,
	}
	if len(types.AllCategories) != len(expected) {
		t.Errorf("AllCategories length %d, expected %d", len(types.AllCategories), len(expected))
	}
}

func TestClassifyByName(t *testing.T) {
	cases := []struct {
		name     string
		expected types.ToolCategory
	}{
		{"sql_query", types.CategoryDB},
		{"vector_search", types.CategoryDB},
		{"redis_get", types.CategoryDB},
		{"run_python", types.CategoryCode},
		{"bash_exec", types.CategoryCode},
		{"read_file", types.CategoryFile},
		{"s3_upload", types.CategoryFile},
		{"agent_delegate", types.CategoryAgent},
		{"search_web", types.CategoryHTTP},
		{"get_weather", types.CategoryHTTP},
		{"fetch_url", types.CategoryHTTP},
	}
	for _, c := range cases {
		got := types.ClassifyByName(c.name)
		if got != c.expected {
			t.Errorf("ClassifyByName(%q) = %q, want %q", c.name, got, c.expected)
		}
	}
}
