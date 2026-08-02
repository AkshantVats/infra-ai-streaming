// SPDX-License-Identifier: MIT

package report

import (
	"fmt"
	"html"
	"io"
	"strings"
)

// Layout constants for the flame graph SVG. Unlike RenderTimelineSVG's
// fixed-width boxes (timeline.go), a flame graph box's width is data —
// proportional to cost — so only the floor, ceiling, and per-row spacing
// are fixed. See DESIGN.md's "Flame Graph Timeline — Colored by Cost".
const (
	flameMinWidth = 40
	flameMaxWidth = 220
	flameHeight   = 32
	flameGap      = 4
	flameRowGap   = 44
	flameMarginX  = 16
	flameMarginY  = 16
	flameLabelGut = 96
	flameStepH    = 20
)

// Cool-to-hot gradient endpoints for cost coloring, chosen from the same
// palette as timeline.go's matched/divergent colors so the two SVGs read
// as one visual system rather than two unrelated color schemes.
const (
	costCoolFill   = "#0d2137"
	costCoolStroke = "#4a90d9"
	costHotFill    = "#e0665a"
	costHotStroke  = "#ffb199"
)

const peakStrokeWidth = "3"

// CostReport pairs a Report with per-tool-call dollar cost, parallel to
// ToolCallsA and ToolCallsB, so RenderFlameGraphSVG can size and color
// each box by what that call cost instead of only by divergence. It wraps
// Report rather than adding cost fields to it — see DESIGN.md's "Why
// CostReport Wraps Report".
type CostReport struct {
	Report
	CostsA []float64 `json:"costs_a"`
	CostsB []float64 `json:"costs_b"`
}

// BuildCostReport combines r with per-call costs. costsA and costsB must
// have the same length as r.ToolCallsA and r.ToolCallsB respectively — a
// cost is meaningless without knowing which call it belongs to, so a
// length mismatch is rejected rather than silently truncated or padded.
func BuildCostReport(r Report, costsA, costsB []float64) (CostReport, error) {
	if len(costsA) != len(r.ToolCallsA) {
		return CostReport{}, fmt.Errorf("report: costsA has %d entries, want %d (len(ToolCallsA))", len(costsA), len(r.ToolCallsA))
	}
	if len(costsB) != len(r.ToolCallsB) {
		return CostReport{}, fmt.Errorf("report: costsB has %d entries, want %d (len(ToolCallsB))", len(costsB), len(r.ToolCallsB))
	}
	return CostReport{Report: r, CostsA: costsA, CostsB: costsB}, nil
}

