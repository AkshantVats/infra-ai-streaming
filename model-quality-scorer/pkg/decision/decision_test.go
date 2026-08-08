// SPDX-License-Identifier: MIT

package decision

import (
	"testing"

	"github.com/akshantvats/model-quality-scorer/pkg/rollup"
)

func TestUtility(t *testing.T) {
	c := Candidate{ModelID: "m", Quality: 0.9, CostPerCall: 0.02, LatencyP99Ms: 400}
	w := RoutingWeights{WQuality: 1.0, WCost: 1.0, WLatency: 0.001}
	got := Utility(c, w)
	want := 0.9 - 0.02 - 0.001*400
	if got != want {
		t.Fatalf("Utility() = %v, want %v", got, want)
	}
}

func TestDecide_PicksHighestUtility(t *testing.T) {
	candidates := []Candidate{
		{ModelID: "cheap-low-quality", Quality: 0.5, CostPerCall: 0.001, LatencyP99Ms: 200},
		{ModelID: "best", Quality: 0.95, CostPerCall: 0.02, LatencyP99Ms: 300},
	}
	w := RoutingWeights{WQuality: 1.0, WCost: 1.0, WLatency: 0.0001}
	d, err := Decide(candidates, w)
	if err != nil {
		t.Fatalf("Decide() unexpected error: %v", err)
	}
	if d.Winner.ModelID != "best" {
		t.Fatalf("Decide() winner = %q, want %q", d.Winner.ModelID, "best")
	}
}

func TestDecide_TieBreaksTowardCheaperModel(t *testing.T) {
	// Both candidates have identical quality and latency, so their raw
	// utility differs only by the cost term — chosen here to fall
	// within tieEpsilon of each other. Decide must pick the cheaper one.
	candidates := []Candidate{
		{ModelID: "pricier", Quality: 0.8, CostPerCall: 0.05, LatencyP99Ms: 250},
		{ModelID: "cheaper", Quality: 0.8, CostPerCall: 0.05 - tieEpsilon/2, LatencyP99Ms: 250},
	}
	w := RoutingWeights{WQuality: 1.0, WCost: 1.0, WLatency: 0}
	d, err := Decide(candidates, w)
	if err != nil {
		t.Fatalf("Decide() unexpected error: %v", err)
	}
	if d.Winner.ModelID != "cheaper" {
		t.Fatalf("Decide() winner = %q, want tie-break toward %q", d.Winner.ModelID, "cheaper")
	}
}

func TestDecide_DeterministicTiebreakOnModelID(t *testing.T) {
	candidates := []Candidate{
		{ModelID: "zebra", Quality: 0.8, CostPerCall: 0.01, LatencyP99Ms: 100},
		{ModelID: "alpha", Quality: 0.8, CostPerCall: 0.01, LatencyP99Ms: 100},
	}
	w := DefaultWeights
	d, err := Decide(candidates, w)
	if err != nil {
		t.Fatalf("Decide() unexpected error: %v", err)
	}
	if d.Winner.ModelID != "alpha" {
		t.Fatalf("Decide() winner = %q, want deterministic tiebreak %q", d.Winner.ModelID, "alpha")
	}
}

func TestDecide_EmptyCandidates(t *testing.T) {
	if _, err := Decide(nil, DefaultWeights); err == nil {
		t.Fatal("Decide(nil, ...) = nil error, want error")
	}
}

func TestDecide_InvalidWeights(t *testing.T) {
	candidates := []Candidate{{ModelID: "m", Quality: 0.5}}
	cases := []RoutingWeights{
		{WQuality: -1, WCost: 1, WLatency: 1},
		{WQuality: 0, WCost: 0, WLatency: 0},
	}
	for _, w := range cases {
		if _, err := Decide(candidates, w); err == nil {
			t.Fatalf("Decide(_, %+v) = nil error, want error", w)
		}
	}
}

func TestDecide_InvalidCandidate(t *testing.T) {
	cases := []Candidate{
		{ModelID: "", Quality: 0.5},
		{ModelID: "m", Quality: 1.5},
		{ModelID: "m", Quality: 0.5, CostPerCall: -1},
		{ModelID: "m", Quality: 0.5, LatencyP99Ms: -1},
	}
	for _, c := range cases {
		if _, err := Decide([]Candidate{c}, DefaultWeights); err == nil {
			t.Fatalf("Decide(%+v, ...) = nil error, want error", c)
		}
	}
}

func TestDecide_SurfacesLowConfidence(t *testing.T) {
	candidates := []Candidate{
		{ModelID: "thin-bucket", Quality: 0.9, LowConfidence: true},
	}
	d, err := Decide(candidates, DefaultWeights)
	if err != nil {
		t.Fatalf("Decide() unexpected error: %v", err)
	}
	if !d.LowConfidence {
		t.Fatal("Decision.LowConfidence = false, want true when winner is low-confidence")
	}
}

func TestWeightsForTenant(t *testing.T) {
	overrides := map[string]RoutingWeights{
		"tenant-a": {WQuality: 2, WCost: 0.5, WLatency: 0.1},
	}
	if got := WeightsForTenant("tenant-a", overrides); got != overrides["tenant-a"] {
		t.Fatalf("WeightsForTenant(tenant-a) = %+v, want override %+v", got, overrides["tenant-a"])
	}
	if got := WeightsForTenant("tenant-b", overrides); got != DefaultWeights {
		t.Fatalf("WeightsForTenant(tenant-b) = %+v, want DefaultWeights %+v", got, DefaultWeights)
	}
}

func TestFromRollupRow(t *testing.T) {
	r := rollup.Row{ModelID: "haiku", AvgNormalizedScore: 0.87, SampleCount: 10}
	c := FromRollupRow(r, 0.002, 350)
	if c.ModelID != "haiku" {
		t.Fatalf("FromRollupRow ModelID = %q, want %q", c.ModelID, "haiku")
	}
	if c.Quality != 0.87 {
		t.Fatalf("FromRollupRow Quality = %v, want %v", c.Quality, 0.87)
	}
	if c.CostPerCall != 0.002 || c.LatencyP99Ms != 350 {
		t.Fatalf("FromRollupRow cost/latency = %v/%v, want 0.002/350", c.CostPerCall, c.LatencyP99Ms)
	}
	if !c.LowConfidence {
		t.Fatalf("FromRollupRow LowConfidence = false, want true (sample_count=10 < %d)", rollup.MinSamplesForConfidence)
	}
}
