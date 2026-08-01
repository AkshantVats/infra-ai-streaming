// SPDX-License-Identifier: MIT

package orchestrator

import (
	"math"
	"sort"
)

// Summary aggregates a batch of RunResults into the distribution a
// benchmark report actually needs: not one pass/fail, but a rate with a
// confidence interval attached, plus the shape of the step-count
// distribution. See DESIGN.md's "Summarizing N Runs" section.
type Summary struct {
	N int
	// Completed is the number of repetitions whose agentFn call did not
	// error. PassRate, CILow/CIHigh, MedianSteps, and P95Steps are all
	// computed only over completed repetitions.
	Completed     int
	Passed        int
	PassRate      float64
	CILow, CIHigh float64
	MedianSteps   float64
	P95Steps      float64
}

// wilsonZ95 is the z-score for a 95% confidence interval.
const wilsonZ95 = 1.959963984540054

// Summarize aggregates results into a Summary. A repetition whose agentFn
// call errored contributes no statistical signal about the agent's
// behavior — it counts toward N but is excluded from PassRate, the
// confidence interval, and the step-count percentiles.
func Summarize(results []RunResult) Summary {
	s := Summary{N: len(results)}

	steps := make([]float64, 0, len(results))
	for _, r := range results {
		if r.Err != nil {
			continue
		}
		s.Completed++
		if r.Passed {
			s.Passed++
		}
		steps = append(steps, float64(r.StepCount))
	}

	if s.Completed == 0 {
		return s
	}

	s.PassRate = float64(s.Passed) / float64(s.Completed)
	s.CILow, s.CIHigh = wilsonInterval(s.Passed, s.Completed)
	s.MedianSteps = percentile(steps, 0.5)
	s.P95Steps = percentile(steps, 0.95)
	return s
}

// wilsonInterval computes the 95% Wilson score interval for k successes out
// of n trials. Wilson was chosen over the naive normal (Wald) approximation
// because Wald produces nonsensical bounds — below 0, above 1, or a
// zero-width interval at k=0 or k=n — at exactly the small-N sample sizes a
// benchmark batch typically has (a 10 or 30-run batch, not a 10,000-run
// A/B test).
func wilsonInterval(k, n int) (low, high float64) {
	if n == 0 {
		return 0, 0
	}
	p := float64(k) / float64(n)
	z := wilsonZ95
	nf := float64(n)

	denom := 1 + z*z/nf
	center := p + z*z/(2*nf)
	margin := z * math.Sqrt(p*(1-p)/nf+z*z/(4*nf*nf))

	low = (center - margin) / denom
	high = (center + margin) / denom
	if low < 0 {
		low = 0
	}
	if high > 1 {
		high = 1
	}
	return low, high
}

// percentile returns the p-th percentile (0 <= p <= 1) of values using
// linear interpolation between the two closest ranks. values is not
// mutated.
func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	if len(sorted) == 1 {
		return sorted[0]
	}

	rank := p * float64(len(sorted)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return sorted[lo]
	}
	frac := rank - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}
