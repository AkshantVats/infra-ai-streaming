// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBlocksOnceBudgetExceeded(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "requests.jsonl")
	// tenant-a's first request spends 900 of a 1000-token budget; the
	// second (another 900) should cross the hard limit and block.
	content := `{"tenant_id":"tenant-a","model":"gpt-4o","prompt":"` + strings.Repeat("x", 3600) + `"}
{"tenant_id":"tenant-a","model":"gpt-4o","prompt":"` + strings.Repeat("y", 3600) + `"}
`
	if err := os.WriteFile(inputPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", inputPath, "--budget-tokens", "1000"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr = %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "action=inference") {
		t.Errorf("expected at least one inference line, got:\n%s", out)
	}
	if !strings.Contains(out, "action=block") {
		t.Errorf("expected a block line once budget is exceeded, got:\n%s", out)
	}
}

func TestRunRequiresInputFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when --input is missing")
	}
}

func TestRunSkipsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "requests.jsonl")
	content := "not json\n{\"tenant_id\":\"tenant-a\",\"model\":\"gpt-4o\",\"prompt\":\"hello\"}\n"
	if err := os.WriteFile(inputPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", inputPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "action=inference") {
		t.Errorf("expected the valid line to still be processed, got:\n%s", stdout.String())
	}
}
