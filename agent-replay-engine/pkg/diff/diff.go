// SPDX-License-Identifier: MIT
// Package diff finds the first point where two recorded agent traces
// diverge. It compares tool_call events structurally — by ToolName and
// InputHash, the same fields pkg/mocker uses as its lookup key — rather
// than diffing raw JSON text. See DESIGN.md's Diff Algorithm section.
package diff

import "github.com/akshantvats/agent-replay-engine/pkg/eventlog"

// Reason identifies why two traces diverged at a given step.
type Reason string

const (
	// ReasonToolName means the two traces called a different tool at the
	// same step index.
	ReasonToolName Reason = "tool_name"

	// ReasonInputHash means the two traces called the same tool at the
	// same step index, but with different input (different InputHash).
	ReasonInputHash Reason = "input_hash"

	// ReasonMissingInA means trace B issued a tool call at this step
	// index that trace A has no corresponding call for — A ended first.
	ReasonMissingInA Reason = "missing_in_a"

	// ReasonMissingInB means trace A issued a tool call at this step
	// index that trace B has no corresponding call for — B ended first.
	ReasonMissingInB Reason = "missing_in_b"
)

// Divergence describes the first step at which two traces disagree.
type Divergence struct {
	// StepIndex is the 1-based position of the diverging tool_call within
	// each trace's own tool_call sequence (both traces are indexed the
	// same way, so step N in A lines up with step N in B).
	StepIndex int
	Reason    Reason

	// SpanIDA and SpanIDB are the diverging step's span_id in each trace.
	// One is empty when Reason is a "missing_in_*" case.
	SpanIDA string
	SpanIDB string

	// ToolNameA and ToolNameB are the diverging step's tool_name in each
	// trace. One is empty when Reason is a "missing_in_*" case.
	ToolNameA string
	ToolNameB string
}

// Result is the outcome of comparing two traces.
type Result struct {
	// Divergence is the zero value (Found == false via StepIndex == 0)
	// when every step both traces share in common matched — the traces
	// agree everywhere they overlap.
	Divergence Divergence

	// StepsCompared is the number of leading steps that matched before
	// Divergence, or the full compared length if the traces never
	// diverge.
	StepsCompared int

	// StepsTotalA and StepsTotalB are each trace's total tool_call count,
	// independent of where the divergence (if any) landed.
	StepsTotalA int
	StepsTotalB int
}

// Found reports whether Compare located a divergence between the traces.
func (r Result) Found() bool {
	return r.Divergence.StepIndex > 0
}

// Compare walks a and b's tool_call events in seq_num order, step by step,
// and returns the first point where they disagree.
//
// Two steps match when both ToolName and InputHash are equal — the same
// composite key pkg/mocker uses to serve frozen responses, so "the traces
// agree" here means "replay would serve the same mocked response at this
// step in both." If one trace has more tool_call events than the other and
// every shared step matched, the shorter trace's end is reported as the
// divergence (ReasonMissingInA / ReasonMissingInB) rather than treated as
// agreement.
func Compare(a, b eventlog.EventLog) Result {
	callsA := a.AllOfKind(eventlog.KindToolCall)
	callsB := b.AllOfKind(eventlog.KindToolCall)

	result := Result{
		StepsTotalA: len(callsA),
		StepsTotalB: len(callsB),
	}

	n := len(callsA)
	if len(callsB) < n {
		n = len(callsB)
	}

	for i := 0; i < n; i++ {
		ca, cb := callsA[i], callsB[i]
		switch {
		case ca.ToolName != cb.ToolName:
			result.Divergence = Divergence{
				StepIndex: i + 1,
				Reason:    ReasonToolName,
				SpanIDA:   ca.SpanID,
				SpanIDB:   cb.SpanID,
				ToolNameA: ca.ToolName,
				ToolNameB: cb.ToolName,
			}
			result.StepsCompared = i
			return result
		case ca.InputHash != cb.InputHash:
			result.Divergence = Divergence{
				StepIndex: i + 1,
				Reason:    ReasonInputHash,
				SpanIDA:   ca.SpanID,
				SpanIDB:   cb.SpanID,
				ToolNameA: ca.ToolName,
				ToolNameB: cb.ToolName,
			}
			result.StepsCompared = i
			return result
		}
	}

	result.StepsCompared = n

	if len(callsA) != len(callsB) {
		if len(callsA) > n {
			// B ran out of steps first — the extra call in A has nothing
			// to compare against in B.
			extra := callsA[n]
			result.Divergence = Divergence{
				StepIndex: n + 1,
				Reason:    ReasonMissingInB,
				SpanIDA:   extra.SpanID,
				ToolNameA: extra.ToolName,
			}
		} else {
			extra := callsB[n]
			result.Divergence = Divergence{
				StepIndex: n + 1,
				Reason:    ReasonMissingInA,
				SpanIDB:   extra.SpanID,
				ToolNameB: extra.ToolName,
			}
		}
	}

	return result
}
