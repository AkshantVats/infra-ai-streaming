// SPDX-License-Identifier: MIT

package task

import (
	"strings"
	"testing"
)

const validTaskYAML = `
task_id: checkout-happy-path
seed: 42
prompt: "Complete a checkout for cart ID 8842."
timeout_seconds: 30
tools_allowed:
  - check_inventory
  - charge_payment
success_criteria:
  - type: final_output_contains
    value: "order confirmed"
  - type: tool_call_sequence
    values: ["check_inventory", "charge_payment"]
  - type: max_tool_calls
    max: 5
`

func TestLoadYAMLValid(t *testing.T) {
	tk, err := LoadYAML(strings.NewReader(validTaskYAML))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	if tk.TaskID != "checkout-happy-path" {
		t.Errorf("TaskID = %q, want checkout-happy-path", tk.TaskID)
	}
	if tk.Seed != 42 {
		t.Errorf("Seed = %d, want 42", tk.Seed)
	}
	if len(tk.SuccessCriteria) != 3 {
		t.Fatalf("SuccessCriteria len = %d, want 3", len(tk.SuccessCriteria))
	}
	if len(tk.ToolsAllowed) != 2 {
		t.Errorf("ToolsAllowed len = %d, want 2", len(tk.ToolsAllowed))
	}
}

func TestLoadYAMLMissingTaskID(t *testing.T) {
	yaml := `
prompt: "hi"
timeout_seconds: 10
success_criteria:
  - type: final_output_contains
    value: "hi"
`
	if _, err := LoadYAML(strings.NewReader(yaml)); err == nil {
		t.Fatal("expected error for missing task_id, got nil")
	}
}

func TestLoadYAMLMissingCriteria(t *testing.T) {
	yaml := `
task_id: t1
prompt: "hi"
timeout_seconds: 10
`
	if _, err := LoadYAML(strings.NewReader(yaml)); err == nil {
		t.Fatal("expected error for empty success_criteria, got nil")
	}
}

func TestLoadYAMLZeroTimeout(t *testing.T) {
	yaml := `
task_id: t1
prompt: "hi"
timeout_seconds: 0
success_criteria:
  - type: final_output_contains
    value: "hi"
`
	if _, err := LoadYAML(strings.NewReader(yaml)); err == nil {
		t.Fatal("expected error for zero timeout_seconds, got nil")
	}
}

func TestLoadYAMLUnknownCriterionType(t *testing.T) {
	yaml := `
task_id: t1
prompt: "hi"
timeout_seconds: 10
success_criteria:
  - type: banana
    value: "hi"
`
	if _, err := LoadYAML(strings.NewReader(yaml)); err == nil {
		t.Fatal("expected error for unknown criterion type, got nil")
	}
}

func TestCriterionValidateRequiresFieldsPerType(t *testing.T) {
	cases := []struct {
		name string
		c    Criterion
		ok   bool
	}{
		{"contains with value", Criterion{Type: FinalOutputContains, Value: "x"}, true},
		{"contains without value", Criterion{Type: FinalOutputContains}, false},
		{"exact with value", Criterion{Type: FinalOutputExact, Value: "x"}, true},
		{"sequence with values", Criterion{Type: ToolCallSequence, Values: []string{"a"}}, true},
		{"sequence without values", Criterion{Type: ToolCallSequence}, false},
		{"max with positive", Criterion{Type: MaxToolCalls, Max: 3}, true},
		{"max with zero", Criterion{Type: MaxToolCalls, Max: 0}, false},
		{"empty type", Criterion{}, false},
	}
	for _, tc := range cases {
		err := tc.c.validate()
		if tc.ok && err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}

func TestLoadFileMissing(t *testing.T) {
	if _, err := LoadFile("/nonexistent/path/task.yaml"); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
