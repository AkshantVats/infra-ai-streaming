// SPDX-License-Identifier: MIT
package stats_test

import (
	"testing"

	"github.com/AkshantVats/tool-call-analyzer/pkg/stats"
	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
)

func TestP99Single(t *testing.T) {
	if got := stats.P99([]uint64{100}); got != 100 {
		t.Errorf("P99([100]) = %d, want 100", got)
	}
}

func TestP99Empty(t *testing.T) {
	if got := stats.P99(nil); got != 0 {
		t.Errorf("P99(nil) = %d, want 0", got)
	}
}

func TestP99Hundred(t *testing.T) {
	// 100 values 1..100; P99 = value at index floor(99 * 0.99) = index 98 of sorted = 99
	vals := make([]uint64, 100)
	for i := range vals {
		vals[i] = uint64(i + 1)
	}
	got := stats.P99(vals)
	if got != 99 {
		t.Errorf("P99(1..100) = %d, want 99", got)
	}
}

func TestP99DoesNotMutateInput(t *testing.T) {
	input := []uint64{50, 10, 90, 30}
	_ = stats.P99(input)
	if input[0] != 50 {
		t.Error("P99 must not mutate the input slice")
	}
}

var alertTests = []struct {
	name            string
	durationMs      uint64
	traceDurationMs uint64
	wantAlert       bool
}{
	{"exactly 40 pct — not over threshold", 40, 100, false},
	{"41 pct — just over", 41, 100, true},
	{"80 pct — clear alert", 80, 100, true},
	{"zero trace duration — division guard", 100, 0, false},
	{"100 pct — whole trace", 100, 100, true},
	{"1 pct — no alert", 1, 100, false},
	{"39.9 pct — no alert", 399, 1000, false},
	{"40.1 pct — alert", 401, 1000, true},
}

func TestAlertThreshold(t *testing.T) {
	agg := stats.New()
	for _, tt := range alertTests {
		t.Run(tt.name, func(t *testing.T) {
			got := agg.IsAlertThreshold(tt.durationMs, tt.traceDurationMs)
			if got != tt.wantAlert {
				t.Errorf("IsAlertThreshold(%d, %d) = %v, want %v",
					tt.durationMs, tt.traceDurationMs, got, tt.wantAlert)
			}
		})
	}
}

func TestAggregateGroups(t *testing.T) {
	agg := stats.New()

	records := []stats.CallRecord{
		{Tool: types.ToolCall{Name: "search_web", Vendor: "openai", Cost: types.CostEstimate{CostUSD: 0.01}}, DurationMs: 100, HasError: false},
		{Tool: types.ToolCall{Name: "search_web", Vendor: "openai", Cost: types.CostEstimate{CostUSD: 0.01}}, DurationMs: 200, HasError: true},
		{Tool: types.ToolCall{Name: "bash", Vendor: "anthropic", Cost: types.CostEstimate{CostUSD: 0.005}}, DurationMs: 80, HasError: false},
	}

	results := agg.Aggregate(records)

	byKey := make(map[string]stats.ToolStats)
	for _, r := range results {
		byKey[r.ToolName+"/"+r.Vendor] = r
	}

	sw := byKey["search_web/openai"]
	if sw.CallCount != 2 {
		t.Errorf("search_web call_count = %d, want 2", sw.CallCount)
	}
	if sw.ErrorCount != 1 {
		t.Errorf("search_web error_count = %d, want 1", sw.ErrorCount)
	}
	// idx = floor((len-1) * 0.99) = floor(1 * 0.99) = 0 -> the smaller of the two latencies.
	// P99 needs a larger sample to land on the higher value; this only verifies the grouping is correct.
	if sw.P99Ms != 100 {
		t.Errorf("search_web P99 = %d, want 100", sw.P99Ms)
	}
	if sw.CostUSDSum < 0.019 || sw.CostUSDSum > 0.021 {
		t.Errorf("search_web cost_sum = %f, want ~0.02", sw.CostUSDSum)
	}

	bash := byKey["bash/anthropic"]
	if bash.CallCount != 1 {
		t.Errorf("bash call_count = %d, want 1", bash.CallCount)
	}
	if bash.P99Ms != 80 {
		t.Errorf("bash P99 = %d, want 80", bash.P99Ms)
	}
}

func TestAggregateEmpty(t *testing.T) {
	agg := stats.New()
	results := agg.Aggregate(nil)
	if len(results) != 0 {
		t.Errorf("Aggregate(nil) = %d groups, want 0", len(results))
	}
}
