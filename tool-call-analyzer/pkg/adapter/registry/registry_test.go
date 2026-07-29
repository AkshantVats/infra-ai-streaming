// SPDX-License-Identifier: MIT
package registry_test

import (
	"testing"

	"github.com/AkshantVats/tool-call-analyzer/pkg/adapter/registry"
	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
)

func TestRegistry_AutoDetect_OpenAI(t *testing.T) {
	r := registry.Default()
	raw := []byte(`{"id":"call_abc","type":"function","function":{"name":"search_web","arguments":"{\"q\":\"test\"}"}}`)
	tc, err := r.Parse(raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if tc.Vendor != "openai" {
		t.Errorf("expected vendor=openai, got %q", tc.Vendor)
	}
}

func TestRegistry_AutoDetect_Anthropic(t *testing.T) {
	r := registry.Default()
	raw := []byte(`{"type":"tool_use","id":"toolu_01","name":"search_web","input":{"query":"test"}}`)
	tc, err := r.Parse(raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if tc.Vendor != "anthropic" {
		t.Errorf("expected vendor=anthropic, got %q", tc.Vendor)
	}
}

func TestRegistry_AutoDetect_LangChain(t *testing.T) {
	r := registry.Default()
	raw := []byte(`{"type":"AgentAction","tool":"search_web","tool_input":{"query":"test"},"log":"invoking"}`)
	tc, err := r.Parse(raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if tc.Vendor != "langchain" {
		t.Errorf("expected vendor=langchain, got %q", tc.Vendor)
	}
}

func TestRegistry_UnknownFormat(t *testing.T) {
	r := registry.Default()
	_, err := r.Parse([]byte(`{"completely":"different","structure":"here"}`))
	if err != types.ErrUnknownFormat {
		t.Errorf("expected ErrUnknownFormat, got %v", err)
	}
}

func TestRegistry_VendorDetection(t *testing.T) {
	r := registry.Default()
	raw := []byte(`{"type":"tool_use","id":"toolu_x","name":"run_python","input":{}}`)
	v := r.Vendor(raw)
	if v != "anthropic" {
		t.Errorf("expected vendor=anthropic, got %q", v)
	}
}

// TestRegistry_Vendor_UnknownFormat verifies Vendor returns empty string when no adapter matches.
func TestRegistry_Vendor_UnknownFormat(t *testing.T) {
	r := registry.Default()
	v := r.Vendor([]byte(`{"completely":"unknown"}`))
	if v != "" {
		t.Errorf("expected empty vendor for unknown format, got %q", v)
	}
}

// TestRegistry_Parse_NilInput verifies Parse with nil returns ErrUnknownFormat.
func TestRegistry_Parse_Nil_ErrorPath(t *testing.T) {
	r := registry.Default()
	_, err := r.Parse(nil)
	if err != types.ErrUnknownFormat {
		t.Errorf("expected ErrUnknownFormat for nil, got %v", err)
	}
}
