// SPDX-License-Identifier: MIT

// Package compare runs the same task's success criteria against two
// agents' outcomes and reports where their tool call sequences first
// diverge. See DESIGN.md's "Comparing Two Agents" section.
package compare

import (
	"github.com/akshantvats/agent-benchmark-runner/pkg/criteria"
	"github.com/akshantvats/agent-benchmark-runner/pkg/task"
)

// AgentRun names one agent's outcome for a task, so a Result can report
// which agent a criterion result or divergence belongs to.
type AgentRun struct {
	AgentName string
	Outcome   criteria.RunOutcome
}

// Divergence marks the first step at which the two agents' tool call
// sequences disagree. ToolA or ToolB is empty when one agent's sequence
// ended before the other's.
type Divergence struct {
	StepIndex int
	ToolA     string
	ToolB     string
}

// Result is the outcome of grading and comparing two agents on the same
// Task.
type Result struct {
	TaskID        string
	AgentA        string
	AgentB        string
	ResultsA      []criteria.Result
	ResultsB      []criteria.Result
	PassedA       bool
	PassedB       bool
	SequenceMatch bool
	Divergence    *Divergence
}

// Compare grades runA and runB against t's success criteria and locates
// the first point where their tool call sequences diverge, if any.
//
// A and B are graded independently: it is expected and unremarkable for
// one agent to pass and the other to fail — that is the whole point of
// running a benchmark. Divergence only describes *where* their behavior
// first differs, not which one (if either) was correct.
func Compare(t task.Task, runA, runB AgentRun) Result {
	resultsA := criteria.EvaluateAll(t.SuccessCriteria, runA.Outcome)
	resultsB := criteria.EvaluateAll(t.SuccessCriteria, runB.Outcome)

	div := firstDivergence(runA.Outcome.ToolCallSequence, runB.Outcome.ToolCallSequence)

	return Result{
		TaskID:        t.TaskID,
		AgentA:        runA.AgentName,
		AgentB:        runB.AgentName,
		ResultsA:      resultsA,
		ResultsB:      resultsB,
		PassedA:       criteria.AllPassed(resultsA),
		PassedB:       criteria.AllPassed(resultsB),
		SequenceMatch: div == nil,
		Divergence:    div,
	}
}

// firstDivergence walks a and b step by step and returns the first index
// at which they disagree, or nil if every shared step matched and both
// sequences are the same length. If one sequence is a strict prefix of
// the other, the shorter sequence's end is reported as the divergence —
// "stopped early" is a difference, not agreement.
func firstDivergence(a, b []string) *Divergence {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return &Divergence{StepIndex: i, ToolA: a[i], ToolB: b[i]}
		}
	}
	if len(a) != len(b) {
		d := &Divergence{StepIndex: n}
		if n < len(a) {
			d.ToolA = a[n]
		}
		if n < len(b) {
			d.ToolB = b[n]
		}
		return d
	}
	return nil
}
