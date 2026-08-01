// SPDX-License-Identifier: MIT

package criteria

import (
	"testing"

	"github.com/akshantvats/agent-benchmark-runner/pkg/task"
)

func TestEvaluateFinalOutputContains(t *testing.T) {
	c := task.Criterion{Type: task.FinalOutputContains, Value: "confirmed"}
	pass := Evaluate(c, RunOutcome{FinalOutput: "order confirmed for cart 8842"})
	if !pass.Passed {
		t.Errorf("expected pass, got fail: %s", pass.Detail)
	}
	fail := Evaluate(c, RunOutcome{FinalOutput: "order cancelled"})
	if fail.Passed {
		t.Errorf("expected fail, got pass: %s", fail.Detail)
	}
}

func TestEvaluateFinalOutputExact(t *testing.T) {
	c := task.Criterion{Type: task.FinalOutputExact, Value: "ok"}
	if !Evaluate(c, RunOutcome{FinalOutput: "ok"}).Passed {
		t.Error("expected exact match to pass")
	}
	if Evaluate(c, RunOutcome{FinalOutput: "ok "}).Passed {
		t.Error("expected trailing-space mismatch to fail")
	}
}

func TestEvaluateToolCallSequenceExactMatch(t *testing.T) {
	c := task.Criterion{Type: task.ToolCallSequence, Values: []string{"check_inventory", "charge_payment"}}
	if !Evaluate(c, RunOutcome{ToolCallSequence: []string{"check_inventory", "charge_payment"}}).Passed {
		t.Error("expected identical sequence to pass")
	}
}

func TestEvaluateToolCallSequenceWrongOrder(t *testing.T) {
	c := task.Criterion{Type: task.ToolCallSequence, Values: []string{"check_inventory", "charge_payment"}}
	if Evaluate(c, RunOutcome{ToolCallSequence: []string{"charge_payment", "check_inventory"}}).Passed {
		t.Error("expected reordered sequence to fail")
	}
}

func TestEvaluateToolCallSequenceDifferentLength(t *testing.T) {
	c := task.Criterion{Type: task.ToolCallSequence, Values: []string{"check_inventory", "charge_payment"}}
	if Evaluate(c, RunOutcome{ToolCallSequence: []string{"check_inventory"}}).Passed {
		t.Error("expected shorter sequence to fail")
	}
}

func TestEvaluateMaxToolCalls(t *testing.T) {
	c := task.Criterion{Type: task.MaxToolCalls, Max: 2}
	if !Evaluate(c, RunOutcome{ToolCallSequence: []string{"a", "b"}}).Passed {
		t.Error("expected count == max to pass")
	}
	if Evaluate(c, RunOutcome{ToolCallSequence: []string{"a", "b", "c"}}).Passed {
		t.Error("expected count > max to fail")
	}
}

func TestEvaluateUnknownType(t *testing.T) {
	c := task.Criterion{Type: "not_a_real_type"}
	if Evaluate(c, RunOutcome{}).Passed {
		t.Error("expected unknown criterion type to fail closed, not pass")
	}
}

func TestEvaluateAllAndAllPassed(t *testing.T) {
	criteria := []task.Criterion{
		{Type: task.FinalOutputContains, Value: "ok"},
		{Type: task.MaxToolCalls, Max: 5},
	}
	outcome := RunOutcome{FinalOutput: "ok", ToolCallSequence: []string{"a"}}
	results := EvaluateAll(criteria, outcome)
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if !AllPassed(results) {
		t.Error("expected all criteria to pass")
	}

	failing := EvaluateAll([]task.Criterion{{Type: task.FinalOutputExact, Value: "nope"}}, outcome)
	if AllPassed(failing) {
		t.Error("expected AllPassed to be false when one criterion fails")
	}
}

func TestAllPassedEmptyIsVacuouslyTrue(t *testing.T) {
	if !AllPassed(nil) {
		t.Error("expected AllPassed(nil) to be true")
	}
}
