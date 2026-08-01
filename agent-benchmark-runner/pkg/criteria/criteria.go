// SPDX-License-Identifier: MIT

// Package criteria grades a single agent run's outcome against a task's
// success criteria. It has no notion of "two agents" — that comparison
// lives in pkg/compare, one layer up. See DESIGN.md's "Grading a Single
// Run" section.
package criteria

import (
	"fmt"
	"strings"

	"github.com/akshantvats/agent-benchmark-runner/pkg/task"
)

// RunOutcome is the observable result of one agent run against a task:
// what it said, and what tools it called along the way.
type RunOutcome struct {
	FinalOutput      string
	ToolCallSequence []string
}

// Result is the graded outcome of one Criterion against one RunOutcome.
type Result struct {
	Criterion task.Criterion
	Passed    bool
	Detail    string
}

// Evaluate grades a single criterion against a run's outcome.
func Evaluate(c task.Criterion, outcome RunOutcome) Result {
	switch c.Type {
	case task.FinalOutputContains:
		passed := strings.Contains(outcome.FinalOutput, c.Value)
		return Result{c, passed, fmt.Sprintf("final output %s contain %q", containOrNot(passed), c.Value)}

	case task.FinalOutputExact:
		passed := outcome.FinalOutput == c.Value
		return Result{c, passed, fmt.Sprintf("final output %s equal %q", equalOrNot(passed), c.Value)}

	case task.ToolCallSequence:
		passed := equalSequence(outcome.ToolCallSequence, c.Values)
		return Result{c, passed, fmt.Sprintf("tool call sequence %v vs expected %v", outcome.ToolCallSequence, c.Values)}

	case task.MaxToolCalls:
		n := len(outcome.ToolCallSequence)
		passed := n <= c.Max
		return Result{c, passed, fmt.Sprintf("%d tool calls, max allowed %d", n, c.Max)}

	default:
		return Result{c, false, fmt.Sprintf("unknown criterion type %q", c.Type)}
	}
}

// EvaluateAll grades every criterion against the same run outcome, in
// order.
func EvaluateAll(criteria []task.Criterion, outcome RunOutcome) []Result {
	results := make([]Result, len(criteria))
	for i, c := range criteria {
		results[i] = Evaluate(c, outcome)
	}
	return results
}

// AllPassed reports whether every result passed. An empty results slice
// is considered passing (vacuously true) — callers grading a Task always
// pass a non-empty criteria list, since task.Validate requires at least
// one, so this only matters for callers evaluating an ad hoc subset.
func AllPassed(results []Result) bool {
	for _, r := range results {
		if !r.Passed {
			return false
		}
	}
	return true
}

func equalSequence(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containOrNot(passed bool) string {
	if passed {
		return "does"
	}
	return "does not"
}

func equalOrNot(passed bool) string {
	if passed {
		return "does"
	}
	return "does not"
}
