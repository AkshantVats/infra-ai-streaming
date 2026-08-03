// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"
)

func TestParsePromptsHappyPath(t *testing.T) {
	input := `{"tenant_id":"t1","prompt":"summarize this","response":"a summary"}
{"tenant_id":"t1","prompt":"translate this","response":"a translation"}
`
	got, err := parsePrompts(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parsePrompts: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(got))
	}
	if got[0].TenantID != "t1" || got[0].Prompt != "summarize this" || got[0].Response != "a summary" {
		t.Fatalf("unexpected first record: %+v", got[0])
	}
}

func TestParsePromptsSkipsBlankLines(t *testing.T) {
	input := "{\"tenant_id\":\"t1\",\"prompt\":\"a\"}\n\n{\"tenant_id\":\"t1\",\"prompt\":\"b\"}\n"
	got, err := parsePrompts(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parsePrompts: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(got))
	}
}

func TestParsePromptsRejectsMissingTenant(t *testing.T) {
	input := `{"prompt":"a"}`
	if _, err := parsePrompts(strings.NewReader(input)); err == nil {
		t.Fatal("expected an error for a record missing tenant_id")
	}
}

func TestParsePromptsRejectsMissingPrompt(t *testing.T) {
	input := `{"tenant_id":"t1"}`
	if _, err := parsePrompts(strings.NewReader(input)); err == nil {
		t.Fatal("expected an error for a record missing prompt")
	}
}

func TestParsePromptsRejectsInvalidJSON(t *testing.T) {
	input := `not json`
	if _, err := parsePrompts(strings.NewReader(input)); err == nil {
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
