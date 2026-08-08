// SPDX-License-Identifier: MIT

package rollup

import "math"

// MinSamplesForConfidence is the statistical noise floor (see
// NOISE-FLOOR.md): the standard CLT rule-of-thumb threshold below which
// a sampling distribution of the mean can't be reasonably approximated
// as normal regardless of the underlying distribution's shape, so a
// bucket's standard error is too wide to report the average with the
// same confidence as a full window.
const MinSamplesForConfidence = 30

// StandardError returns the standard error of the mean for a bucket
// with population standard deviation stddev and sampleCount samples:
// stddev / sqrt(n). It returns 0 for a non-positive sample count rather
// than dividing by zero — an empty bucket has no mean to report an
// error bar for in the first place.
func StandardError(stddev float64, sampleCount int) float64 {
	if sampleCount <= 0 {
		return 0
	}
	return stddev / math.Sqrt(float64(sampleCount))
}

// LowConfidence reports whether sampleCount sits below the statistical
// noise floor — a thin bucket whose reported mean should not be trusted
// the same way a full bucket's is.
func LowConfidence(sampleCount int) bool {
	return sampleCount < MinSamplesForConfidence
}
