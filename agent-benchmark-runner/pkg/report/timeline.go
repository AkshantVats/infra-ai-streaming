// SPDX-License-Identifier: MIT

package report

import (
	"fmt"
	"html"
	"io"
	"strings"
)

// Layout constants for the timeline SVG. Kept small and fixed rather than
// configurable — this is a report artifact, not a charting library, and a
// single consistent size is easier to embed in a PR body or email than a
// parameterized one. See DESIGN.md's "Why a Hand-Rolled SVG, Not a
// Charting Dependency".
const (
	boxWidth    = 88
	boxHeight   = 32
	boxGap      = 8
	rowGap      = 40
	marginX     = 16
	marginY     = 16
	labelGutter = 96
	stepLabelH  = 18
	maxLabelLen = 12
)

const (
	fillMatched     = "#0d2137"
	strokeMatched   = "#4a90d9"
	fillDivergent   = "#5f1e26"
	strokeDivergent = "#e0665a"
	fillBeyond      = "#13202f"
	strokeBeyond    = "#33475b"
	textColor       = "#f0f4f8"
	mutedText       = "#8ba3bd"
)

// RenderTimelineSVG writes r's two tool call sequences as a side-by-side
// SVG timeline: one row of boxes per agent, one box per tool call, with
// the divergence step (if any) highlighted so a reader sees where the two
// runs split apart without reading the JSON. A sequence that ended before
// the other's is drawn as a dashed empty slot rather than omitted, so a
// shorter run reads as "stopped here," not as a rendering gap.
func RenderTimelineSVG(w io.Writer, r Report) error {
	n := len(r.ToolCallsA)
	if len(r.ToolCallsB) > n {
		n = len(r.ToolCallsB)
	}
	cols := n
	if cols < 1 {
		cols = 1
	}
	width := marginX*2 + labelGutter + cols*boxWidth + (cols-1)*boxGap
	rowAY := marginY + stepLabelH
	rowBY := rowAY + boxHeight + rowGap
	height := rowBY + boxHeight + marginY

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" font-family="monospace">`+"\n", width, height, width, height)
	fmt.Fprintf(&b, `<rect x="0" y="0" width="%d" height="%d" fill="none"/>`+"\n", width, height)

	for i := 0; i < n; i++ {
		x := marginX + labelGutter + i*(boxWidth+boxGap)
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="10" fill="%s">%d</text>`+"\n", x+boxWidth/2-4, marginY+stepLabelH-6, mutedText, i)
	}

	renderRow(&b, r.AgentA, r.ToolCallsA, rowAY, n, r.DivergenceStep, r.SequenceMatch)
	renderRow(&b, r.AgentB, r.ToolCallsB, rowBY, n, r.DivergenceStep, r.SequenceMatch)

	b.WriteString("</svg>\n")
	_, err := io.WriteString(w, b.String())
	return err
}

func renderRow(b *strings.Builder, agentName string, seq []string, y, n, divergenceStep int, sequenceMatch bool) {
	fmt.Fprintf(b, `<text x="%d" y="%d" font-size="11" fill="%s">%s</text>`+"\n",
		marginX, y+boxHeight/2+4, textColor, truncateLabel(agentName, 14))

	for i := 0; i < n; i++ {
		x := marginX + labelGutter + i*(boxWidth+boxGap)
		if i >= len(seq) {
			fmt.Fprintf(b, `<rect x="%d" y="%d" width="%d" height="%d" fill="none" stroke="%s" stroke-dasharray="3,3" rx="4"/>`+"\n",
				x, y, boxWidth, boxHeight, strokeBeyond)
			continue
		}

		fill, stroke := fillMatched, strokeMatched
		switch {
		case !sequenceMatch && i == divergenceStep:
			fill, stroke = fillDivergent, strokeDivergent
		case !sequenceMatch && i > divergenceStep:
			fill, stroke = fillBeyond, strokeBeyond
		}

		fmt.Fprintf(b, `<rect x="%d" y="%d" width="%d" height="%d" fill="%s" stroke="%s" rx="4"><title>%s</title></rect>`+"\n",
			x, y, boxWidth, boxHeight, fill, stroke, html.EscapeString(seq[i]))
		fmt.Fprintf(b, `<text x="%d" y="%d" font-size="10" fill="%s" text-anchor="middle">%s</text>`+"\n",
			x+boxWidth/2, y+boxHeight/2+4, textColor, html.EscapeString(truncateLabel(seq[i], maxLabelLen)))
	}
}

// truncateLabel keeps SVG text short enough to fit inside a fixed-width
// box — matching the diagram style guide's node-label rule elsewhere in
// this monorepo, adapted to a character count since tool call names are
// single tokens, not multi-word phrases.
func truncateLabel(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}
