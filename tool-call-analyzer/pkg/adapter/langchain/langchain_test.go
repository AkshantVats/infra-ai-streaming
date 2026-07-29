// SPDX-License-Identifier: MIT
package langchain_test

import (
	"os"
	"testing"

	lc "github.com/AkshantVats/tool-call-analyzer/pkg/adapter/langchain"
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

func TestLangChain_AgentActionHTTP(t *testing.T) {
	a := &lc.Adapter{}
	raw := fixture(t, "../../../testdata/langchain/agent_action_http.json")

	if !a.CanHandle(raw) {
		t.Fatal("expected CanHandle=true for langchain agent_action_http fixture")
	}
	tc, err := a.Parse(raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if tc.Vendor != "langchain" {
		t.Errorf("expected vendor=langchain, got %q", tc.Vendor)
	}
	if tc.Category != types.CategoryHTTP {
		t.Errorf("expected category=http, got %q", tc.Category)
	}
	if tc.Name != "search_web" {
		t.Errorf("expected name=search_web, got %q", tc.Name)
	}
}

func TestLangChain_AgentActionDB(t *testing.T) {
	a := &lc.Adapter{}
	raw := fixture(t, "../../../testdata/langchain/agent_action_db.json")

	tc, err := a.Parse(raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if tc.Category != types.CategoryDB {
		t.Errorf("expected category=db for sql_query, got %q", tc.Category)
	}
}

func TestLangChain_MissingTool(t *testing.T) {
	a := &lc.Adapter{}
	_, err := a.Parse([]byte(`{"type":"AgentAction","tool":"","tool_input":{},"log":""}`))
	if err != types.ErrMissingField {
		t.Errorf("expected ErrMissingField, got %v", err)
	}
}

func TestLangChain_NilInput(t *testing.T) {
	a := &lc.Adapter{}
	_, err := a.Parse(nil)
	if err != types.ErrNilInput {
		t.Errorf("expected ErrNilInput, got %v", err)
	}
}

// TestLangChain_CanHandle_Nil verifies CanHandle returns false for nil input.
func TestLangChain_CanHandle_Nil(t *testing.T) {
	a := &lc.Adapter{}
	if a.CanHandle(nil) {
		t.Error("CanHandle(nil) should return false")
	}
}

// TestLangChain_CanHandle_InvalidJSON verifies CanHandle returns false for malformed JSON.
func TestLangChain_CanHandle_InvalidJSON(t *testing.T) {
	a := &lc.Adapter{}
	if a.CanHandle([]byte(`{bad-json`)) {
		t.Error("CanHandle(invalid-json) should return false")
	}
}

// TestLangChain_Parse_InvalidJSON verifies Parse returns ErrUnknownFormat for bad JSON.
func TestLangChain_Parse_InvalidJSON(t *testing.T) {
	a := &lc.Adapter{}
	_, err := a.Parse([]byte(`{bad json`))
	if err != types.ErrUnknownFormat {
		t.Errorf("expected ErrUnknownFormat for malformed JSON, got %v", err)
	}
}

// TestLangChain_Vendor verifies Vendor() returns the expected string.
func TestLangChain_Vendor(t *testing.T) {
	a := &lc.Adapter{}
	if a.Vendor() != "langchain" {
		t.Errorf("Vendor() = %q, want langchain", a.Vendor())
	}
}
