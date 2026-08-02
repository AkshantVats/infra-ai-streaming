// SPDX-License-Identifier: MIT

package report

import (
	"fmt"
	"html"
	"io"
	"strings"
)

// RenderLandingHTML writes r as a self-contained static HTML page: the
// headline, the per-agent pass/fail table, and an inlined
// RenderTimelineSVG diagram — the same artifact a reader would otherwise
// have to reconstruct from the markdown report and a separately-generated
// SVG. It exists so a single URL can be screenshotted for a launch post
// or shared in a thread, instead of stitching report.md and a .svg file
// together by hand. See DESIGN.md's "Landing Page — One Screenshot, Not
// Three Files".
//
// lensaiLink is the URL of the LensAI dashboard view for this batch, or
// empty to omit the cross-link entirely — a report built without
// --lensai-url (see cmd/traceforge) has nothing to link to.
func RenderLandingHTML(w io.Writer, r Report, lensaiLink string) error {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n")
	fmt.Fprintf(&b, "<title>Benchmark Report — %s</title>\n", html.EscapeString(r.TaskID))
	b.WriteString(landingCSS)
	b.WriteString("</head>\n<body>\n<main>\n")

	fmt.Fprintf(&b, "<h1>Benchmark Report — %s</h1>\n", html.EscapeString(r.TaskID))
	fmt.Fprintf(&b, "<p class=\"headline\">%s</p>\n", html.EscapeString(r.Headline))

	b.WriteString("<table>\n<thead><tr><th>Agent</th><th>Calls</th><th>Result</th></tr></thead>\n<tbody>\n")
	fmt.Fprintf(&b, "<tr><td>%s</td><td>%d</td><td class=\"%s\">%s</td></tr>\n",
		html.EscapeString(r.AgentA), len(r.ToolCallsA), passFailClass(r.PassedA), passFail(r.PassedA))
	fmt.Fprintf(&b, "<tr><td>%s</td><td>%d</td><td class=\"%s\">%s</td></tr>\n",
		html.EscapeString(r.AgentB), len(r.ToolCallsB), passFailClass(r.PassedB), passFail(r.PassedB))
	b.WriteString("</tbody>\n</table>\n")

	b.WriteString("<div class=\"timeline\">\n")
	if err := RenderTimelineSVG(&b, r); err != nil {
		return err
	}
	b.WriteString("</div>\n")

	if lensaiLink != "" {
		fmt.Fprintf(&b, "<p class=\"lensai\"><a href=\"%s\">View this batch in LensAI &rarr;</a></p>\n",
			html.EscapeString(lensaiLink))
	}

	b.WriteString("</main>\n</body>\n</html>\n")
	_, err := io.WriteString(w, b.String())
	return err
}

func passFailClass(passed bool) string {
	if passed {
		return "pass"
	}
	return "fail"
}

// landingCSS matches the SVG palette in timeline.go and flame.go so the
// page and its embedded diagram read as one visual system.
const landingCSS = `<style>
  body { background: #0a1a2e; color: #f0f4f8; font-family: -apple-system, sans-serif; margin: 0; padding: 32px 16px; }
  main { max-width: 720px; margin: 0 auto; }
  h1 { font-size: 20px; margin: 0 0 8px; }
  .headline { color: #8ba3bd; margin: 0 0 24px; }
  table { border-collapse: collapse; width: 100%; margin-bottom: 24px; }
  th, td { text-align: left; padding: 8px 12px; border-bottom: 1px solid #33475b; }
  td.pass { color: #4ade80; }
  td.fail { color: #e0665a; }
  .timeline { overflow-x: auto; margin-bottom: 24px; }
  .lensai a { color: #4a90d9; }
</style>
`
