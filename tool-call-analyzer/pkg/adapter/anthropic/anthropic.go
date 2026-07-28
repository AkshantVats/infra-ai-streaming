// SPDX-License-Identifier: MIT
// Package anthropic normalizes Anthropic tool_use content blocks into canonical ToolCall structs.
package anthropic

import (
	"encoding/json"

	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
)

// raw represents a single Anthropic content block of type "tool_use".
type raw struct {
	Type  string          `json:"type"`
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// Adapter normalizes Anthropic tool_use format.
type Adapter struct{}

func (a *Adapter) Vendor() string { return "anthropic" }

func (a *Adapter) CanHandle(b []byte) bool {
	if b == nil {
		return false
	}
	var probe raw
	if err := json.Unmarshal(b, &probe); err != nil {
		return false
	}
	return probe.Type == "tool_use" && probe.Name != ""
}

func (a *Adapter) Parse(b []byte) (types.ToolCall, error) {
	if b == nil {
		return types.ToolCall{}, types.ErrNilInput
	}
	var r raw
	if err := json.Unmarshal(b, &r); err != nil {
		return types.ToolCall{}, types.ErrUnknownFormat
	}
	if r.Name == "" {
		return types.ToolCall{}, types.ErrMissingField
	}

	inputJSON := "{}"
	if len(r.Input) > 0 {
		inputJSON = string(r.Input)
	}

	return types.ToolCall{
		ID:        r.ID,
		Name:      r.Name,
		Vendor:    a.Vendor(),
		Category:  types.ClassifyByName(r.Name),
		InputJSON: inputJSON,
	}, nil
}
