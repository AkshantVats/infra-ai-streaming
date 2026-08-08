// SPDX-License-Identifier: MIT

package rollup

import (
	"strings"
	"testing"
)

func TestQuery(t *testing.T) {
	cases := []struct {
		name       string
		w          Window
		wantBucket string
		wantErr    bool
	}{
		{"1h", Window1h, "toStartOfHour(scored_at)", false},
		{"24h", Window24h, "toStartOfDay(scored_at)", false},
		{"unknown", Window("7d"), "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sql, err := Query(c.w)
			if c.wantErr {
				if err == nil {
					t.Fatalf("Query(%q) = %q, nil; want error", c.w, sql)
				}
				return
			}
			if err != nil {
				t.Fatalf("Query(%q) unexpected error: %v", c.w, err)
			}
			for _, frag := range []string{c.wantBucket, "avg(normalized_score)", "model_id", "task_type", "sample_count"} {
				if !strings.Contains(sql, frag) {
					t.Fatalf("Query(%q) = %q, missing fragment %q", c.w, sql, frag)
				}
			}
			if strings.Contains(sql, "avg(score)") {
				t.Fatalf("Query(%q) aggregates raw score, want normalized_score only", c.w)
			}
		})
	}
}

func TestRowHelpers(t *testing.T) {
	r := Row{SampleCount: 10, StddevNormalizedScore: 0.2}
	if got := r.StandardError(); got <= 0 {
		t.Fatalf("StandardError() = %v, want > 0", got)
	}
	if !r.LowConfidence() {
		t.Fatalf("LowConfidence() = false, want true for sample_count=10")
	}
	full := Row{SampleCount: 500, StddevNormalizedScore: 0.2}
	if full.LowConfidence() {
		t.Fatalf("LowConfidence() = true, want false for sample_count=500")
	}
}
