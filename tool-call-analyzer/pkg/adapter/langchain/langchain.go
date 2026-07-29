// SPDX-License-Identifier: MIT
// Package langchain normalizes LangChain AgentAction payloads into canonical ToolCall structs.
package langchain

import (
	"encoding/json"

	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
)

// raw represents a LangChain AgentAction dict.
type raw struct {
	Type      string          `json:"type"`
	Tool      string          `json:"tool"`
	ToolInput json.RawMessage `json:"tool_input"`
	Log       string          `json:"log"`
}

// Adapter normalizes LangChain AgentAction format.
type Adapter struct{}

func (a *Adapter) Vendor() string { return "langchain" }

func (a *Adapter) CanHandle(b []byte) bool {
	if b == nil {
		return false
	}
	var probe raw
	if err := json.Unmarshal(b, &probe); err != nil {
		return false
	}
	return probe.Type == "AgentAction" && probe.Tool != ""
}

func (a *Adapter) Parse(b []byte) (types.ToolCall, error) {
	if b == nil {
		return types.ToolCall{}, types.ErrNilInput
	}
	var r raw
	if err := json.Unmarshal(b, &r); err != nil {
		return types.ToolCall{}, types.ErrUnknownFormat
	}
	if r.Tool == "" {
		return types.ToolCall{}, types.ErrMissingField
	}

	inputJSON := "{}"
	if len(r.ToolInput) > 0 {
		inputJSON = string(r.ToolInput)
	}

	return types.ToolCall{
		Name:      r.Tool,
		Vendor:    a.Vendor(),
		Category:  types.ClassifyByName(r.Tool),
		InputJSON: inputJSON,
	}, nil
}
