// SPDX-License-Identifier: MIT

// Package task defines the benchmark task specification: a YAML file
// describing one scenario an agent is run against, plus the success
// criteria used to grade the run. See DESIGN.md's "Task YAML" section.
package task

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// CriterionType identifies how a single success criterion is graded.
type CriterionType string

const (
	// FinalOutputContains passes when the run's final output contains
	// Criterion.Value as a substring.
	FinalOutputContains CriterionType = "final_output_contains"

	// FinalOutputExact passes when the run's final output equals
	// Criterion.Value exactly.
	FinalOutputExact CriterionType = "final_output_exact"

	// ToolCallSequence passes when the run's tool call sequence equals
	// Criterion.Values exactly, in order.
	ToolCallSequence CriterionType = "tool_call_sequence"

	// MaxToolCalls passes when the run issued no more than Criterion.Max
	// tool calls.
	MaxToolCalls CriterionType = "max_tool_calls"
)

// Criterion is one gradeable assertion about a run's outcome. Which
// fields are populated depends on Type — see the CriterionType constants.
type Criterion struct {
	Type   CriterionType `yaml:"type"`
	Value  string        `yaml:"value,omitempty"`
	Values []string      `yaml:"values,omitempty"`
	Max    int           `yaml:"max,omitempty"`
}

// Task is one benchmark scenario: a prompt, a fixed seed, the tools the
// agent is allowed to call, and the criteria a passing run must satisfy.
//
// Two agents run against the same Task (same TaskID, Seed, Prompt) so
// that any difference in outcome is attributable to the agents, not to
// the scenario — see DESIGN.md's "Why the seed is part of the task, not
// the run".
type Task struct {
	TaskID          string      `yaml:"task_id"`
	Seed            int64       `yaml:"seed"`
	Prompt          string      `yaml:"prompt"`
	TimeoutSeconds  int         `yaml:"timeout_seconds"`
	ToolsAllowed    []string    `yaml:"tools_allowed"`
	SuccessCriteria []Criterion `yaml:"success_criteria"`
}

// LoadYAML decodes a Task from r and validates it.
func LoadYAML(r io.Reader) (Task, error) {
	var t Task
	if err := yaml.NewDecoder(r).Decode(&t); err != nil {
		return Task{}, fmt.Errorf("task: decode: %w", err)
	}
	if err := t.Validate(); err != nil {
		return Task{}, err
	}
	return t, nil
}

// LoadFile reads and validates a Task from a YAML file on disk.
func LoadFile(path string) (Task, error) {
	f, err := os.Open(path)
	if err != nil {
		return Task{}, fmt.Errorf("task: open %s: %w", path, err)
	}
	defer f.Close()
	return LoadYAML(f)
}

// Validate checks that the task is well-formed enough to run and grade.
// It does not check that ToolsAllowed matches what any particular agent
// implementation supports — that is the runner's responsibility.
func (t Task) Validate() error {
	if t.TaskID == "" {
		return fmt.Errorf("task: task_id is required")
	}
	if t.Prompt == "" {
		return fmt.Errorf("task %s: prompt is required", t.TaskID)
	}
	if t.TimeoutSeconds <= 0 {
		return fmt.Errorf("task %s: timeout_seconds must be > 0", t.TaskID)
	}
	if len(t.SuccessCriteria) == 0 {
		return fmt.Errorf("task %s: at least one success_criteria entry is required", t.TaskID)
	}
	for i, c := range t.SuccessCriteria {
		if err := c.validate(); err != nil {
			return fmt.Errorf("task %s: criterion %d: %w", t.TaskID, i, err)
		}
	}
	return nil
}

func (c Criterion) validate() error {
	switch c.Type {
	case FinalOutputContains, FinalOutputExact:
		if c.Value == "" {
			return fmt.Errorf("%s requires a non-empty value", c.Type)
		}
	case ToolCallSequence:
		if len(c.Values) == 0 {
			return fmt.Errorf("%s requires a non-empty values list", c.Type)
		}
	case MaxToolCalls:
		if c.Max <= 0 {
			return fmt.Errorf("%s requires max > 0", c.Type)
		}
	case "":
		return fmt.Errorf("type is required")
	default:
		return fmt.Errorf("unknown criterion type %q", c.Type)
	}
	return nil
}
