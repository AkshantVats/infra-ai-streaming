// SPDX-License-Identifier: MIT

package compare

import (
	"testing"

	"github.com/akshantvats/agent-benchmark-runner/pkg/criteria"
	"github.com/akshantvats/agent-benchmark-runner/pkg/task"
)

func testTask() task.Task {
	return task.Task{
		TaskID:         "checkout-happy-path",
		Seed:           42,
		Prompt:         "Complete a checkout for cart ID 8842.",
		TimeoutSeconds: 30,
		SuccessCriteria: []task.Criterion{
			{Type: task.FinalOutputContains, Value: "confirmed"},
			{Type: task.ToolCallSequence, Values: []string{"check_inventory", "charge_payment"}},
		},
	}
}

func TestCompareBothPassIdenticalSequence(t *testing.T) {
	tk := testTask()
	runA := AgentRun{"agent-a", criteria.RunOutcome{FinalOutput: "order confirmed", ToolCallSequence: []string{"check_inventory", "charge_payment"}}}
	runB := AgentRun{"agent-b", criteria.RunOutcome{FinalOutput: "order confirmed", ToolCallSequence: []string{"check_inventory", "charge_payment"}}}

	res := Compare(tk, runA, runB)
	if !res.PassedA || !res.PassedB {
		t.Fatalf("expected both to pass: A=%v B=%v", res.PassedA, res.PassedB)
	}
	if !res.SequenceMatch || res.Divergence != nil {
		t.Errorf("expected no divergence, got %+v", res.Divergence)
	}
}

func TestCompareOnePassesOneFails(t *testing.T) {
	tk := testTask()
	runA := AgentRun{"agent-a", criteria.RunOutcome{FinalOutput: "order confirmed", ToolCallSequence: []string{"check_inventory", "charge_payment"}}}
	runB := AgentRun{"agent-b", criteria.RunOutcome{FinalOutput: "order cancelled", ToolCallSequence: []string{"check_inventory"}}}

	res := Compare(tk, runA, runB)
	if !res.PassedA {
		t.Error("expected agent A to pass")
	}
	if res.PassedB {
		t.Error("expected agent B to fail")
	}
}

func TestCompareDivergenceAtMismatchedStep(t *testing.T) {
	tk := testTask()
	runA := AgentRun{"agent-a", criteria.RunOutcome{ToolCallSequence: []string{"check_inventory", "charge_payment"}}}
	runB := AgentRun{"agent-b", criteria.RunOutcome{ToolCallSequence: []string{"check_inventory", "apply_coupon"}}}

	res := Compare(tk, runA, runB)
	if res.Divergence == nil {
		t.Fatal("expected a divergence")
	}
	if res.Divergence.StepIndex != 1 {
		t.Errorf("StepIndex = %d, want 1", res.Divergence.StepIndex)
	}
	if res.Divergence.ToolA != "charge_payment" || res.Divergence.ToolB != "apply_coupon" {
		t.Errorf("unexpected divergence tools: %+v", res.Divergence)
	}
}

func TestCompareDivergenceWhenOneSequenceShorter(t *testing.T) {
	tk := testTask()
	runA := AgentRun{"agent-a", criteria.RunOutcome{ToolCallSequence: []string{"check_inventory", "charge_payment"}}}
	runB := AgentRun{"agent-b", criteria.RunOutcome{ToolCallSequence: []string{"check_inventory"}}}

	res := Compare(tk, runA, runB)
	if res.Divergence == nil {
		t.Fatal("expected a divergence when one sequence stops early")
	}
	if res.Divergence.StepIndex != 1 {
		t.Errorf("StepIndex = %d, want 1", res.Divergence.StepIndex)
	}
	if res.Divergence.ToolA != "charge_payment" || res.Divergence.ToolB != "" {
		t.Errorf("unexpected divergence tools: %+v", res.Divergence)
	}
}

func TestCompareIdenticalLengthButAllMatchingHasNilDivergence(t *testing.T) {
	tk := testTask()
	seq := []string{"check_inventory", "charge_payment"}
	runA := AgentRun{"agent-a", criteria.RunOutcome{ToolCallSequence: seq}}
	runB := AgentRun{"agent-b", criteria.RunOutcome{ToolCallSequence: seq}}

	res := Compare(tk, runA, runB)
	if res.Divergence != nil {
		t.Errorf("expected nil divergence, got %+v", res.Divergence)
	}
}

func TestCompareResultCarriesTaskAndAgentNames(t *testing.T) {
	tk := testTask()
	runA := AgentRun{"agent-a", criteria.RunOutcome{}}
	runB := AgentRun{"agent-b", criteria.RunOutcome{}}

	res := Compare(tk, runA, runB)
	if res.TaskID != "checkout-happy-path" {
		t.Errorf("TaskID = %q", res.TaskID)
	}
	if res.AgentA != "agent-a" || res.AgentB != "agent-b" {
		t.Errorf("agent names not carried through: A=%q B=%q", res.AgentA, res.AgentB)
	}
}
