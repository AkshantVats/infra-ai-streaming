// SPDX-License-Identifier: MIT
// Package adapter defines the Adapter interface for normalizing vendor-specific
// tool call payloads into canonical types.ToolCall structs.
package adapter

import "github.com/AkshantVats/tool-call-analyzer/pkg/types"

// Adapter normalizes a vendor-specific tool call payload into a canonical ToolCall.
// Each vendor (openai, anthropic, langchain, llamaindex) implements this interface.
type Adapter interface {
	// Vendor returns the adapter's vendor identifier: "openai", "anthropic", "langchain", etc.
	Vendor() string

	// Parse normalizes raw vendor JSON into a canonical ToolCall.
	// Returns types.ErrNilInput for nil raw input.
	// Returns types.ErrUnknownFormat if raw cannot be parsed as this vendor's format.
	// Returns types.ErrMissingField if a required field is absent in a parseable payload.
	Parse(raw []byte) (types.ToolCall, error)

	// CanHandle returns true if this adapter recognizes the raw payload format.
	// Used for auto-detection in the registry when the vendor is not explicitly known.
	CanHandle(raw []byte) bool
}
