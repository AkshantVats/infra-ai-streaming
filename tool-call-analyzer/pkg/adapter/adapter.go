// SPDX-License-Identifier: MIT
// Package adapter defines the Adapter interface for normalizing vendor-specific
// tool call payloads into canonical types.ToolCall structs.
package adapter

import "github.com/AkshantVats/tool-call-analyzer/pkg/types"

// Adapter normalizes a vendor-specific tool call payload into a canonical ToolCall.
type Adapter interface {
	Vendor() string
	Parse(raw []byte) (types.ToolCall, error)
	CanHandle(raw []byte) bool
}
