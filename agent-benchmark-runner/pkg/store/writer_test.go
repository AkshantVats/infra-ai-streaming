// SPDX-License-Identifier: MIT

package store

import (
	"errors"
	"testing"
	"time"

	"github.com/akshantvats/agent-benchmark-runner/pkg/orchestrator"
)

func TestNewRunRecords_MapsFieldsAndStampsTimestamp(t *testing.T) {
	ts := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	results := []orchestrator.RunResult{
		{RepetitionIndex: 0, Seed: 111, Passed: true, StepCount: 3},
		{RepetitionIndex: 1, Seed: 222, Passed: false, StepCount: 5},
	}

	records := NewRunRecords("checkout-happy-path", "agent-a", results, ts)
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	r0 := records[0]
	if r0.TaskID != "checkout-happy-path" || r0.AgentName != "agent-a" {
		t.Errorf("record[0] task/agent = %q/%q, want checkout-happy-path/agent-a", r0.TaskID, r0.AgentName)
	}
	if r0.RepetitionIndex != 0 || r0.Seed != 111 || !r0.Passed || r0.StepCount != 3 {
		t.Errorf("record[0] = %+v, unexpected field mapping", r0)
	}
	if r0.ErrorMessage != "" {
		t.Errorf("record[0].ErrorMessage = %q, want empty", r0.ErrorMessage)
	}
	if !r0.Timestamp.Equal(ts) {
		t.Errorf("record[0].Timestamp = %v, want %v", r0.Timestamp, ts)
	}

	r1 := records[1]
	if r1.RepetitionIndex != 1 || r1.Seed != 222 || r1.Passed || r1.StepCount != 5 {
		t.Errorf("record[1] = %+v, unexpected field mapping", r1)
	}
}

func TestNewRunRecords_CarriesErrorMessage(t *testing.T) {
	results := []orchestrator.RunResult{
		{RepetitionIndex: 0, Seed: 1, Err: errors.New("timeout waiting for tool response")},
	}

	records := NewRunRecords("checkout-happy-path", "agent-a", results, time.Now())
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].ErrorMessage != "timeout waiting for tool response" {
		t.Errorf("ErrorMessage = %q, want %q", records[0].ErrorMessage, "timeout waiting for tool response")
	}
	if records[0].Passed {
		t.Errorf("Passed = true for an errored repetition, want false")
	}
}

func TestNewRunRecords_EmptyInput(t *testing.T) {
	records := NewRunRecords("checkout-happy-path", "agent-a", nil, time.Now())
	if len(records) != 0 {
		t.Fatalf("expected 0 records, got %d", len(records))
	}
}

func TestBoolToUInt8(t *testing.T) {
	if got := boolToUInt8(true); got != 1 {
		t.Errorf("boolToUInt8(true) = %d, want 1", got)
	}
	if got := boolToUInt8(false); got != 0 {
		t.Errorf("boolToUInt8(false) = %d, want 0", got)
	}
}
