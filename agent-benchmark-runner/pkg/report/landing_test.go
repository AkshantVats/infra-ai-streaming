// SPDX-License-Identifier: MIT

package report

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderLandingHTMLContainsHeadlineAndTable(t *testing.T) {
	res, a, b := fourteenVsNine()
	r := Build(res, a, b)

	var buf bytes.Buffer
	if err := RenderLandingHTML(&buf, r, ""); err != nil {
		t.Fatalf("RenderLandingHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"<!DOCTYPE html>",
		"Benchmark Report — checkout-happy-path",
		"14 calls vs 9, diverged at step 5",
		"<td>agent-a</td><td>14</td>",
		"<td>agent-b</td><td>9</td>",
		"<svg",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("landing HTML missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestRenderLandingHTMLOmitsLensaiLinkWhenEmpty(t *testing.T) {
	res, a, b := fourteenVsNine()
	r := Build(res, a, b)

	var buf bytes.Buffer
	if err := RenderLandingHTML(&buf, r, ""); err != nil {
		t.Fatalf("RenderLandingHTML: %v", err)
	}
	if strings.Contains(buf.String(), "View this batch in LensAI") {
		t.Errorf("expected no lensai link block when lensaiLink is empty, got:\n%s", buf.String())
	}
}

func TestRenderLandingHTMLIncludesLensaiLinkWhenSet(t *testing.T) {
	res, a, b := fourteenVsNine()
	r := Build(res, a, b)

	var buf bytes.Buffer
	if err := RenderLandingHTML(&buf, r, "https://lensai.example/batches/abc123"); err != nil {
		t.Fatalf("RenderLandingHTML: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "https://lensai.example/batches/abc123") {
		t.Errorf("expected lensai link in output, got:\n%s", out)
	}
	if !strings.Contains(out, "View this batch in LensAI") {
		t.Errorf("expected lensai link copy in output, got:\n%s", out)
	}
}

func TestRenderLandingHTMLEscapesTaskID(t *testing.T) {
	res, a, b := fourteenVsNine()
	res.TaskID = "<script>alert(1)</script>"
	r := Build(res, a, b)

	var buf bytes.Buffer
	if err := RenderLandingHTML(&buf, r, ""); err != nil {
		t.Fatalf("RenderLandingHTML: %v", err)
	}
	if strings.Contains(buf.String(), "<script>alert(1)</script>") {
		t.Errorf("expected task id to be HTML-escaped, got:\n%s", buf.String())
	}
}
