// SPDX-License-Identifier: MIT

// Package report renders a compare.Result as a human-readable comparison
// report: markdown for a PR or Slack summary, JSON for machine
// consumption, and an SVG timeline for a side-by-side visual of where two
// agents' tool call sequences diverged. See DESIGN.md's "Generating a
// Report" section.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/akshantvats/agent-benchmark-runner/pkg/compare"
)

// Report reshapes a compare.Result for rendering. compare.Result
// deliberately carries only the comparison, not the tool call sequences
// it was computed from (see compare.go) — a report needs both, so Build
// takes the sequences separately.
type Report struct {
	TaskID        string   `json:"task_id"`
	AgentA        string   `json:"agent_a"`
	AgentB        string   `json:"agent_b"`
	ToolCallsA    []string `json:"tool_calls_a"`
	ToolCallsB    []string `json:"tool_calls_b"`
	PassedA       bool     `json:"passed_a"`
	PassedB       bool     `json:"passed_b"`
	SequenceMatch bool     `json:"sequence_match"`
	// DivergenceStep is -1 when SequenceMatch is true.
	DivergenceStep int    `json:"divergence_step"`
	Headline       string `json:"headline"`
}

// Build combines result with the two agents' raw tool call sequences
// into a Report, including the one-line Headline a reader sees first.
func Build(result compare.Result, toolCallsA, toolCallsB []string) Report {
	r := Report{
		TaskID:         result.TaskID,
		AgentA:         result.AgentA,
		AgentB:         result.AgentB,
		ToolCallsA:     toolCallsA,
		ToolCallsB:     toolCallsB,
		PassedA:        result.PassedA,
		PassedB:        result.PassedB,
		SequenceMatch:  result.SequenceMatch,
		DivergenceStep: -1,
	}
	if result.Divergence != nil {
		r.DivergenceStep = result.Divergence.StepIndex
	}
	r.Headline = headline(r)
	return r
}

// headline is the one-line summary a report leads with. "14 calls vs 9,
// diverged at step 5" tells a reader where to look before they read a
// single pass/fail badge — see DESIGN.md's "Why the Headline Leads With
// Divergence, Not Pass/Fail".
func headline(r Report) string {
	base := fmt.Sprintf("%d calls vs %d", len(r.ToolCallsA), len(r.ToolCallsB))
	if r.SequenceMatch {
		return base + ", sequences matched"
	}
	return fmt.Sprintf("%s, diverged at step %d", base, r.DivergenceStep)
}

// RenderMarkdown writes r as a markdown report: the headline, a per-agent
// pass/fail table, and the divergence point if any.
func RenderMarkdown(w io.Writer, r Report) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# Benchmark Report — %s\n\n", r.TaskID)
	fmt.Fprintf(&b, "**%s**\n\n", r.Headline)
	b.WriteString("| Agent | Calls | Result |\n|---|---|---|\n")
	fmt.Fprintf(&b, "| %s | %d | %s |\n", r.AgentA, len(r.ToolCallsA), passFail(r.PassedA))
	fmt.Fprintf(&b, "| %s | %d | %s |\n\n", r.AgentB, len(r.ToolCallsB), passFail(r.PassedB))
	if r.SequenceMatch {
		b.WriteString("Tool call sequences matched exactly.\n")
	} else {
		fmt.Fprintf(&b, "Tool call sequences diverged at step %d.\n", r.DivergenceStep)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func passFail(passed bool) string {
	if passed {
		return "PASS"
	}
	return "FAIL"
}

// RenderJSON writes r as indented JSON.
func RenderJSON(w io.Writer, r Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
