// SPDX-License-Identifier: MIT

package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/akshantvats/agent-benchmark-runner/pkg/compare"
)

func fourteenVsNine() (compare.Result, []string, []string) {
	a := make([]string, 14)
	for i := range a {
		a[i] = "tool"
	}
	b := make([]string, 9)
	for i := range b {
		b[i] = "tool"
	}
	return compare.Result{
		TaskID:        "checkout-happy-path",
		AgentA:        "agent-a",
		AgentB:        "agent-b",
		PassedA:       true,
		PassedB:       false,
		SequenceMatch: false,
		Divergence:    &compare.Divergence{StepIndex: 5, ToolA: "charge_payment", ToolB: "retry_charge"},
	}, a, b
}

func TestBuildHeadlineDivergent(t *testing.T) {
	res, a, b := fourteenVsNine()
	r := Build(res, a, b)

	want := "14 calls vs 9, diverged at step 5"
	if r.Headline != want {
		t.Errorf("Headline = %q, want %q", r.Headline, want)
	}
	if r.DivergenceStep != 5 {
		t.Errorf("DivergenceStep = %d, want 5", r.DivergenceStep)
	}
}

func TestBuildHeadlineSequenceMatch(t *testing.T) {
	res := compare.Result{
		TaskID:        "checkout-happy-path",
		AgentA:        "agent-a",
		AgentB:        "agent-b",
		PassedA:       true,
		PassedB:       true,
		SequenceMatch: true,
	}
	r := Build(res, []string{"check_inventory", "charge_payment"}, []string{"check_inventory", "charge_payment"})

	want := "2 calls vs 2, sequences matched"
	if r.Headline != want {
		t.Errorf("Headline = %q, want %q", r.Headline, want)
	}
	if r.DivergenceStep != -1 {
		t.Errorf("DivergenceStep = %d, want -1 when sequences matched", r.DivergenceStep)
	}
}

func TestBuildCarriesPassFailPerAgent(t *testing.T) {
	res, a, b := fourteenVsNine()
	r := Build(res, a, b)

	if !r.PassedA {
		t.Error("PassedA = false, want true")
	}
	if r.PassedB {
		t.Error("PassedB = true, want false")
	}
}

func TestRenderMarkdownContainsHeadlineAndTable(t *testing.T) {
	res, a, b := fourteenVsNine()
	r := Build(res, a, b)

	var buf bytes.Buffer
	if err := RenderMarkdown(&buf, r); err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"# Benchmark Report — checkout-happy-path",
		"14 calls vs 9, diverged at step 5",
		"| agent-a | 14 | PASS |",
		"| agent-b | 9 | FAIL |",
		"diverged at step 5",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestRenderMarkdownSequenceMatchNote(t *testing.T) {
	res := compare.Result{TaskID: "t", AgentA: "a", AgentB: "b", PassedA: true, PassedB: true, SequenceMatch: true}
	r := Build(res, []string{"x"}, []string{"x"})

	var buf bytes.Buffer
	if err := RenderMarkdown(&buf, r); err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if !strings.Contains(buf.String(), "Tool call sequences matched exactly.") {
		t.Errorf("expected sequence-match note, got:\n%s", buf.String())
	}
}

func TestRenderJSONRoundTrips(t *testing.T) {
	res, a, b := fourteenVsNine()
	r := Build(res, a, b)

	var buf bytes.Buffer
	if err := RenderJSON(&buf, r); err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}

	var got Report
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Headline != r.Headline {
		t.Errorf("round-tripped Headline = %q, want %q", got.Headline, r.Headline)
	}
	if got.DivergenceStep != r.DivergenceStep {
		t.Errorf("round-tripped DivergenceStep = %d, want %d", got.DivergenceStep, r.DivergenceStep)
	}
	if len(got.ToolCallsA) != len(r.ToolCallsA) {
		t.Errorf("round-tripped ToolCallsA len = %d, want %d", len(got.ToolCallsA), len(r.ToolCallsA))
	}
}

func TestRenderJSONIsIndented(t *testing.T) {
	res, a, b := fourteenVsNine()
	r := Build(res, a, b)

	var buf bytes.Buffer
	if err := RenderJSON(&buf, r); err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	if !strings.Contains(buf.String(), "\n  \"task_id\"") {
		t.Errorf("expected indented JSON, got:\n%s", buf.String())
	}
}
