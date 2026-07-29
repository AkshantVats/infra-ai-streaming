// SPDX-License-Identifier: MIT
package anthropic_test

import (
	"os"
	"testing"

	ant "github.com/AkshantVats/tool-call-analyzer/pkg/adapter/anthropic"
	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
)

func fixture(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return b
}

func TestAnthropic_ToolUseHTTP(t *testing.T) {
	a := &ant.Adapter{}
	raw := fixture(t, "../../../testdata/anthropic/tool_use_http.json")

	if !a.CanHandle(raw) {
		t.Fatal("expected CanHandle=true for anthropic tool_use_http fixture")
	}
	tc, err := a.Parse(raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if tc.Vendor != "anthropic" {
		t.Errorf("expected vendor=anthropic, got %q", tc.Vendor)
	}
	if tc.Category != types.CategoryHTTP {
		t.Errorf("expected category=http, got %q", tc.Category)
	}
	if tc.ID != "toolu_01A09q90qw90lq917835lq9" {
		t.Errorf("unexpected ID: %q", tc.ID)
	}
}

func TestAnthropic_ToolUseAgent(t *testing.T) {
	a := &ant.Adapter{}
	raw := fixture(t, "../../../testdata/anthropic/tool_use_agent.json")

	tc, err := a.Parse(raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if tc.Category != types.CategoryAgent {
		t.Errorf("expected category=agent for delegate_task, got %q", tc.Category)
	}
}

func TestAnthropic_NotOpenAI(t *testing.T) {
	a := &ant.Adapter{}
	// OpenAI format should not be handled by anthropic adapter
	raw := []byte(`{"id":"call_x","type":"function","function":{"name":"search","arguments":"{}"}}`)
	if a.CanHandle(raw) {
		t.Error("anthropic adapter should not handle OpenAI function format")
	}
}

func TestAnthropic_NilInput(t *testing.T) {
	a := &ant.Adapter{}
	_, err := a.Parse(nil)
	if err != types.ErrNilInput {
		t.Errorf("expected ErrNilInput, got %v", err)
	}
}

// TestAnthropic_CanHandle_Nil verifies CanHandle returns false for nil input.
func TestAnthropic_CanHandle_Nil(t *testing.T) {
	a := &ant.Adapter{}
	if a.CanHandle(nil) {
		t.Error("CanHandle(nil) should return false")
	}
}

// TestAnthropic_CanHandle_InvalidJSON verifies CanHandle returns false for malformed JSON.
func TestAnthropic_CanHandle_InvalidJSON(t *testing.T) {
	a := &ant.Adapter{}
	if a.CanHandle([]byte(`not-json`)) {
		t.Error("CanHandle(invalid-json) should return false")
	}
}

// TestAnthropic_Parse_InvalidJSON verifies Parse returns ErrUnknownFormat for bad JSON.
func TestAnthropic_Parse_InvalidJSON(t *testing.T) {
	a := &ant.Adapter{}
	_, err := a.Parse([]byte(`{bad json`))
	if err != types.ErrUnknownFormat {
		t.Errorf("expected ErrUnknownFormat for malformed JSON, got %v", err)
	}
}

// TestAnthropic_Parse_MissingName verifies Parse returns ErrMissingField when name is empty.
func TestAnthropic_Parse_MissingName(t *testing.T) {
	a := &ant.Adapter{}
	_, err := a.Parse([]byte(`{"type":"tool_use","id":"toolu_01","name":"","input":{}}`))
	if err != types.ErrMissingField {
		t.Errorf("expected ErrMissingField, got %v", err)
	}
}

// TestAnthropic_Vendor verifies Vendor returns the expected string.
func TestAnthropic_Vendor(t *testing.T) {
	a := &ant.Adapter{}
	if a.Vendor() != "anthropic" {
		t.Errorf("Vendor() = %q, want %q", a.Vendor(), "anthropic")
	}
}

// TestAnthropic_Parse_EmptyInput verifies Parse handles a payload with empty input field.
func TestAnthropic_Parse_EmptyInput(t *testing.T) {
	a := &ant.Adapter{}
	tc, err := a.Parse([]byte(`{"type":"tool_use","id":"toolu_01","name":"web_search","input":{}}`))
	if err != nil {
		t.Fatalf("Parse: unexpected error: %v", err)
	}
	if tc.InputJSON != "{}" {
		t.Errorf("InputJSON = %q, want {}", tc.InputJSON)
	}
}
