// SPDX-License-Identifier: MIT
package diff

import (
	"encoding/json"
	"testing"

	"github.com/akshantvats/agent-replay-engine/pkg/eventlog"
)

func rawEmpty(t *testing.T) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(map[string]any{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func call(t *testing.T, seq int64, spanID, tool, inputHash string) eventlog.AgentEvent {
	t.Helper()
	return eventlog.AgentEvent{
		SeqNum:    seq,
		SpanID:    spanID,
		Kind:      eventlog.KindToolCall,
		ToolName:  tool,
		InputHash: inputHash,
		Payload:   rawEmpty(t),
	}
}

func TestCompareIdenticalTracesFindsNoDivergence(t *testing.T) {
	a := eventlog.EventLog{
		call(t, 1, "a1", "route_eta", "sha256:aaa"),
		call(t, 2, "a2", "geocode", "sha256:bbb"),
		call(t, 3, "a3", "route_eta", "sha256:ccc"),
	}
	b := eventlog.EventLog{
		call(t, 1, "b1", "route_eta", "sha256:aaa"),
		call(t, 2, "b2", "geocode", "sha256:bbb"),
		call(t, 3, "b3", "route_eta", "sha256:ccc"),
	}

	got := Compare(a, b)
	if got.Found() {
		t.Fatalf("Found() = true for identical traces, divergence = %+v", got.Divergence)
	}
	if got.StepsCompared != 3 {
		t.Errorf("StepsCompared = %d, want 3", got.StepsCompared)
	}
	if got.StepsTotalA != 3 || got.StepsTotalB != 3 {
		t.Errorf("StepsTotalA/B = %d/%d, want 3/3", got.StepsTotalA, got.StepsTotalB)
	}
}

func TestCompareFindsToolNameDivergence(t *testing.T) {
	a := eventlog.EventLog{
		call(t, 1, "a1", "route_eta", "sha256:aaa"),
		call(t, 2, "a2", "geocode", "sha256:bbb"),
	}
	b := eventlog.EventLog{
		call(t, 1, "b1", "route_eta", "sha256:aaa"),
		call(t, 2, "b2", "traffic_lookup", "sha256:bbb"),
	}

	got := Compare(a, b)
	if !got.Found() {
		t.Fatal("Found() = false, want true")
	}
	d := got.Divergence
	if d.StepIndex != 2 {
		t.Errorf("StepIndex = %d, want 2", d.StepIndex)
	}
	if d.Reason != ReasonToolName {
		t.Errorf("Reason = %q, want %q", d.Reason, ReasonToolName)
	}
	if d.ToolNameA != "geocode" || d.ToolNameB != "traffic_lookup" {
		t.Errorf("ToolNameA/B = %q/%q, want geocode/traffic_lookup", d.ToolNameA, d.ToolNameB)
	}
	if d.SpanIDA != "a2" || d.SpanIDB != "b2" {
		t.Errorf("SpanIDA/B = %q/%q, want a2/b2", d.SpanIDA, d.SpanIDB)
	}
	if got.StepsCompared != 1 {
		t.Errorf("StepsCompared = %d, want 1", got.StepsCompared)
	}
}

func TestCompareFindsInputHashDivergence(t *testing.T) {
	a := eventlog.EventLog{
		call(t, 1, "a1", "route_eta", "sha256:aaa"),
		call(t, 2, "a2", "route_eta", "sha256:bbb-driver-took-highway"),
	}
	b := eventlog.EventLog{
		call(t, 1, "b1", "route_eta", "sha256:aaa"),
		call(t, 2, "b2", "route_eta", "sha256:ccc-driver-took-surface-street"),
	}

	got := Compare(a, b)
	if !got.Found() {
		t.Fatal("Found() = false, want true")
	}
	if got.Divergence.StepIndex != 2 {
		t.Errorf("StepIndex = %d, want 2", got.Divergence.StepIndex)
	}
	if got.Divergence.Reason != ReasonInputHash {
		t.Errorf("Reason = %q, want %q", got.Divergence.Reason, ReasonInputHash)
	}
	if got.Divergence.ToolNameA != "route_eta" || got.Divergence.ToolNameB != "route_eta" {
		t.Errorf("ToolNameA/B = %q/%q, want route_eta/route_eta (same tool, different input)",
			got.Divergence.ToolNameA, got.Divergence.ToolNameB)
	}
}

func TestCompareToolNameCheckedBeforeInputHash(t *testing.T) {
	// Both fields differ at the same step — ToolName is the reported
	// reason since a different tool call is the more fundamental
	// divergence than a different input to the same tool.
	a := eventlog.EventLog{call(t, 1, "a1", "route_eta", "sha256:aaa")}
	b := eventlog.EventLog{call(t, 1, "b1", "geocode", "sha256:bbb")}

	got := Compare(a, b)
	if got.Divergence.Reason != ReasonToolName {
		t.Errorf("Reason = %q, want %q", got.Divergence.Reason, ReasonToolName)
	}
}

func TestCompareMissingInBWhenAIsLonger(t *testing.T) {
	a := eventlog.EventLog{
		call(t, 1, "a1", "route_eta", "sha256:aaa"),
		call(t, 2, "a2", "geocode", "sha256:bbb"),
	}
	b := eventlog.EventLog{
		call(t, 1, "b1", "route_eta", "sha256:aaa"),
	}

	got := Compare(a, b)
	if !got.Found() {
		t.Fatal("Found() = false, want true")
	}
	if got.Divergence.Reason != ReasonMissingInB {
		t.Errorf("Reason = %q, want %q", got.Divergence.Reason, ReasonMissingInB)
	}
	if got.Divergence.StepIndex != 2 {
		t.Errorf("StepIndex = %d, want 2", got.Divergence.StepIndex)
	}
	if got.Divergence.ToolNameA != "geocode" {
		t.Errorf("ToolNameA = %q, want geocode", got.Divergence.ToolNameA)
	}
	if got.Divergence.SpanIDB != "" {
		t.Errorf("SpanIDB = %q, want empty (B has no step 2)", got.Divergence.SpanIDB)
	}
	if got.StepsCompared != 1 {
		t.Errorf("StepsCompared = %d, want 1", got.StepsCompared)
	}
}

func TestCompareMissingInAWhenBIsLonger(t *testing.T) {
	a := eventlog.EventLog{
		call(t, 1, "a1", "route_eta", "sha256:aaa"),
	}
	b := eventlog.EventLog{
		call(t, 1, "b1", "route_eta", "sha256:aaa"),
		call(t, 2, "b2", "geocode", "sha256:bbb"),
	}

	got := Compare(a, b)
	if !got.Found() {
		t.Fatal("Found() = false, want true")
	}
	if got.Divergence.Reason != ReasonMissingInA {
		t.Errorf("Reason = %q, want %q", got.Divergence.Reason, ReasonMissingInA)
	}
	if got.Divergence.StepIndex != 2 {
		t.Errorf("StepIndex = %d, want 2", got.Divergence.StepIndex)
	}
	if got.Divergence.ToolNameB != "geocode" {
		t.Errorf("ToolNameB = %q, want geocode", got.Divergence.ToolNameB)
	}
}

func TestCompareEqualLengthTracesWithNoDivergenceReportsNotFound(t *testing.T) {
	a := eventlog.EventLog{call(t, 1, "a1", "route_eta", "sha256:aaa")}
	b := eventlog.EventLog{call(t, 1, "b1", "route_eta", "sha256:aaa")}

	got := Compare(a, b)
	if got.Found() {
		t.Fatalf("Found() = true, want false; divergence = %+v", got.Divergence)
	}
}

func TestCompareBothEmptyTracesFindNoDivergence(t *testing.T) {
	got := Compare(eventlog.EventLog{}, eventlog.EventLog{})
	if got.Found() {
		t.Fatalf("Found() = true for two empty traces, divergence = %+v", got.Divergence)
	}
	if got.StepsTotalA != 0 || got.StepsTotalB != 0 {
		t.Errorf("StepsTotalA/B = %d/%d, want 0/0", got.StepsTotalA, got.StepsTotalB)
	}
}

func TestCompareOneEmptyOneNonEmptyReportsMissingInA(t *testing.T) {
	a := eventlog.EventLog{}
	b := eventlog.EventLog{call(t, 1, "b1", "route_eta", "sha256:aaa")}

	got := Compare(a, b)
	if !got.Found() {
		t.Fatal("Found() = false, want true")
	}
	if got.Divergence.Reason != ReasonMissingInA {
		t.Errorf("Reason = %q, want %q", got.Divergence.Reason, ReasonMissingInA)
	}
	if got.Divergence.StepIndex != 1 {
		t.Errorf("StepIndex = %d, want 1", got.Divergence.StepIndex)
	}
}

func TestCompareIgnoresNonToolCallEvents(t *testing.T) {
	a := eventlog.EventLog{
		{SeqNum: 1, Kind: eventlog.KindPrompt, Payload: rawEmpty(t)},
		call(t, 2, "a1", "route_eta", "sha256:aaa"),
		{SeqNum: 3, Kind: eventlog.KindToolResponse, ToolName: "route_eta", Payload: rawEmpty(t)},
	}
	b := eventlog.EventLog{
		{SeqNum: 1, Kind: eventlog.KindPrompt, Payload: rawEmpty(t)},
		call(t, 2, "b1", "route_eta", "sha256:aaa"),
	}

	got := Compare(a, b)
	if got.Found() {
		t.Fatalf("Found() = true, want false; non-tool_call events should be ignored; divergence = %+v", got.Divergence)
	}
	if got.StepsTotalA != 1 || got.StepsTotalB != 1 {
		t.Errorf("StepsTotalA/B = %d/%d, want 1/1", got.StepsTotalA, got.StepsTotalB)
	}
}
