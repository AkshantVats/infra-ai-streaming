// SPDX-License-Identifier: MIT

// Package rubric loads and validates the JudgeRubric contract DESIGN.md §2
// commits to: one versioned, weighted criteria list per task_type, embedded
// verbatim into the judge prompt rather than retyped as freeform instructions.
package rubric

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
)

// Criterion is one weighted, named axis a rubric grades a response against.
type Criterion struct {
	Name        string  `json:"name"`
	Weight      float64 `json:"weight"`
	Description string  `json:"description"`
}

// JudgeRubric is the versioned, structured grading contract for one
// task_type. Additive: a new criterion or task_type never invalidates a
// score computed under an older rubric version — it stays graded and
// stored as that version's contract (DESIGN.md §2).
type JudgeRubric struct {
	TaskType string      `json:"task_type"`
	Version  int         `json:"version"`
	Criteria []Criterion `json:"criteria"`
}

// weightSumTolerance absorbs float64 accumulation error in a handwritten
// JSON template — rubric authors write decimals like 0.34+0.33+0.33, not
// values guaranteed to sum to exactly 1.0 in binary floating point.
const weightSumTolerance = 1e-6

// Validate reports why r is not a usable rubric, or nil if it is. A
// malformed rubric is the one input this package refuses to guess about:
// callers route a Validate failure straight to the DLQ (DESIGN.md §5)
// rather than grading against a contract that doesn't hold together.
func (r JudgeRubric) Validate() error {
	if r.TaskType == "" {
		return fmt.Errorf("rubric: task_type is empty")
	}
	if r.Version < 1 {
		return fmt.Errorf("rubric: version must be >= 1, got %d", r.Version)
	}
	if len(r.Criteria) == 0 {
		return fmt.Errorf("rubric: task_type %q version %d has no criteria", r.TaskType, r.Version)
	}
	seen := make(map[string]bool, len(r.Criteria))
	var weightSum float64
	for i, c := range r.Criteria {
		if c.Name == "" {
			return fmt.Errorf("rubric: criterion %d has empty name", i)
		}
		if seen[c.Name] {
			return fmt.Errorf("rubric: duplicate criterion name %q", c.Name)
		}
		seen[c.Name] = true
		if c.Weight <= 0 {
			return fmt.Errorf("rubric: criterion %q has non-positive weight %v", c.Name, c.Weight)
		}
		if c.Description == "" {
			return fmt.Errorf("rubric: criterion %q has empty description", c.Name)
		}
		weightSum += c.Weight
	}
	if math.Abs(weightSum-1.0) > weightSumTolerance {
		return fmt.Errorf("rubric: task_type %q version %d criteria weights sum to %v, want 1.0", r.TaskType, r.Version, weightSum)
	}
	return nil
}

// Load parses and validates a single JudgeRubric from r. It returns the
// same error a malformed-rubric DLQ path checks for — callers do not need
// a second type check to decide whether a load failure means "bad JSON"
// or "well-formed JSON, invalid rubric": both come back as one error.
func Load(r io.Reader) (JudgeRubric, error) {
	var jr JudgeRubric
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&jr); err != nil {
		return JudgeRubric{}, fmt.Errorf("rubric: decode: %w", err)
	}
	if err := jr.Validate(); err != nil {
		return JudgeRubric{}, err
	}
	return jr, nil
}

// WeightedScore combines per-criterion 0-10 judge scores into the 0-100
// normalized score DESIGN.md §2 stores. scores must contain every
// criterion in r.Criteria by name; a missing criterion is a judge
// response bug, not a rubric problem, so it returns an error rather than
// silently normalizing over a partial set.
func (r JudgeRubric) WeightedScore(scores map[string]float64) (float64, error) {
	var total float64
	for _, c := range r.Criteria {
		s, ok := scores[c.Name]
		if !ok {
			return 0, fmt.Errorf("rubric: judge response missing score for criterion %q", c.Name)
		}
		if s < 0 || s > 10 {
			return 0, fmt.Errorf("rubric: criterion %q score %v out of range [0,10]", c.Name, s)
		}
		total += (s / 10.0) * c.Weight
	}
	return total * 100.0, nil
}
