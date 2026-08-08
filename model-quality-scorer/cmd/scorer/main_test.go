// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeRubricsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	body := `{"task_type":"summarization","version":1,"criteria":[
		{"name":"factual_grounding","weight":0.6,"description":"grounded"},
		{"name":"conciseness","weight":0.4,"description":"short"}
	]}`
	if err := os.WriteFile(filepath.Join(dir, "summarization.v1.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write rubric fixture: %v", err)
	}
	return dir
}

func TestRun_scoresValidSamples(t *testing.T) {
	rubricsDir := writeRubricsDir(t)
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "samples.jsonl")
	content := `{"tenant_id":"tenant-a","task_type":"summarization","model_id":"gpt-4o-mini","rubric_version":1,"prompt":"long prompt text","response":"a reasonably detailed summary response"}` + "\n"
	if err := os.WriteFile(inputPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", inputPath, "--rubrics-dir", rubricsDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "tenant=tenant-a") {
		t.Errorf("expected a scored row for tenant-a, got:\n%s", stdout.String())
	}
}

func TestRun_malformedRubricGoesToDLQOutput(t *testing.T) {
	rubricsDir := t.TempDir() // no rubric files at all
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "samples.jsonl")
	content := `{"tenant_id":"tenant-a","task_type":"summarization","model_id":"gpt-4o-mini","rubric_version":1,"prompt":"p","response":"r"}` + "\n"
	if err := os.WriteFile(inputPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", inputPath, "--rubrics-dir", rubricsDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "dlq reason=malformed_rubric") {
		t.Errorf("expected a malformed_rubric DLQ line, got:\n%s", stderr.String())
	}
	if stdout.String() != "" {
		t.Errorf("expected no scored rows, got:\n%s", stdout.String())
	}
}

func TestRun_requiresInputFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when --input is missing")
	}
}

func TestRun_skipsBlankLines(t *testing.T) {
	rubricsDir := writeRubricsDir(t)
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "samples.jsonl")
	content := "\n\n" + `{"tenant_id":"tenant-a","task_type":"summarization","model_id":"m","rubric_version":1,"prompt":"p","response":"a detailed enough response"}` + "\n\n"
	if err := os.WriteFile(inputPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", inputPath, "--rubrics-dir", rubricsDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr = %s", code, stderr.String())
	}
	if strings.Count(stdout.String(), "tenant=") != 1 {
		t.Errorf("expected exactly 1 scored row, got:\n%s", stdout.String())
	}
}

func TestRun_emptyResponseGoesToDLQAsJudgeUnavailable(t *testing.T) {
	rubricsDir := writeRubricsDir(t)
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "samples.jsonl")
	content := `{"tenant_id":"tenant-a","task_type":"summarization","model_id":"m","rubric_version":1,"prompt":"p","response":""}` + "\n"
	if err := os.WriteFile(inputPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", inputPath, "--rubrics-dir", rubricsDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "dlq reason=malformed_message") {
		t.Errorf("expected empty response to be caught by SampleMessage validation, got:\n%s", stderr.String())
	}
}
