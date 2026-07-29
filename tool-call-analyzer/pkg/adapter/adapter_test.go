// SPDX-License-Identifier: MIT
package adapter_test

import (
	"errors"
	"testing"

	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
)

// stubbedAdapter is a minimal adapter for testing the interface contract.
type stubbedAdapter struct{}

func (s *stubbedAdapter) Vendor() string { return "stub" }

func (s *stubbedAdapter) CanHandle(raw []byte) bool { return len(raw) > 0 }

func (s *stubbedAdapter) Parse(raw []byte) (types.ToolCall, error) {
	if raw == nil {
		return types.ToolCall{}, types.ErrNilInput
	}
	if len(raw) == 0 {
		return types.ToolCall{}, types.ErrUnknownFormat
	}
	return types.ToolCall{Vendor: "stub"}, nil
}

func TestAdapterNilInput(t *testing.T) {
	a := &stubbedAdapter{}
	_, err := a.Parse(nil)
	if !errors.Is(err, types.ErrNilInput) {
		t.Errorf("expected ErrNilInput for nil input, got: %v", err)
	}
}

func TestAdapterEmptyInput(t *testing.T) {
	a := &stubbedAdapter{}
	_, err := a.Parse([]byte{})
	if !errors.Is(err, types.ErrUnknownFormat) {
		t.Errorf("expected ErrUnknownFormat for empty input, got: %v", err)
	}
}

func TestAdapterHappyPath(t *testing.T) {
	a := &stubbedAdapter{}
	tc, err := a.Parse([]byte(`{"tool": "test"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tc.Vendor != "stub" {
		t.Errorf("expected vendor=stub, got %s", tc.Vendor)
	}
}

func TestCanHandleNilReturnsFalse(t *testing.T) {
	a := &stubbedAdapter{}
	if a.CanHandle(nil) {
		t.Error("CanHandle(nil) should return false")
	}
}
