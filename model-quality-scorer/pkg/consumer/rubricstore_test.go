// SPDX-License-Identifier: MIT

package consumer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/akshantvats/model-quality-scorer/pkg/rubric"
)

func TestMapRubricStore_hitAndMiss(t *testing.T) {
	r := rubric.JudgeRubric{TaskType: "summarization", Version: 1, Criteria: []rubric.Criterion{
		{Name: "a", Weight: 1.0, Description: "d"},
	}}
	s := NewMapRubricStore([]rubric.JudgeRubric{r})

	got, err := s.Get("summarization", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TaskType != "summarization" {
		t.Fatalf("unexpected rubric: %+v", got)
	}
	if _, err := s.Get("summarization", 2); err == nil {
		t.Fatal("expected error for unknown version")
	}
	if _, err := s.Get("extraction", 1); err == nil {
		t.Fatal("expected error for unknown task_type")
	}
}

func TestFileRubricStore_loadsAndCaches(t *testing.T) {
	dir := t.TempDir()
	body := `{"task_type":"summarization","version":1,"criteria":[{"name":"a","weight":1.0,"description":"d"}]}`
	if err := os.WriteFile(filepath.Join(dir, "summarization.v1.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	s := NewFileRubricStore(dir)

	got, err := s.Get("summarization", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TaskType != "summarization" || got.Version != 1 {
		t.Fatalf("unexpected rubric: %+v", got)
	}

	// Second Get hits the cache — remove the file and confirm the
	// cached copy still resolves.
	if err := os.Remove(filepath.Join(dir, "summarization.v1.json")); err != nil {
		t.Fatalf("remove fixture: %v", err)
	}
	if _, err := s.Get("summarization", 1); err != nil {
		t.Fatalf("expected cached rubric to still resolve, got: %v", err)
	}
}

func TestFileRubricStore_missingFile(t *testing.T) {
	s := NewFileRubricStore(t.TempDir())
	if _, err := s.Get("summarization", 1); err == nil {
		t.Fatal("expected error for missing rubric file")
	}
}

func TestFileRubricStore_malformedRubricSurfacesValidateError(t *testing.T) {
	dir := t.TempDir()
	// Weights don't sum to 1.0 — fails rubric.Validate.
	body := `{"task_type":"summarization","version":1,"criteria":[{"name":"a","weight":0.5,"description":"d"}]}`
	if err := os.WriteFile(filepath.Join(dir, "summarization.v1.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	s := NewFileRubricStore(dir)
	if _, err := s.Get("summarization", 1); err == nil {
		t.Fatal("expected error for malformed rubric (weights not summing to 1.0)")
	}
}
