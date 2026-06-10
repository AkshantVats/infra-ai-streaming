// SPDX-License-Identifier: MIT
package eval_test

import (
	"fmt"
	"testing"

	"github.com/akshantvats/distributed-flagd/internal/eval"
)

// BenchmarkEvaluateBool measures pure in-memory boolean flag resolution.
// No I/O. Measures JSON unmarshal + type assertion path only.
func BenchmarkEvaluateBool(b *testing.B) {
	fv := eval.FlagValue{FlagName: "bench-bool", Type: "bool", ValueJSON: "true"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = eval.Evaluate(fv, fmt.Sprintf("req-%d", i))
	}
}

// BenchmarkEvaluatePercentage measures percentage-rollout evaluation.
// Includes FNV-1a hash + modulo + variant lookup.
func BenchmarkEvaluatePercentage(b *testing.B) {
	variants := []eval.PercentageVariant{
		{Value: "gpt-4o", Weight: 20},
		{Value: "gpt-3.5-turbo", Weight: 80},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = eval.EvaluatePercentage("model-flag", fmt.Sprintf("session-%d", i), variants)
	}
}
