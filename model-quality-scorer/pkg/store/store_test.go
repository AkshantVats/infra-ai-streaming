// SPDX-License-Identifier: MIT

package store

import (
	"testing"
	"time"
)

func TestValidate_ok(t *testing.T) {
	s := ScoredSample{
		TenantID:      "tenant-a",
		TaskType:      "summarization",
		ModelID:       "gpt-4o-mini",
		RubricVersion: 1,
		Score:         87.5,
		Rationale:     "solid grounding, a bit verbose",
		ScoredAt:      time.Now(),
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("expected valid row, got: %v", err)
	}
}

func TestValidate_emptyTenantID(t *testing.T) {
	s := ScoredSample{TaskType: "x", Score: 10, ScoredAt: time.Now()}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for empty tenant_id")
	}
}

func TestValidate_emptyTaskType(t *testing.T) {
	s := ScoredSample{TenantID: "t", Score: 10, ScoredAt: time.Now()}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for empty task_type")
	}
}

func TestValidate_scoreOutOfRange(t *testing.T) {
	s := ScoredSample{TenantID: "t", TaskType: "x", Score: 101, ScoredAt: time.Now()}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for score > 100")
	}
	s.Score = -1
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for score < 0")
	}
}

func TestValidate_zeroScoredAt(t *testing.T) {
	s := ScoredSample{TenantID: "t", TaskType: "x", Score: 10}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for zero scored_at")
	}
}
