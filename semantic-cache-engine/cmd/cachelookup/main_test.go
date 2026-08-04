// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"strings"
	"testing"
)

func TestParseQueriesHappyPath(t *testing.T) {
	input := `{"tenant_id":"t1","prompt":"summarize this"}
{"tenant_id":"t1","prompt":"translate this"}
`
	got, err := parseQueries(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parseQueries: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 queries, got %d", len(got))
	}
	if got[0].TenantID != "t1" || got[0].Prompt != "summarize this" {
		t.Fatalf("unexpected first record: %+v", got[0])
	}
}

func TestParseQueriesSkipsBlankLines(t *testing.T) {
	input := "{\"tenant_id\":\"t1\",\"prompt\":\"a\"}\n\n{\"tenant_id\":\"t1\",\"prompt\":\"b\"}\n"
	got, err := parseQueries(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parseQueries: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 queries, got %d", len(got))
	}
}

func TestParseQueriesRejectsMissingTenant(t *testing.T) {
	if _, err := parseQueries(strings.NewReader(`{"prompt":"a"}`)); err == nil {
		t.Fatal("expected an error for a record missing tenant_id")
	}
}

func TestParseQueriesRejectsMissingPrompt(t *testing.T) {
	if _, err := parseQueries(strings.NewReader(`{"tenant_id":"t1"}`)); err == nil {
		t.Fatal("expected an error for a record missing prompt")
	}
}

func TestParseQueriesRejectsInvalidJSON(t *testing.T) {
	if _, err := parseQueries(strings.NewReader(`not json`)); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}

func TestRunRequiresInputFlag(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2 without --input, got %d", code)
	}
}

func TestRunRequiresOpenAIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("PGVECTOR_DSN", "postgres://unused")

	dir := t.TempDir()
	inputPath := dir + "/queries.jsonl"
	if err := os.WriteFile(inputPath, []byte(`{"tenant_id":"t1","prompt":"a"}`), 0o600); err != nil {
		t.Fatalf("write input file: %v", err)
	}

	var stdout, stderr strings.Builder
	code := run([]string{"--input", inputPath}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2 without OPENAI_API_KEY, got %d", code)
	}
}
