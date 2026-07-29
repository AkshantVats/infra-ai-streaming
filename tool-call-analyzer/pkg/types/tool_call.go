// SPDX-License-Identifier: MIT
// Package types defines the canonical ToolCall struct and supporting types
// for the tool-call-analyzer normalization pipeline.
package types

import "errors"

// ToolCategory is the semantic type of a tool invocation.
// Used for grouping metrics, setting latency budgets, and routing alerts.
type ToolCategory string

const (
	// CategoryHTTP covers all tool calls that make outbound HTTP requests.
	// Examples: search_web, get_weather, fetch_url, call_api
	CategoryHTTP ToolCategory = "http"

	// CategoryDB covers all tool calls that query or write to a database.
	// Examples: sql_query, vector_search, redis_get, elasticsearch_search
	CategoryDB ToolCategory = "db"

	// CategoryCode covers all tool calls that execute or analyze code.
	// Examples: run_python, bash_exec, code_interpreter, compile_check
	CategoryCode ToolCategory = "code"

	// CategoryFile covers all tool calls that read or write to a filesystem.
	// Examples: read_file, write_file, list_dir, fetch_s3_object
	CategoryFile ToolCategory = "file"

	// CategoryAgent covers all tool calls that invoke a sub-agent or model.
	// Examples: call_subagent, delegate_task, run_llm_chain
	CategoryAgent ToolCategory = "agent"
)

// AllCategories is used in tests to verify exhaustiveness.
var AllCategories = []ToolCategory{
	CategoryHTTP, CategoryDB, CategoryCode, CategoryFile, CategoryAgent,
}

// CostEstimate records the LLM token cost for the call that produced this tool invocation.
type CostEstimate struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	ModelName    string  `json:"model_name"`
	CostUSD      float64 `json:"cost_usd"`
}

// RetryMeta records retry count and total cost attribution.
type RetryMeta struct {
	RetryCount     int     `json:"retry_count"`
	AttemptCostUSD float64 `json:"attempt_cost_usd"`
	TotalCostUSD   float64 `json:"total_cost_usd"`
	LastErrorMsg   string  `json:"last_error_msg"`
	RetryReason    string  `json:"retry_reason"` // "timeout" | "rate_limit" | "server_error" | "empty_response"
}

// ToolCall is the canonical normalized representation of a single tool invocation.
// All adapter implementations must populate every documented required field.
type ToolCall struct {
	// Identity
	ID      string `json:"id"`
	TraceID string `json:"trace_id"`
	SpanID  string `json:"span_id"`

	// Tool identity
	Name     string       `json:"name"`
	Vendor   string       `json:"vendor"`
	Category ToolCategory `json:"category"`

	// Invocation payload
	InputJSON  string `json:"input_json"`
	OutputJSON string `json:"output_json"`

	// Timing
	StartedAtNs  int64 `json:"started_at_ns"`
	FinishedAtNs int64 `json:"finished_at_ns"`
	DurationMs   int64 `json:"duration_ms"`

	// Cost
	Cost    CostEstimate `json:"cost"`
	Retries RetryMeta    `json:"retries"`

	// Status
	Status   string `json:"status"`
	ErrorMsg string `json:"error_msg"`
	HasError bool   `json:"has_error"`

	// Source metadata
	ModelName    string `json:"model_name"`
	AgentStep    int    `json:"agent_step"`
	FrameworkVer string `json:"framework_ver"`
}

// perMillionTokens holds USD cost per 1M tokens: [inputCostPerM, outputCostPerM].
// Source: published pricing as of 2026-07. Update when pricing changes.
var perMillionTokens = map[string][2]float64{
	"gpt-4o":                    {2.50, 10.00},
	"gpt-4o-mini":               {0.15, 0.60},
	"gpt-4-turbo":               {10.00, 30.00},
	"claude-opus-4-8":           {15.00, 75.00},
	"claude-sonnet-4-6":         {3.00, 15.00},
	"claude-haiku-4-5-20251001": {0.80, 4.00},
}

// EstimateCost returns the USD cost for a single LLM call given token counts and model name.
// Returns 0.0 for unknown models.
func EstimateCost(inputTokens, outputTokens int, modelName string) float64 {
	prices, ok := perMillionTokens[modelName]
	if !ok {
		return 0.0
	}
	return float64(inputTokens)/1_000_000*prices[0] + float64(outputTokens)/1_000_000*prices[1]
}

// NewRetryMeta constructs RetryMeta with TotalCostUSD computed from attempt cost × (retries+1).
func NewRetryMeta(retryCount int, attemptCostUSD float64, lastErr, reason string) RetryMeta {
	return RetryMeta{
		RetryCount:     retryCount,
		AttemptCostUSD: attemptCostUSD,
		TotalCostUSD:   attemptCostUSD * float64(retryCount+1),
		LastErrorMsg:   lastErr,
		RetryReason:    reason,
	}
}

// Sentinel errors for adapter implementations.
var (
	ErrNilInput      = errors.New("tool-call-analyzer: nil input to adapter")
	ErrUnknownFormat = errors.New("tool-call-analyzer: unrecognized tool call format")
	ErrMissingField  = errors.New("tool-call-analyzer: required field missing in payload")
)