// RenderFlameGraphSVG writes cr's two tool call sequences as a flame
// graph: one row per agent, boxes laid out contiguously left to right
// within each row (not aligned by step index — see DESIGN.md), width
// proportional to that call's cost, fill on a cool-to-hot gradient scaled
// to the report's most expensive call, and the single most expensive
// call per row marked with a heavier stroke.
func RenderFlameGraphSVG(w io.Writer, cr CostReport) error {
	maxCost := peakCost(cr.CostsA, cr.CostsB)
	widthA := rowWidth(cr.CostsA, maxCost)
	widthB := rowWidth(cr.CostsB, maxCost)
	rowWidthMax := widthA
	if widthB > rowWidthMax {
		rowWidthMax = widthB
	}

	width := flameMarginX*2 + flameLabelGut + rowWidthMax
	if width < flameMarginX*2+flameLabelGut+flameMinWidth {
		width = flameMarginX*2 + flameLabelGut + flameMinWidth
	}
	rowAY := flameMarginY + flameStepH
	rowBY := rowAY + flameHeight + flameRowGap
	height := rowBY + flameHeight + flameMarginY

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" font-family="monospace">`+"\n", width, height, width, height)
	fmt.Fprintf(&b, `<rect x="0" y="0" width="%d" height="%d" fill="none"/>`+"\n", width, height)
	fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="10" fill="%s">cost, coolest to hottest</text>`+"\n", flameMarginX, flameMarginY+flameStepH-8, mutedText)

	renderFlameRow(&b, cr.AgentA, cr.ToolCallsA, cr.CostsA, rowAY, maxCost)
	renderFlameRow(&b, cr.AgentB, cr.ToolCallsB, cr.CostsB, rowBY, maxCost)

	b.WriteString("</svg>\n")
	_, err := io.WriteString(w, b.String())
	return err
}

func renderFlameRow(b *strings.Builder, agentName string, seq []string, costs []float64, y int, maxCost float64) {
	fmt.Fprintf(b, `<text x="%d" y="%d" font-size="11" fill="%s">%s</text>`+"\n",
		flameMarginX, y+flameHeight/2+4, textColor, truncateLabel(agentName, 14))

	peak := peakIndex(costs)
	x := flameMarginX + flameLabelGut
	for i, call := range seq {
		var cost float64
		if i < len(costs) {
			cost = costs[i]
		}
		w := widthForCost(cost, maxCost)
		fill, stroke := costColor(cost, maxCost)
		strokeW := "1"
		if i == peak {
			strokeW = peakStrokeWidth
		}
		fmt.Fprintf(b, `<rect x="%d" y="%d" width="%d" height="%d" fill="%s" stroke="%s" stroke-width="%s" rx="4"><title>%s: $%.4f</title></rect>`+"\n",
			x, y, w, flameHeight, fill, stroke, strokeW, html.EscapeString(call), cost)
		fmt.Fprintf(b, `<text x="%d" y="%d" font-size="10" fill="%s" text-anchor="middle">%s</text>`+"\n",
			x+w/2, y+flameHeight/2+4, textColor, html.EscapeString(truncateLabel(call, maxLabelLen)))
		x += w + flameGap
	}
}

// widthForCost maps cost linearly onto [flameMinWidth, flameMaxWidth],
// scaled to maxCost — the most expensive call in the whole report renders
// at flameMaxWidth, a zero-cost call still renders at the floor so it
// stays visible instead of collapsing to a zero-width sliver. See
// DESIGN.md's "Width Is Linear in Cost, Clamped to a Floor".
func widthForCost(cost, maxCost float64) int {
	if maxCost <= 0 || cost <= 0 {
		return flameMinWidth
	}
	ratio := cost / maxCost
	if ratio > 1 {
		ratio = 1
	}
	return flameMinWidth + int(ratio*float64(flameMaxWidth-flameMinWidth))
}

// costColor interpolates fill/stroke between the cool and hot endpoints
// by cost/maxCost.
func costColor(cost, maxCost float64) (fill, stroke string) {
	if maxCost <= 0 {
		return costCoolFill, costCoolStroke
	}
	t := cost / maxCost
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return lerpHex(costCoolFill, costHotFill, t), lerpHex(costCoolStroke, costHotStroke, t)
}

// lerpHex linearly interpolates between two "#rrggbb" colors at t in
// [0, 1].
func lerpHex(a, b string, t float64) string {
	ar, ag, ab := hexRGB(a)
	br, bg, bb := hexRGB(b)
	r := lerpByte(ar, br, t)
	g := lerpByte(ag, bg, t)
	bl := lerpByte(ab, bb, t)
	return fmt.Sprintf("#%02x%02x%02x", r, g, bl)
}

func hexRGB(s string) (r, g, b int) {
	s = strings.TrimPrefix(s, "#")
	fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b)
	return
}

func lerpByte(a, b int, t float64) int {
	v := float64(a) + t*float64(b-a)
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return int(v)
}

// rowWidth is the total pixel width one row's boxes occupy, laid out
// contiguously with flameGap between them.
func rowWidth(costs []float64, maxCost float64) int {
	if len(costs) == 0 {
		return 0
	}
	total := 0
	for _, c := range costs {
		total += widthForCost(c, maxCost)
	}
	total += flameGap * (len(costs) - 1)
	return total
}

// peakCost is the single most expensive call across both rows — the
// scale the whole SVG's widths and colors are normalized against, so a
// box's width and heat are comparable between agent A and agent B, not
// just within one row.
func peakCost(costsA, costsB []float64) float64 {
	max := 0.0
	for _, c := range costsA {
		if c > max {
			max = c
		}
	}
	for _, c := range costsB {
		if c > max {
			max = c
		}
	}
	return max
}

// peakIndex returns the index of the most expensive call in costs, or -1
// if costs is empty. Ties resolve to the first occurrence.
func peakIndex(costs []float64) int {
	idx := -1
	max := 0.0
	for i, c := range costs {
		if idx == -1 || c > max {
			idx = i
			max = c
		}
	}
	return idx
}
