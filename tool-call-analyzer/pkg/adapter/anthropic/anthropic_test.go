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
