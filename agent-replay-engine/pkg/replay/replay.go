// SPDX-License-Identifier: MIT
// Package replay walks a recorded eventlog.EventLog through a
// mocker.ToolMocker, step by step, and can halt after a caller-chosen
// number of steps instead of always running to the recorded run's
// final_output. See DESIGN.md at the repo root for the full replay
// algorithm this builds on.
//
// A full ModelClient-driven replay (comparing a live model's tool-call
// choices against the recorded sequence) is out of scope here — see the
// Deviation note in DESIGN.md. Run walks the recorded tool_call sequence
// directly against the mocker, which is enough to support the Day 46
// use case: replay a run partially, and stop before a step you don't want
// to re-trigger yet.
package replay

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/akshantvats/agent-replay-engine/pkg/eventlog"
	"github.com/akshantvats/agent-replay-engine/pkg/mocker"
)

// ErrNoFinalOutput is returned when a replay run reaches the end of the
// recorded tool_call sequence but the log has no final_output event to
// report as the completed run's output.
var ErrNoFinalOutput = errors.New("replay: recorded log has no final_output event")

// ErrEmptyTrace is returned by Run when the log has no tool_call events at
// all — there is nothing to replay.
var ErrEmptyTrace = errors.New("replay: no tool_call events in log")

// Result is the outcome of a Run.
type Result struct {
	// Output is the recorded final_output text, populated only when replay
	// ran to completion (StoppedEarly is false and Err is nil).
	Output string

	// CallHistory is mocker.CallHistory() at the point replay stopped —
	// the composite keys of every tool call actually served, in order.
	CallHistory []string

	// StepsRun is the number of recorded tool_call events replay served
	// before stopping (by StopAtStep, by reaching the end, or by error).
	StepsRun int

	// StoppedEarly is true when Run halted because it reached the
	// caller's StopAtStep limit before serving every recorded tool call.
	// This is the intended outcome of a partial replay, not a failure.
	StoppedEarly bool

	// Err is set if replay could not continue: an unrecorded tool call
	// (mocker.ErrUnknownCall) or a log missing final_output.
	Err error
}

// Run replays log against m, serving at most stopAtStep recorded tool
// calls. stopAtStep <= 0 means no limit — replay every recorded tool call
// and report the recorded final_output.
//
// Each step consumes one recorded tool_call event, in seq_num order, by
// asking m to serve its frozen response. This never reaches a live tool
// API — m.Respond either returns the response frozen at record time or
// mocker.ErrUnknownCall.
func Run(log eventlog.EventLog, m *mocker.ToolMocker, stopAtStep int) Result {
	calls := log.AllOfKind(eventlog.KindToolCall)
	if len(calls) == 0 {
		return Result{Err: ErrEmptyTrace}
	}

	steps := 0
	for _, call := range calls {
		if stopAtStep > 0 && steps >= stopAtStep {
			return Result{
				CallHistory:  m.CallHistory(),
				StepsRun:     steps,
				StoppedEarly: true,
			}
		}
		if _, err := m.Respond(call.ToolName, call.InputHash); err != nil {
			return Result{
				CallHistory: m.CallHistory(),
				StepsRun:    steps,
				Err:         fmt.Errorf("replay: step %d: %w", steps+1, err),
			}
		}
		steps++
	}

	final, err := log.First(eventlog.KindFinalOutput)
	if err != nil {
		return Result{
			CallHistory: m.CallHistory(),
			StepsRun:    steps,
			Err:         ErrNoFinalOutput,
		}
	}

	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(final.Payload, &payload); err != nil {
		return Result{
			CallHistory: m.CallHistory(),
			StepsRun:    steps,
			Err:         fmt.Errorf("replay: decode final_output payload: %w", err),
		}
	}

	return Result{
		Output:      payload.Text,
		CallHistory: m.CallHistory(),
		StepsRun:    steps,
	}
}

// RunFromReader replays traceID's tool_call events by streaming r through
// an eventlog.Scanner instead of requiring an already-loaded
// eventlog.EventLog — see Run for the batch equivalent. r is read once,
// forward-only, so peak memory is bounded by one scanner line, not by how
// large the recorded log is; this is the CLI's default replay path (see
// cmd/traceforge) because a laptop debugging a multi-GB production trace
// can't afford Run's read-everything-then-sort-then-filter memory
// profile.
//
// Streaming trusts r's events to already be in seq_num order, the
// guarantee an append-only recorder provides — RunFromReader does not
// re-sort, because buffering enough of the log to sort it would defeat
// the point of streaming.
//
// When stopAtStep halts replay before EOF, RunFromReader returns
// immediately without reading the remainder of r. It does not report how
// many recorded steps remain, unlike the CLI's old fully-buffered path —
// computing that would mean reading to EOF anyway, which is exactly the
// cost early stopping exists to avoid.
//
// The returned error reports I/O or malformed-JSON failures reading r;
// domain-level replay outcomes (an unrecorded tool call, a trace with no
// final_output) are reported on Result.Err, matching Run's convention.
func RunFromReader(r io.Reader, traceID string, m *mocker.ToolMocker, stopAtStep int) (Result, error) {
	sc := eventlog.NewScanner(r)
	steps := 0
	sawToolCall := false
	var finalPayload json.RawMessage

	for sc.Scan() {
		ev := sc.Event()
		if ev.TraceID != traceID {
			continue
		}
		switch ev.Kind {
		case eventlog.KindToolCall:
			sawToolCall = true
			if stopAtStep > 0 && steps >= stopAtStep {
				return Result{
					CallHistory:  m.CallHistory(),
					StepsRun:     steps,
					StoppedEarly: true,
				}, nil
			}
			if _, err := m.Respond(ev.ToolName, ev.InputHash); err != nil {
				return Result{
					CallHistory: m.CallHistory(),
					StepsRun:    steps,
					Err:         fmt.Errorf("replay: step %d: %w", steps+1, err),
				}, nil
			}
			steps++
		case eventlog.KindFinalOutput:
			// First one wins, mirroring eventlog.EventLog.First's
			// lowest-SeqNum contract when the stream is in order.
			if finalPayload == nil {
				finalPayload = ev.Payload
			}
		}
	}
	if err := sc.Err(); err != nil {
		return Result{CallHistory: m.CallHistory(), StepsRun: steps}, fmt.Errorf("replay: %w", err)
	}

	if !sawToolCall {
		return Result{Err: ErrEmptyTrace}, nil
	}
	if finalPayload == nil {
		return Result{
			CallHistory: m.CallHistory(),
			StepsRun:    steps,
			Err:         ErrNoFinalOutput,
		}, nil
	}

	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(finalPayload, &payload); err != nil {
		return Result{
			CallHistory: m.CallHistory(),
			StepsRun:    steps,
			Err:         fmt.Errorf("replay: decode final_output payload: %w", err),
		}, nil
	}

	return Result{
		Output:      payload.Text,
		CallHistory: m.CallHistory(),
		StepsRun:    steps,
	}, nil
}
