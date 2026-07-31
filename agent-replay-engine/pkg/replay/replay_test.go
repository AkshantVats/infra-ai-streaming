// SPDX-License-Identifier: MIT
package replay

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/akshantvats/agent-replay-engine/pkg/eventlog"
	"github.com/akshantvats/agent-replay-engine/pkg/fault"
	"github.com/akshantvats/agent-replay-engine/pkg/mocker"
)

func rawJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// sevenStepLog builds a recorded run with 7 tool calls and a final_output,
// mirroring the "stop before step 7" scenario from the Day 46 blog: a
// three-call log would be too small to exercise a mid-run stop.
func sevenStepLog(t *testing.T) eventlog.EventLog {
	t.Helper()
	var log eventlog.EventLog
	seq := int64(1)

	log = append(log, eventlog.AgentEvent{
		SeqNum: seq, TraceID: "trace-1", Kind: eventlog.KindPrompt,
		Payload: rawJSON(t, map[string]any{"text": "roll out config v9"}),
	})
	seq++

	for i := 1; i <= 7; i++ {
		toolName := "deploy_shard"
		inputHash := "hash-shard-" + string(rune('0'+i))
		log = append(log, eventlog.AgentEvent{
			SeqNum: seq, TraceID: "trace-1", Kind: eventlog.KindToolCall,
			ToolName: toolName, InputHash: inputHash,
			Payload: rawJSON(t, map[string]any{"shard": i}),
		})
		seq++
		log = append(log, eventlog.AgentEvent{
			SeqNum: seq, TraceID: "trace-1", Kind: eventlog.KindToolResponse,
			ToolName: toolName, InputHash: inputHash,
			Payload: rawJSON(t, map[string]any{"shard": i, "status": "ok"}),
		})
		seq++
	}

	log = append(log, eventlog.AgentEvent{
		SeqNum: seq, TraceID: "trace-1", Kind: eventlog.KindFinalOutput,
		Payload: rawJSON(t, map[string]any{"text": "rolled out 7 shards"}),
	})

	return log
}

func TestRunToCompletion(t *testing.T) {
	log := sevenStepLog(t)
	m, err := mocker.LoadFromLog(log)
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}

	result := Run(log, m, 0)
	if result.Err != nil {
		t.Fatalf("Run: unexpected error: %v", result.Err)
	}
	if result.StoppedEarly {
		t.Fatalf("Run: StoppedEarly = true, want false for stopAtStep=0")
	}
	if result.StepsRun != 7 {
		t.Fatalf("StepsRun = %d, want 7", result.StepsRun)
	}
	if result.Output != "rolled out 7 shards" {
		t.Fatalf("Output = %q, want %q", result.Output, "rolled out 7 shards")
	}
	if len(result.CallHistory) != 7 {
		t.Fatalf("len(CallHistory) = %d, want 7", len(result.CallHistory))
	}
}

func TestRunStopsBeforeBlastRadius(t *testing.T) {
	log := sevenStepLog(t)
	m, err := mocker.LoadFromLog(log)
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}

	// Stop before step 7 — the scenario from the Day 46 blog post.
	result := Run(log, m, 6)
	if result.Err != nil {
		t.Fatalf("Run: unexpected error: %v", result.Err)
	}
	if !result.StoppedEarly {
		t.Fatalf("StoppedEarly = false, want true for stopAtStep=6 on a 7-step log")
	}
	if result.StepsRun != 6 {
		t.Fatalf("StepsRun = %d, want 6", result.StepsRun)
	}
	if result.Output != "" {
		t.Fatalf("Output = %q, want empty — replay halted before final_output", result.Output)
	}
	if len(result.CallHistory) != 6 {
		t.Fatalf("len(CallHistory) = %d, want 6", len(result.CallHistory))
	}
}

func TestRunStopAtStepBeyondLogLengthRunsToCompletion(t *testing.T) {
	log := sevenStepLog(t)
	m, err := mocker.LoadFromLog(log)
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}

	result := Run(log, m, 100)
	if result.Err != nil {
		t.Fatalf("Run: unexpected error: %v", result.Err)
	}
	if result.StoppedEarly {
		t.Fatalf("StoppedEarly = true, want false when stopAtStep exceeds log length")
	}
	if result.StepsRun != 7 {
		t.Fatalf("StepsRun = %d, want 7", result.StepsRun)
	}
}

