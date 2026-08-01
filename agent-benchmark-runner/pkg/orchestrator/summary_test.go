// SPDX-License-Identifier: MIT

package orchestrator

import (
	"errors"
	"math"
	"testing"
)

func approxEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func TestSummarize_EmptyResults(t *testing.T) {
	s := Summarize(nil)
	if s.N != 0 || s.Completed != 0 || s.Passed != 0 {
		t.Fatalf("Summarize(nil) = %+v, want all zero", s)
	}
	if s.PassRate != 0 || s.CILow != 0 || s.CIHigh != 0 {
		t.Fatalf("Summarize(nil) rate/CI = %+v, want all zero", s)
	}
}

func TestSummarize_ExcludesErroredRepetitions(t *testing.T) {
	results := []RunResult{
		{RepetitionIndex: 0, Passed: true, StepCount: 2},
		{RepetitionIndex: 1, Err: errors.New("timeout")},
		{RepetitionIndex: 2, Passed: false, StepCount: 4},
	}

	s := Summarize(results)
	if s.N != 3 {
		t.Errorf("N = %d, want 3", s.N)
	}
	if s.Completed != 2 {
		t.Errorf("Completed = %d, want 2 (errored repetition excluded)", s.Completed)
	}
	if s.Passed != 1 {
		t.Errorf("Passed = %d, want 1", s.Passed)
	}
}

func TestSummarize_PassRateAndWilsonInterval(t *testing.T) {
	// 8 passes out of 10: p = 0.8. Reference Wilson 95% CI for k=8, n=10 is
	// [0.49016, 0.94332] (computed independently from the closed-form
	// formula, not from this implementation).
	results := make([]RunResult, 10)
	for i := range results {
		results[i] = RunResult{RepetitionIndex: i, Passed: i < 8, StepCount: 1}
	}

	s := Summarize(results)
	if !approxEqual(s.PassRate, 0.8, 1e-9) {
		t.Errorf("PassRate = %v, want 0.8", s.PassRate)
	}
	if !approxEqual(s.CILow, 0.49016, 1e-4) {
		t.Errorf("CILow = %v, want ~0.49016", s.CILow)
	}
	if !approxEqual(s.CIHigh, 0.94332, 1e-4) {
		t.Errorf("CIHigh = %v, want ~0.94332", s.CIHigh)
	}
	if s.CILow > s.PassRate || s.CIHigh < s.PassRate {
		t.Errorf("CI [%v, %v] does not contain PassRate %v", s.CILow, s.CIHigh, s.PassRate)
	}
}

func TestSummarize_WilsonInterval_BoundedZeroToOne(t *testing.T) {
	// All-pass and all-fail are the cases where a naive Wald interval
	// produces a zero-width or out-of-range interval; Wilson must stay
	// within [0, 1] and have positive width for small n.
	allPass := make([]RunResult, 5)
	for i := range allPass {
		allPass[i] = RunResult{RepetitionIndex: i, Passed: true, StepCount: 1}
	}
	s := Summarize(allPass)
	if s.CIHigh > 1 {
		t.Errorf("all-pass CIHigh = %v, want <= 1", s.CIHigh)
	}
	if s.CILow < 0 || s.CILow >= 1 {
		t.Errorf("all-pass CILow = %v, want in [0, 1)", s.CILow)
	}

	allFail := make([]RunResult, 5)
	for i := range allFail {
		allFail[i] = RunResult{RepetitionIndex: i, Passed: false, StepCount: 1}
	}
	s = Summarize(allFail)
	if s.CILow < 0 {
		t.Errorf("all-fail CILow = %v, want >= 0", s.CILow)
	}
	if s.CIHigh > 1 || s.CIHigh <= 0 {
		t.Errorf("all-fail CIHigh = %v, want in (0, 1]", s.CIHigh)
	}
}

func TestSummarize_MedianAndP95Steps_OddN(t *testing.T) {
	// Step counts 1,2,3,4,5 (odd N=5): median is the middle value, 3.
	results := make([]RunResult, 5)
	for i, steps := range []int{5, 1, 4, 2, 3} {
		results[i] = RunResult{RepetitionIndex: i, Passed: true, StepCount: steps}
	}

	s := Summarize(results)
	if !approxEqual(s.MedianSteps, 3, 1e-9) {
		t.Errorf("MedianSteps = %v, want 3", s.MedianSteps)
	}
	// P95 of [1,2,3,4,5] via linear interpolation: rank = 0.95*4 = 3.8,
	// interpolating between sorted[3]=4 and sorted[4]=5 gives 4.8.
	if !approxEqual(s.P95Steps, 4.8, 1e-9) {
		t.Errorf("P95Steps = %v, want 4.8", s.P95Steps)
	}
}

func TestSummarize_MedianSteps_EvenN(t *testing.T) {
	// Step counts 1,2,3,4 (even N=4): median is the average of the two
	// middle values, (2+3)/2 = 2.5.
	results := make([]RunResult, 4)
	for i, steps := range []int{4, 1, 3, 2} {
		results[i] = RunResult{RepetitionIndex: i, Passed: true, StepCount: steps}
	}

	s := Summarize(results)
	if !approxEqual(s.MedianSteps, 2.5, 1e-9) {
		t.Errorf("MedianSteps = %v, want 2.5", s.MedianSteps)
	}
}

func TestPercentile_SingleValue(t *testing.T) {
	if got := percentile([]float64{7}, 0.5); got != 7 {
		t.Errorf("percentile single value = %v, want 7", got)
	}
	if got := percentile(nil, 0.5); got != 0 {
		t.Errorf("percentile empty = %v, want 0", got)
	}
}
