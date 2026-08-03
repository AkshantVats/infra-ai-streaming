// SPDX-License-Identifier: MIT
package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/akshantvats/agent-benchmark-runner/pkg/compare"
	"github.com/akshantvats/agent-benchmark-runner/pkg/criteria"
	"github.com/akshantvats/agent-benchmark-runner/pkg/orchestrator"
	"github.com/akshantvats/agent-benchmark-runner/pkg/report"
	"github.com/akshantvats/agent-benchmark-runner/pkg/task"
)

// TestLaunchRehearsal rehearses the full `traceforge run` path end to end —
// task YAML in, two agents run N times each, a pass-rate summary and a
// three-artifact divergence report out — without shelling out to `go run`
// or even to pkg/subprocess. It drives the exact functions cmd/traceforge's
// run() composes (orchestrator.Run, compare.Compare, report.Build, and
// run()'s own unexported firstOutcome/writeReport helpers) against two
// in-process stub AgentFuncs with fixed, different pass rates, so a
// regression in how those pieces are wired together — not just in any one
// package's own unit tests — shows up here.
func TestLaunchRehearsal(t *testing.T) {
	tsk, err := task.LoadFile("../../testdata/checkout-happy-path.yaml")
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	const repetitions = 10

	// agentAlpha passes its first 8 repetitions and fails its last 2 —
	// a fixed 80% pass rate. "Fail" here means a run that completes (no
	// AgentFunc error) but falls one tool call short of the task's
	// tool_call_sequence criterion, the same realistic failure shape
	// main_test.go's divergingAgentCmd uses.
	agentAlpha := fixedRateAgent(repetitions, func(i int) bool { return i < 8 })
	// agentBeta fails its first 6 repetitions and passes its last 4 — a
	// fixed 40% pass rate, and (deliberately) the opposite ordering from
	// agentAlpha so their first completed repetitions disagree and the
	// divergence report has something real to show.
	agentBeta := fixedRateAgent(repetitions, func(i int) bool { return i >= 6 })

	ctx := context.Background()
	resultsA, err := orchestrator.Run(ctx,
		orchestrator.Config{Task: tsk, AgentName: "rehearsal-alpha", Repetitions: repetitions, MaxParallel: 1},
		agentAlpha)
	if err != nil {
		t.Fatalf("orchestrator.Run(alpha): %v", err)
	}
	resultsB, err := orchestrator.Run(ctx,
		orchestrator.Config{Task: tsk, AgentName: "rehearsal-beta", Repetitions: repetitions, MaxParallel: 1},
		agentBeta)
	if err != nil {
		t.Fatalf("orchestrator.Run(beta): %v", err)
	}

	summaryA := orchestrator.Summarize(resultsA)
	summaryB := orchestrator.Summarize(resultsB)

	if summaryA.Completed != repetitions {
		t.Errorf("alpha Completed = %d, want %d", summaryA.Completed, repetitions)
	}
	if summaryA.Passed != 8 {
		t.Errorf("alpha Passed = %d, want 8", summaryA.Passed)
	}
	if summaryA.PassRate != 0.8 {
		t.Errorf("alpha PassRate = %v, want 0.8", summaryA.PassRate)
	}

	if summaryB.Completed != repetitions {
		t.Errorf("beta Completed = %d, want %d", summaryB.Completed, repetitions)
	}
	if summaryB.Passed != 4 {
		t.Errorf("beta Passed = %d, want 4", summaryB.Passed)
	}
	if summaryB.PassRate != 0.4 {
		t.Errorf("beta PassRate = %v, want 0.4", summaryB.PassRate)
	}

	// Build the divergence report the same way run() does: from the
	// first completed repetition of each agent.
	outcomeA, ok := firstOutcome(resultsA)
	if !ok {
		t.Fatalf("firstOutcome(alpha): no completed repetition")
	}
	outcomeB, ok := firstOutcome(resultsB)
	if !ok {
		t.Fatalf("firstOutcome(beta): no completed repetition")
	}

	cmpResult := compare.Compare(tsk,
		compare.AgentRun{AgentName: "rehearsal-alpha", Outcome: outcomeA},
		compare.AgentRun{AgentName: "rehearsal-beta", Outcome: outcomeB})
	rep := report.Build(cmpResult, outcomeA.ToolCallSequence, outcomeB.ToolCallSequence)

	if rep.SequenceMatch {
		t.Fatalf("rep.SequenceMatch = true, want false (alpha's first rep passes, beta's first rep fails one tool call short)")
	}
	if rep.DivergenceStep != 1 {
		t.Errorf("rep.DivergenceStep = %d, want 1", rep.DivergenceStep)
	}
	if !strings.Contains(rep.Headline, "diverged at step 1") {
		t.Errorf("rep.Headline = %q, want it to mention diverging at step 1", rep.Headline)
	}

	outDir := t.TempDir()
	if err := writeReport(outDir, tsk.TaskID, rep, ""); err != nil {
		t.Fatalf("writeReport: %v", err)
	}

	mdPath := filepath.Join(outDir, tsk.TaskID+"-report.md")
	mdBytes, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", mdPath, err)
	}
	if len(mdBytes) == 0 {
		t.Errorf("markdown report at %s is empty", mdPath)
	}
	for _, want := range []string{"rehearsal-alpha", "rehearsal-beta"} {
		if !strings.Contains(string(mdBytes), want) {
			t.Errorf("markdown report missing %q:\n%s", want, mdBytes)
		}
	}

	jsonPath := filepath.Join(outDir, tsk.TaskID+"-report.json")
	jsonBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", jsonPath, err)
	}
	var decoded report.Report
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("json report is not valid JSON: %v\n%s", err, jsonBytes)
	}
	if decoded.AgentA != "rehearsal-alpha" || decoded.AgentB != "rehearsal-beta" {
		t.Errorf("json report agents = %s/%s, want rehearsal-alpha/rehearsal-beta", decoded.AgentA, decoded.AgentB)
	}

	htmlPath := filepath.Join(outDir, tsk.TaskID+"-landing.html")
	htmlBytes, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", htmlPath, err)
	}
	html := string(htmlBytes)
	for _, want := range []string{"<!DOCTYPE html>", "rehearsal-alpha", "rehearsal-beta", "<svg"} {
		if !strings.Contains(html, want) {
			t.Errorf("landing page missing %q:\n%s", want, html)
		}
	}
}

