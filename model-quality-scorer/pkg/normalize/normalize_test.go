// SPDX-License-Identifier: MIT

package normalize

import "testing"

func TestScore(t *testing.T) {
	cases := []struct {
		name    string
		raw     float64
		want    float64
		wantErr bool
	}{
		{"zero", 0, 0, false},
		{"max", 100, 1, false},
		{"midpoint", 62.5, 0.625, false},
		{"below range", -0.01, 0, true},
		{"above range", 100.01, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Score(c.raw)
			if c.wantErr {
				if err == nil {
					t.Fatalf("Score(%v) = %v, nil; want error", c.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Score(%v) unexpected error: %v", c.raw, err)
			}
			if got != c.want {
				t.Fatalf("Score(%v) = %v, want %v", c.raw, got, c.want)
			}
		})
	}
}