func TestRunUnknownToolCallReturnsError(t *testing.T) {
	log := eventlog.EventLog{
		{SeqNum: 1, TraceID: "trace-1", Kind: eventlog.KindToolCall,
			ToolName: "deploy_shard", InputHash: "hash-1",
			Payload: rawJSON(t, map[string]any{"shard": 1})},
		// No matching tool_response recorded — LoadFromLog will skip this
		// pairing, so Respond must return mocker.ErrUnknownCall.
	}
	m, err := mocker.LoadFromLog(log)
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}

	result := Run(log, m, 0)
	if result.Err == nil {
		t.Fatal("Run: expected error for unrecorded tool call, got nil")
	}
	if !errors.Is(result.Err, mocker.ErrUnknownCall) {
		t.Fatalf("Run: err = %v, want wrapping mocker.ErrUnknownCall", result.Err)
	}
	if result.StepsRun != 0 {
		t.Fatalf("StepsRun = %d, want 0", result.StepsRun)
	}
}

func TestRunMissingFinalOutputReturnsError(t *testing.T) {
	log := eventlog.EventLog{
		{SeqNum: 1, TraceID: "trace-1", Kind: eventlog.KindToolCall,
			ToolName: "deploy_shard", InputHash: "hash-1",
			Payload: rawJSON(t, map[string]any{"shard": 1})},
		{SeqNum: 2, TraceID: "trace-1", Kind: eventlog.KindToolResponse,
			ToolName: "deploy_shard", InputHash: "hash-1",
			Payload: rawJSON(t, map[string]any{"status": "ok"})},
		// No final_output event.
	}
	m, err := mocker.LoadFromLog(log)
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}

	result := Run(log, m, 0)
	if !errors.Is(result.Err, ErrNoFinalOutput) {
		t.Fatalf("Run: err = %v, want ErrNoFinalOutput", result.Err)
	}
	if result.StepsRun != 1 {
		t.Fatalf("StepsRun = %d, want 1", result.StepsRun)
	}
}

// TestRunSurfacesInjectedFault verifies the agent error path a caller
// actually cares about: when a fault is injected at a step, Run stops
// there, wraps the fault's sentinel error, and reports exactly the steps
// that ran before it — the same shape as an unrecorded tool call, so a
// caller's error handling doesn't need a separate case for injected faults.
func TestRunSurfacesInjectedFault(t *testing.T) {
	log := sevenStepLog(t)
	m, err := mocker.LoadFromLog(log)
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}
	m.Inject(4, fault.KindTimeout)

	result := Run(log, m, 0)
	if !errors.Is(result.Err, fault.ErrTimeout) {
		t.Fatalf("Run: err = %v, want wrapping fault.ErrTimeout", result.Err)
	}
	if result.StepsRun != 3 {
		t.Fatalf("StepsRun = %d, want 3 (steps before the injected step 4 failed)", result.StepsRun)
	}
	if len(result.CallHistory) != 3 {
		t.Fatalf("len(CallHistory) = %d, want 3", len(result.CallHistory))
	}
	if result.StoppedEarly {
		t.Fatalf("StoppedEarly = true, want false — an injected fault is a failure, not an intentional stop")
	}
}

// TestRunStopAtStepBeforeInjectedFaultNeverTriggersIt confirms StopAtStep
// and Inject compose the way their descriptions promise: halting before the
// injected step means the fault never fires at all.
func TestRunStopAtStepBeforeInjectedFaultNeverTriggersIt(t *testing.T) {
	log := sevenStepLog(t)
	m, err := mocker.LoadFromLog(log)
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}
	m.Inject(4, fault.KindTimeout)

	result := Run(log, m, 3)
	if result.Err != nil {
		t.Fatalf("Run: unexpected error: %v", result.Err)
	}
	if !result.StoppedEarly {
		t.Fatalf("StoppedEarly = false, want true for stopAtStep=3 before the injected step")
	}
	if result.StepsRun != 3 {
		t.Fatalf("StepsRun = %d, want 3", result.StepsRun)
	}
}

func TestRunEmptyTraceReturnsError(t *testing.T) {
	log := eventlog.EventLog{
		{SeqNum: 1, TraceID: "trace-1", Kind: eventlog.KindPrompt,
			Payload: rawJSON(t, map[string]any{"text": "hello"})},
	}
	m, err := mocker.LoadFromLog(log)
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}

	result := Run(log, m, 0)
	if !errors.Is(result.Err, ErrEmptyTrace) {
		t.Fatalf("Run: err = %v, want ErrEmptyTrace", result.Err)
	}
}
