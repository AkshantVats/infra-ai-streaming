// SPDX-License-Identifier: MIT
package schema

import "errors"

var (
	ErrMissingTraceID  = errors.New("span: trace_id is required")
	ErrMissingSpanID   = errors.New("span: span_id is required")
	ErrMissingToolName = errors.New("span: tool_name is required")
	ErrInvalidStatus   = errors.New("span: status must be one of ok/error/retry/timeout/cancelled")
)
