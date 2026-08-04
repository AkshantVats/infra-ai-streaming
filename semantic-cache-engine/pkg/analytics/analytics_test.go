// SPDX-License-Identifier: MIT

package analytics

import (
	"strings"
	"testing"
)

func TestHitRate(t *testing.T) {
	cases := []struct {
		name         string
		hits, misses int64
		want         float64
	}{
		{"typical mix", 92, 8, 0.92},
		{"all hits", 10, 0, 1.0},
		{"all misses", 0, 10, 0.0},
		{"no lookups yet", 0, 0, 0.0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := HitRate(c.hits, c.misses)
			if got != c.want {
				t.Errorf("HitRate(%d, %d) = %v, want %v", c.hits, c.misses, got, c.want)
			}
		})
	}
}

func TestFalsePositiveRateProxy(t *testing.T) {
	cases := []struct {
		name             string
		thumbsDown, hits int64
		want             float64
	}{
		{"typical", 1, 1000, 0.001},
		{"zero flags", 0, 500, 0},
		{"no hits yet", 0, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := FalsePositiveRateProxy(c.thumbsDown, c.hits)
			if got != c.want {
				t.Errorf("FalsePositiveRateProxy(%d, %d) = %v, want %v", c.thumbsDown, c.hits, got, c.want)
			}
		})
	}
}

func TestEstimatedCostSaved(t *testing.T) {
	got := EstimatedCostSaved(100, 0.002)
	want := 0.2
	if diff := got - want; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("EstimatedCostSaved(100, 0.002) = %v, want %v", got, want)
	}

	if got := EstimatedCostSaved(0, 0.002); got != 0 {
		t.Errorf("EstimatedCostSaved(0, 0.002) = %v, want 0", got)
	}
}

func TestSummaryString(t *testing.T) {
	s := Summary{Hits: 920, Misses: 80, ThumbsDown: 1, AvgInferenceCostUSD: 0.002}
	got := s.String()
	for _, want := range []string{"hit_rate=0.9200", "false_positive_proxy=0.0011", "estimated_cost_saved_usd=1.8400"} {
		if !strings.Contains(got, want) {
			t.Errorf("Summary.String() = %q, want it to contain %q", got, want)
		}
	}
}

func TestQueryConstantsReferenceExpectedSources(t *testing.T) {
	for name, q := range map[string]string{
		"HitRateQuery":            HitRateQuery,
		"FalsePositiveProxyQuery": FalsePositiveProxyQuery,
		"CostSavedQuery":          CostSavedQuery,
	} {
		if !strings.Contains(q, "infra_ai.inference_events") {
			t.Errorf("%s does not query infra_ai.inference_events", name)
		}
		if !strings.Contains(q, "${tenant_id}") {
			t.Errorf("%s is not scoped by ${tenant_id}", name)
		}
	}
	if !strings.Contains(HitRateQuery, "cache_miss") {
		t.Error("HitRateQuery must include cache_miss as the denominator's other half")
	}
	if !strings.Contains(FalsePositiveProxyQuery, "cache_feedback") {
		t.Error("FalsePositiveProxyQuery must reference cache_feedback")
	}
}
