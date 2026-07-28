// SPDX-License-Identifier: MIT
// Package openai normalizes OpenAI tool_calls payloads into canonical ToolCall structs.
package openai

import (
	"encoding/json"
	"strings"

	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
)

// raw represents a single OpenAI tool_calls array element.
type raw struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Adapter normalizes OpenAI tool_calls format.
type Adapter struct{}

func (a *Adapter) Vendor() string { return "openai" }

func (a *Adapter) CanHandle(b []byte) bool {
	if b == nil {
		return false
	}
	var probe raw
	if err := json.Unmarshal(b, &probe); err != nil {
		return false
	}
	return probe.Type == "function" && probe.Function.Name != ""
}

func (a *Adapter) Parse(b []byte) (types.ToolCall, error) {
	if b == nil {
		return types.ToolCall{}, types.ErrNilInput
	}
	var r raw
	if err := json.Unmarshal(b, &r); err != nil {
		return types.ToolCall{}, types.ErrUnknownFormat
	}
	if r.Function.Name == "" {
		return types.ToolCall{}, types.ErrMissingField
	}
	if !strings.HasPrefix(r.ID, "call_") && r.ID != "" {
		// Accept any non-empty ID; OpenAI uses call_ prefix but allow flexibility
	}
	return types.ToolCall{
		ID:        r.ID,
		Name:      r.Function.Name,
		Vendor:    a.Vendor(),
		Category:  types.ClassifyByName(r.Function.Name),
		InputJSON: r.Function.Arguments,
	}, nil
}
