// SPDX-License-Identifier: MIT
package eval

import (
	"math"
	"testing"
)

// TestStickyAssignment verifies that the same tenant+user always resolves to the
// same variant across repeated calls (deterministic FNV-1a hashing).
func TestStickyAssignment(t *testing.T) {
	variants := []PercentageVariant{
		{Value: "gpt-3.5-turbo", Weight: 50},
		{Value: "gpt-4-turbo-2024-04-09", Weight: 50},
	}

	tenantID := "acme-corp"
	userID := "user-abc-123"
	flagKey := "model-rollout:" + tenantID

	first := EvaluatePercentage(flagKey, tenantID+":"+userID, variants)
	for i := 0; i < 1000; i++ {
		got := EvaluatePercentage(flagKey, tenantID+":"+userID, variants)
		if got != first {
			t.Fatalf("sticky assignment broken: call %d returned %q, want %q", i, got, first)
		}
	}
}

// TestSplitStability verifies that a 50/50 flag distributes traffic within ±2%
// of the 50% target over 10000 distinct user IDs.
func TestSplitStability(t *testing.T) {
	variants := []PercentageVariant{
		{Value: "model-v1", Weight: 50},
		{Value: "model-v2", Weight: 50},
	}
	flagKey := "model-rollout:tenant-load-test"
	const n = 10000
	counts := map[string]int{}
	for i := 0; i < n; i++ {
		userID := generateTestUserID(i)
		v := EvaluatePercentage(flagKey, "tenant-load-test:"+userID, variants)
		counts[v]++
	}
	for variant, count := range counts {
		pct := float64(count) / float64(n) * 100
		if math.Abs(pct-50.0) > 2.0 {
			t.Errorf("variant %q: got %.2f%%, want 50±2%%", variant, pct)
		}
	}
	t.Logf("split stability: model-v1=%.2f%% model-v2=%.2f%%",
		float64(counts["model-v1"])/float64(n)*100,
		float64(counts["model-v2"])/float64(n)*100)
}

// TestNineTenOneVariant checks that a 90/10 split stays within ±2% of targets.
func TestNineTenOneVariant(t *testing.T) {
	variants := []PercentageVariant{
		{Value: "stable-model", Weight: 90},
		{Value: "canary-model", Weight: 10},
	}
	flagKey := "model-rollout:tenant-canary"
	const n = 10000
	counts := map[string]int{}
	for i := 0; i < n; i++ {
		userID := generateTestUserID(i)
		v := EvaluatePercentage(flagKey, "tenant-canary:"+userID, variants)
		counts[v]++
	}
	stablePct := float64(counts["stable-model"]) / float64(n) * 100
	canaryPct := float64(counts["canary-model"]) / float64(n) * 100
	if math.Abs(stablePct-90.0) > 2.0 {
		t.Errorf("stable-model: got %.2f%%, want 90±2%%", stablePct)
	}
	if math.Abs(canaryPct-10.0) > 2.0 {
		t.Errorf("canary-model: got %.2f%%, want 10±2%%", canaryPct)
	}
	t.Logf("90/10 split: stable=%.2f%% canary=%.2f%%", stablePct, canaryPct)
}

// generateTestUserID produces a deterministic user ID string from an integer.
func generateTestUserID(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 16)
	x := n
	for i := range b {
		b[i] = charset[x%len(charset)]
		x = x/len(charset) + i*7
	}
	return "u-" + string(b)
}
