// SPDX-License-Identifier: MIT

package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/akshantvats/agent-benchmark-runner/pkg/compare"
)

func TestRenderTimelineSVGWellFormed(t *testing.T) {
	res, a, b := fourteenVsNine()
	r := Build(res, a, b)

	var buf bytes.Buffer
	if err := RenderTimelineSVG(&buf, r); err != nil {
		t.Fatalf("RenderTimelineSVG: %v", err)
	}
	out := buf.String()

	if !strings.HasPrefix(out, "<svg") {
		t.Errorf("output does not start with <svg>: %s", out[:min(40, len(out))])
	}
	if !strings.HasSuffix(strings.TrimSpace(out), "</svg>") {
		t.Errorf("output does not end with </svg>")
	}
	openRects := strings.Count(out, "<rect")
	if openRects == 0 {
		t.Error("expected at least one <rect> element")
	}
}

func TestRenderTimelineSVGBoxCountMatchesLongerSequence(t *testing.T) {
	res, a, b := fourteenVsNine() // 14 vs 9
	r := Build(res, a, b)

	var buf bytes.Buffer
	if err := RenderTimelineSVG(&buf, r); err != nil {
		t.Fatalf("RenderTimelineSVG: %v", err)
	}
	out := buf.String()

	// One dashed "ended early" placeholder rect per empty slot in the
	// shorter sequence (agent B is 9 calls short of agent A's 14).
	dashed := strings.Count(out, `stroke-dasharray="3,3"`)
	if dashed != 5 {
		t.Errorf("dashed placeholder count = %d, want 5 (14-9)", dashed)
	}
}

func TestRenderTimelineSVGHighlightsDivergence(t *testing.T) {
	res, a, b := fourteenVsNine()
	r := Build(res, a, b)

	var buf bytes.Buffer
	if err := RenderTimelineSVG(&buf, r); err != nil {
		t.Fatalf("RenderTimelineSVG: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, fillDivergent) {
		t.Error("expected divergence fill color to appear in SVG output")
	}
	if !strings.Contains(out, strokeDivergent) {
		t.Error("expected divergence stroke color to appear in SVG output")
	}
}

func TestRenderTimelineSVGNoDivergenceColorWhenSequencesMatch(t *testing.T) {
	res := compare.Result{TaskID: "t", AgentA: "a", AgentB: "b", PassedA: true, PassedB: true, SequenceMatch: true}
	r := Build(res, []string{"x", "y"}, []string{"x", "y"})

	var buf bytes.Buffer
	if err := RenderTimelineSVG(&buf, r); err != nil {
		t.Fatalf("RenderTimelineSVG: %v", err)
	}
	out := buf.String()

	if strings.Contains(out, fillDivergent) {
		t.Error("did not expect divergence fill color when sequences matched")
	}
}

func TestRenderTimelineSVGEmptySequences(t *testing.T) {
	res := compare.Result{TaskID: "t", AgentA: "a", AgentB: "b", SequenceMatch: true}
	r := Build(res, nil, nil)

	var buf bytes.Buffer
	if err := RenderTimelineSVG(&buf, r); err != nil {
		t.Fatalf("RenderTimelineSVG with empty sequences: %v", err)
	}
	if !strings.Contains(buf.String(), "<svg") {
		t.Error("expected valid SVG even with empty sequences")
	}
}

func TestTruncateLabel(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"charge_payment", 12, "charge_paym…"},
		{"short", 12, "short"},
		{"exact12chars", 12, "exact12chars"},
	}
	for _, c := range cases {
		got := truncateLabel(c.in, c.max)
		if got != c.want {
			t.Errorf("truncateLabel(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
		}
	}
}
