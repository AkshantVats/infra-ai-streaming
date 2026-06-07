// SPDX-License-Identifier: MIT
package eval

import (
	"testing"
)

func TestEvaluatePercentage_Deterministic(t *testing.T) {
	variants := []PercentageVariant{
		{Value: "control", Weight: 90},
		{Value: "treatment", Weight: 10},
	}
	v1 := EvaluatePercentage("model-version", "user-123", variants)
	v2 := EvaluatePercentage("model-version", "user-123", variants)
	if v1 != v2 {
		t.Fatalf("non-deterministic: got %q then %q", v1, v2)
	}
}

func TestEvaluatePercentage_Distribution(t *testing.T) {
	variants := []PercentageVariant{
		{Value: "control", Weight: 90},
		{Value: "treatment", Weight: 10},
	}
	counts := map[string]int{}
	for i := 0; i < 10000; i++ {
		key := string(rune('a' + i%26))
		v := EvaluatePercentage("flag", key+string(rune(i)), variants)
		counts[v]++
	}
	total := counts["control"] + counts["treatment"]
	treatmentPct := float64(counts["treatment"]) / float64(total) * 100
	// Allow ±2% deviation from expected 10%
	if treatmentPct < 8 || treatmentPct > 12 {
		t.Fatalf("treatment distribution %.1f%% outside [8,12]%% window", treatmentPct)
	}
}
