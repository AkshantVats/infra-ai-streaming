// SPDX-License-Identifier: MIT

package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/akshantvats/agent-benchmark-runner/pkg/compare"
)

func costedReport(t *testing.T, costsA, costsB []float64) CostReport {
	t.Helper()
	res := compare.Result{TaskID: "t", AgentA: "agent-a", AgentB: "agent-b", SequenceMatch: true}
	seqA := make([]string, len(costsA))
	for i := range seqA {
		seqA[i] = "tool"
	}
	seqB := make([]string, len(costsB))
	for i := range seqB {
		seqB[i] = "tool"
	}
	r := Build(res, seqA, seqB)
	cr, err := BuildCostReport(r, costsA, costsB)
	if err != nil {
		t.Fatalf("BuildCostReport: %v", err)
	}
	return cr
}

func TestRenderFlameGraphSVGWellFormed(t *testing.T) {
	cr := costedReport(t, []float64{0.01, 0.40, 0.02}, []float64{0.05, 0.03})

	var buf bytes.Buffer
	if err := RenderFlameGraphSVG(&buf, cr); err != nil {
		t.Fatalf("RenderFlameGraphSVG: %v", err)
	}
	out := buf.String()

	if !strings.HasPrefix(out, "<svg") {
		t.Errorf("output does not start with <svg>: %s", out[:min(40, len(out))])
	}
	if !strings.HasSuffix(strings.TrimSpace(out), "</svg>") {
		t.Error("output does not end with </svg>")
	}
	if strings.Count(out, "<rect") < 6 { // outer frame rect + 5 call boxes
		t.Errorf("expected at least 6 <rect> elements, got %d", strings.Count(out, "<rect"))
	}
}

func TestRenderFlameGraphSVGWidthProportionalToCost(t *testing.T) {
	cr := costedReport(t, []float64{0.01, 0.40}, nil)

	var buf bytes.Buffer
	if err := RenderFlameGraphSVG(&buf, cr); err != nil {
		t.Fatalf("RenderFlameGraphSVG: %v", err)
	}

	wCheap := widthForCost(0.01, 0.40)
	wExpensive := widthForCost(0.40, 0.40)
	if wExpensive <= wCheap {
		t.Errorf("widthForCost(0.40) = %d, want > widthForCost(0.01) = %d", wExpensive, wCheap)
	}
}

func TestRenderFlameGraphSVGColorGradientBothEnds(t *testing.T) {
	cr := costedReport(t, []float64{0.0, 1.0}, nil)

	var buf bytes.Buffer
	if err := RenderFlameGraphSVG(&buf, cr); err != nil {
		t.Fatalf("RenderFlameGraphSVG: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, costCoolFill) {
		t.Errorf("expected cool fill %q for the cheapest call", costCoolFill)
	}
	if !strings.Contains(out, costHotFill) {
		t.Errorf("expected hot fill %q for the most expensive call", costHotFill)
	}
}

func TestRenderFlameGraphSVGMarksPeakCost(t *testing.T) {
	cr := costedReport(t, []float64{0.01, 0.02, 0.99}, nil)

	var buf bytes.Buffer
	if err := RenderFlameGraphSVG(&buf, cr); err != nil {
		t.Fatalf("RenderFlameGraphSVG: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, `stroke-width="`+peakStrokeWidth+`"`) {
		t.Error("expected the peak-cost call to carry the heavier peak stroke width")
	}
	// Exactly one box in agent A's row should carry the peak marker.
	if got := strings.Count(out, `stroke-width="`+peakStrokeWidth+`"`); got != 1 {
		t.Errorf("peak stroke-width marker count = %d, want 1", got)
	}
}

func TestRenderFlameGraphSVGEmptyCostsNoDivideByZero(t *testing.T) {
	cr := costedReport(t, nil, nil)

	var buf bytes.Buffer
	if err := RenderFlameGraphSVG(&buf, cr); err != nil {
		t.Fatalf("RenderFlameGraphSVG with empty costs: %v", err)
	}
	if !strings.Contains(buf.String(), "<svg") {
		t.Error("expected valid SVG even with empty cost slices")
	}
}

func TestRenderFlameGraphSVGAllZeroCostsUseFloorWidth(t *testing.T) {
	cr := costedReport(t, []float64{0, 0, 0}, nil)

	var buf bytes.Buffer
	if err := RenderFlameGraphSVG(&buf, cr); err != nil {
		t.Fatalf("RenderFlameGraphSVG: %v", err)
	}
	if got := widthForCost(0, 0); got != flameMinWidth {
		t.Errorf("widthForCost(0, 0) = %d, want floor %d", got, flameMinWidth)
	}
}

func TestBuildCostReportRejectsMismatchedLengths(t *testing.T) {
	res := compare.Result{TaskID: "t", AgentA: "a", AgentB: "b", SequenceMatch: true}
	r := Build(res, []string{"x", "y"}, []string{"x"})

	if _, err := BuildCostReport(r, []float64{0.1}, []float64{0.1}); err == nil {
		t.Error("expected error when costsA length does not match ToolCallsA length")
	}
	if _, err := BuildCostReport(r, []float64{0.1, 0.2}, []float64{}); err == nil {
		t.Error("expected error when costsB length does not match ToolCallsB length")
	}
	if _, err := BuildCostReport(r, []float64{0.1, 0.2}, []float64{0.1}); err != nil {
		t.Errorf("BuildCostReport with matching lengths: unexpected error %v", err)
	}
}

func TestLerpHexEndpoints(t *testing.T) {
	if got := lerpHex(costCoolFill, costHotFill, 0); got != costCoolFill {
		t.Errorf("lerpHex(t=0) = %q, want %q", got, costCoolFill)
	}
	if got := lerpHex(costCoolFill, costHotFill, 1); got != costHotFill {
		t.Errorf("lerpHex(t=1) = %q, want %q", got, costHotFill)
	}
}

func TestPeakIndex(t *testing.T) {
	cases := []struct {
		costs []float64
		want  int
	}{
		{nil, -1},
		{[]float64{}, -1},
		{[]float64{0.5}, 0},
		{[]float64{0.1, 0.9, 0.2}, 1},
		{[]float64{0, 0, 0}, 0},
	}
	for _, c := range cases {
		if got := peakIndex(c.costs); got != c.want {
			t.Errorf("peakIndex(%v) = %d, want %d", c.costs, got, c.want)
		}
	}
}