// fixedRateAgent returns an orchestrator.AgentFunc whose pass/fail outcome
// on its i-th invocation (0-indexed, in call order) is decided by pass —
// giving the caller an exact, deterministic pass rate over a batch instead
// of one this package would have to derive from randomness. Invocation
// order tracks repetition order exactly because callers run it with
// MaxParallel: 1 — see orchestrator.Run, whose capacity-1 semaphore only
// admits the next repetition once the previous one's goroutine has
// released it, making the batch effectively serial despite still going
// through the same concurrent code path a MaxParallel > 1 batch does.
//
// "Pass" and "fail" here are graded outcomes, not AgentFunc errors: a
// failing invocation still returns a nil error and a completed
// criteria.RunOutcome, it just stops one tool call short of the task's
// tool_call_sequence criterion — the same realistic failure shape
// TestRunTwoAgentsWritesDivergenceReport's divergingAgentCmd uses.
func fixedRateAgent(totalCount int, pass func(i int) bool) orchestrator.AgentFunc {
	var calls int64
	return func(_ context.Context, _ task.Task, _ int64) (criteria.RunOutcome, error) {
		i := int(atomic.AddInt64(&calls, 1)) - 1
		if i >= totalCount {
			// Defensive: a caller invoking this beyond totalCount
			// repetitions has a test bug, not a benchmark result to grade.
			return criteria.RunOutcome{}, nil
		}
		if pass(i) {
			return criteria.RunOutcome{
				FinalOutput:      "order confirmed",
				ToolCallSequence: []string{"check_inventory", "charge_payment"},
			}, nil
		}
		return criteria.RunOutcome{
			FinalOutput:      "order confirmed",
			ToolCallSequence: []string{"check_inventory"},
		}, nil
	}
}
