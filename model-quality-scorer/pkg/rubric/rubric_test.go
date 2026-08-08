// SPDX-License-Identifier: MIT

package rubric

import (
	"strings"
	"testing"
)

func validRubric() JudgeRubric {
	return JudgeRubric{
		TaskType: "summarization",
		Version:  1,
		Criteria: []Criterion{
			{Name: "factual_grounding", Weight: 0.6, Description: "Does the summary invent facts not present in the source?"},
			{Name: "conciseness", Weight: 0.4, Description: "Is the summary shorter than the source without losing key points?"},
		},
	}
}

func TestValidate_ok(t *testing.T) {
	if err := validRubric().Validate(); err != nil {
		t.Fatalf("expected valid rubric, got error: %v", err)
	}
}

func TestValidate_weightsDontSumToOne(t *testing.T) {
	r := validRubric()
	r.Criteria[0].Weight = 0.9
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for weights not summing to 1.0")
	}
}

func TestValidate_weightSumToleratesFloatNoise(t *testing.T) {
	r := JudgeRubric{
		TaskType: "extraction",
		Version:  1,
		Criteria: []Criterion{
			{Name: "a", Weight: 0.34, Description: "d"},
			{Name: "b", Weight: 0.33, Description: "d"},
			{Name: "c", Weight: 0.33, Description: "d"},
		},
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("expected float noise tolerated, got: %v", err)
	}
}

func TestValidate_emptyTaskType(t *testing.T) {
	r := validRubric()
	r.TaskType = ""
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for empty task_type")
	}
}

func TestValidate_zeroVersion(t *testing.T) {
	r := validRubric()
	r.Version = 0
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for version < 1")
	}
}

func TestValidate_noCriteria(t *testing.T) {
	r := validRubric()
	r.Criteria = nil
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for no criteria")
	}
}

func TestValidate_duplicateCriterionName(t *testing.T) {
	r := validRubric()
	r.Criteria = append(r.Criteria, Criterion{Name: "factual_grounding", Weight: 0.1, Description: "dup"})
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for duplicate criterion name")
	}
}

func TestValidate_nonPositiveWeight(t *testing.T) {
	r := validRubric()
	r.Criteria[0].Weight = 0
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for non-positive weight")
	}
}

func TestValidate_emptyDescription(t *testing.T) {
	r := validRubric()
	r.Criteria[0].Description = ""
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for empty description")
	}
}

func TestLoad_ok(t *testing.T) {
	body := `{"task_type":"summarization","version":1,"criteria":[
		{"name":"factual_grounding","weight":0.6,"description":"d1"},
		{"name":"conciseness","weight":0.4,"description":"d2"}
	]}`
	r, err := Load(strings.NewReader(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.TaskType != "summarization" || r.Version != 1 || len(r.Criteria) != 2 {
		t.Fatalf("unexpected rubric: %+v", r)
	}
}

func TestLoad_malformedJSON(t *testing.T) {
	if _, err := Load(strings.NewReader(`{not json`)); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestLoad_unknownField(t *testing.T) {
	body := `{"task_type":"x","version":1,"criteria":[{"name":"a","weight":1.0,"description":"d"}],"bogus_field":true}`
	if _, err := Load(strings.NewReader(body)); err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestLoad_invalidRubricPropagatesValidateError(t *testing.T) {
	body := `{"task_type":"","version":1,"criteria":[]}`
	if _, err := Load(strings.NewReader(body)); err == nil {
		t.Fatal("expected validate error to propagate through Load")
	}
}

func TestWeightedScore_ok(t *testing.T) {
	r := validRubric()
	score, err := r.WeightedScore(map[string]float64{
		"factual_grounding": 10,
		"conciseness":       5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := 0.6*100 + 0.4*50
	if score != want {
		t.Fatalf("got %v, want %v", score, want)
	}
}

func TestWeightedScore_missingCriterion(t *testing.T) {
	r := validRubric()
	_, err := r.WeightedScore(map[string]float64{"factual_grounding": 10})
	if err == nil {
		t.Fatal("expected error for missing criterion score")
	}
}

func TestWeightedScore_outOfRange(t *testing.T) {
	r := validRubric()
	_, err := r.WeightedScore(map[string]float64{
		"factual_grounding": 11,
		"conciseness":       5,
	})
	if err == nil {
		t.Fatal("expected error for out-of-range score")
	}
}
