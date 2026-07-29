// SPDX-License-Identifier: MIT
package openai_test

import (
	"os"
	"testing"

	oai "github.com/AkshantVats/tool-call-analyzer/pkg/adapter/openai"
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

func TestOpenAI_SearchWeb(t *testing.T) {
	a := &oai.Adapter{}
	raw := fixture(t, "../../../testdata/openai/search_web_call.json")

	if !a.CanHandle(raw) {
		t.Fatal("expected CanHandle=true for openai search_web fixture")
	}
	tc, err := a.Parse(raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if tc.Vendor != "openai" {
		t.Errorf("expected vendor=openai, got %q", tc.Vendor)
	}
	if tc.Name != "search_web" {
		t.Errorf("expected name=search_web, got %q", tc.Name)
	}
	if tc.Category != types.CategoryHTTP {
		t.Errorf("expected category=http, got %q", tc.Category)
	}
	if tc.ID != "call_abc123" {
		t.Errorf("expected id=call_abc123, got %q", tc.ID)
	}
	if tc.InputJSON == "" {
		t.Error("expected non-empty InputJSON")
	}
}

func TestOpenAI_CodeExec(t *testing.T) {
	a := &oai.Adapter{}
	raw := fixture(t, "../../../testdata/openai/code_exec_call.json")

	tc, err := a.Parse(raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if tc.Category != types.CategoryCode {
		t.Errorf("expected category=code for run_python, got %q", tc.Category)
	}
}

func TestOpenAI_NilInput(t *testing.T) {
	a := &oai.Adapter{}
	_, err := a.Parse(nil)
	if err != types.ErrNilInput {
		t.Errorf("expected ErrNilInput, got %v", err)
	}
}

func TestOpenAI_MissingName(t *testing.T) {
	a := &oai.Adapter{}
	_, err := a.Parse([]byte(`{"id":"call_x","type":"function","function":{"name":"","arguments":"{}"}}`))
	if err != types.ErrMissingField {
		t.Errorf("expected ErrMissingField, got %v", err)
	}
}

// TestOpenAI_CanHandle_Nil verifies CanHandle returns false for nil input.
func TestOpenAI_CanHandle_Nil(t *testing.T) {
	a := &oai.Adapter{}
	if a.CanHandle(nil) {
		t.Error("CanHandle(nil) should return false")
	}
}

// TestOpenAI_CanHandle_InvalidJSON verifies CanHandle returns false for bad JSON.
func TestOpenAI_CanHandle_InvalidJSON(t *testing.T) {
	a := &oai.Adapter{}
	if a.CanHandle([]byte(`{not-json`)) {
		t.Error("CanHandle(invalid-json) should return false")
	}
}

// TestOpenAI_Parse_InvalidJSON verifies Parse returns ErrUnknownFormat for bad JSON.
func TestOpenAI_Parse_InvalidJSON(t *testing.T) {
	a := &oai.Adapter{}
	_, err := a.Parse([]byte(`{bad json`))
	if err != types.ErrUnknownFormat {
		t.Errorf("expected ErrUnknownFormat for malformed JSON, got %v", err)
	}
}

// TestOpenAI_Vendor verifies Vendor() returns the expected string.
func TestOpenAI_Vendor(t *testing.T) {
	a := &oai.Adapter{}
	if a.Vendor() != "openai" {
		t.Errorf("Vendor() = %q, want openai", a.Vendor())
	}
}

// TestOpenAI_Parse_NonCallPrefix verifies that a non-"call_" prefixed ID is still accepted.
func TestOpenAI_Parse_NonCallPrefix(t *testing.T) {
	a := &oai.Adapter{}
	tc, err := a.Parse([]byte(`{"id":"custom_id","type":"function","function":{"name":"search_web","arguments":"{}"}}`))
	if err != nil {
		t.Fatalf("Parse: unexpected error: %v", err)
	}
	if tc.ID != "custom_id" {
		t.Errorf("ID = %q, want custom_id", tc.ID)
	}
}
