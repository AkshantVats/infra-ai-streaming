// SPDX-License-Identifier: MIT
// Package registry provides auto-detection of vendor adapters from raw payloads.
package registry

import (
	"github.com/AkshantVats/tool-call-analyzer/pkg/adapter"
	"github.com/AkshantVats/tool-call-analyzer/pkg/adapter/anthropic"
	"github.com/AkshantVats/tool-call-analyzer/pkg/adapter/langchain"
	"github.com/AkshantVats/tool-call-analyzer/pkg/adapter/openai"
	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
)

// Registry holds ordered adapters for auto-detection.
type Registry struct {
	adapters []adapter.Adapter
}

// Default returns a Registry with all known adapters in priority order.
// Priority: anthropic > openai > langchain (langchain is a meta-framework, probe last).
func Default() *Registry {
	return &Registry{
		adapters: []adapter.Adapter{
			&anthropic.Adapter{},
			&openai.Adapter{},
			&langchain.Adapter{},
		},
	}
}

// Parse auto-detects the vendor and normalizes the payload.
// Returns ErrUnknownFormat if no adapter recognizes the payload.
func (r *Registry) Parse(raw []byte) (types.ToolCall, error) {
	for _, a := range r.adapters {
		if a.CanHandle(raw) {
			return a.Parse(raw)
		}
	}
	return types.ToolCall{}, types.ErrUnknownFormat
}

// Vendor returns the vendor name for the first matching adapter, or empty string.
func (r *Registry) Vendor(raw []byte) string {
	for _, a := range r.adapters {
		if a.CanHandle(raw) {
			return a.Vendor()
		}
	}
	return ""
}
