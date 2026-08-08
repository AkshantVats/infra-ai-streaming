// SPDX-License-Identifier: MIT

package rollup

import "testing"

func TestStandardError(t *testing.T) {
	// stddev=0.5, n=100 -> SE = 0.5 / 10 = 0.05
	if got, want := StandardError(0.5, 100), 0.05; abs(got-want) > 1e-9 {
		t.Fatalf("StandardError(0.5, 100) = %v, want %v", got, want)
	}
	if got := StandardError(0.5, 0); got != 0 {
		t.Fatalf("StandardError(0.5, 0) = %v, want 0", got)
	}
	if got := StandardError(0.5, -3); got != 0 {
		t.Fatalf("StandardError(0.5, -3) = %v, want 0", got)
	}
}

func TestLowConfidence(t *testing.T) {
	cases := []struct {
		n    int
		want bool
	}{
		{0, true},
		{29, true},
		{30, false},
		{31, false},
	}
	for _, c := range cases {
		if got := LowConfidence(c.n); got != c.want {
			t.Fatalf("LowConfidence(%d) = %v, want %v", c.n, got, c.want)
		}
	}
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
