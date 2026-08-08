// SPDX-License-Identifier: MIT

// Package normalize converts rubric.WeightedScore's 0-100 output into a
// 0-1 unit that is comparable across task_types whose rubrics carry
// different criteria counts and weight distributions — the same reason
// a z-scored metric, not a raw one, is what you compare across
// populations measured on different scales (see the Day 79 AI Learning
// post for the full analogy).
package normalize

import "fmt"

// maxRawScore is rubric.WeightedScore's documented output ceiling.
const maxRawScore = 100.0

// Score converts a raw 0-100 rubric.WeightedScore output into a 0-1
// normalized score (raw / 100). An out-of-range raw score can only mean
// WeightedScore's own [0,100] contract broke — this is a caller-bug
// signal, not a data-quality one, so it returns an error rather than
// clamping and silently hiding the violation.
func Score(raw float64) (float64, error) {
	if raw < 0 || raw > maxRawScore {
		return 0, fmt.Errorf("normalize: raw score %v out of range [0,%v]", raw, maxRawScore)
	}
	return raw / maxRawScore, nil
}
