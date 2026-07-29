// SPDX-License-Identifier: MIT
// Package types defines the canonical ToolCall struct and supporting types
// for the tool-call-analyzer normalization pipeline.
package types

import (
	"errors"
	"strings"
)

// ToolCategory is the semantic type of a tool invocation.
type ToolCategory string

const (
	CategoryHTTP  ToolCategory = "http"
	CategoryDB    ToolCategory = "db"
	CategoryCode  ToolCategory = "code"
	CategoryFile  ToolCategory = "file"
	CategoryAgent ToolCategory = "agent"
)

// AllCategories enables exhaustiveness checks in tests.
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
	RetryReason    string  `json:"retry_reason"`
}

// ToolCall is the canonical normalized representation of a single tool invocation.
type ToolCall struct {
	ID      string `json:"id"`
	TraceID string `json:"trace_id"`
	SpanID  string `json:"span_id"`

	Name     string       `json:"name"`
	Vendor   string       `json:"vendor"`
	Category ToolCategory `json:"category"`

	InputJSON  string `json:"input_json"`
	OutputJSON string `json:"output_json"`

	StartedAtNs  int64 `json:"started_at_ns"`
	FinishedAtNs int64 `json:"finished_at_ns"`
	DurationMs   int64 `json:"duration_ms"`

	Cost    CostEstimate `json:"cost"`
	Retries RetryMeta    `json:"retries"`
}

var (
	ErrNilInput      = errors.New("nil input")
	ErrUnknownFormat = errors.New("unknown format")
	ErrMissingField  = errors.New("missing required field")
)

// perMillionUSD maps model name to cost per million tokens (input, output).
var perMillionUSD = map[string][2]float64{
	"gpt-4o":                    {2.50, 10.00},
	"gpt-4o-mini":               {0.15, 0.60},
	"gpt-4-turbo":               {10.00, 30.00},
	"claude-opus-4-8":           {15.00, 75.00},
	"claude-sonnet-4-6":         {3.00, 15.00},
	"claude-haiku-4-5-20251001": {0.80, 4.00},
}

// EstimateCost returns cost in USD for a given number of tokens. Returns 0.0 for unknown models.
func EstimateCost(inputTokens, outputTokens int, modelName string) float64 {
	rates, ok := perMillionUSD[modelName]
	if !ok {
		return 0.0
	}
	return float64(inputTokens)/1_000_000*rates[0] + float64(outputTokens)/1_000_000*rates[1]
}

// NewRetryMeta constructs RetryMeta with total cost attributed across all attempts.
func NewRetryMeta(retryCount int, attemptCostUSD float64, lastErr, reason string) RetryMeta {
	return RetryMeta{
		RetryCount:     retryCount,
		AttemptCostUSD: attemptCostUSD,
		TotalCostUSD:   attemptCostUSD * float64(retryCount+1),
		LastErrorMsg:   lastErr,
		RetryReason:    reason,
	}
}

// ClassifyByName returns the ToolCategory for a tool name using ordered priority rules.
// The default is CategoryHTTP for tools that don't match any keyword group.
func ClassifyByName(name string) ToolCategory {
	lower := strings.ToLower(name)
	for _, kw := range []string{"sql", "query", "db", "database", "vector", "redis", "elastic", "search_db", "pg_", "mysql"} {
		if strings.Contains(lower, kw) {
			return CategoryDB
		}
	}
	for _, kw := range []string{"run_", "exec", "python", "bash", "shell", "compile", "interpret", "code_"} {
		if strings.Contains(lower, kw) {
			return CategoryCode
		}
	}
	for _, kw := range []string{"file", "read_", "write_", "dir", "s3", "gcs", "blob", "fs_", "upload", "download"} {
		if strings.Contains(lower, kw) {
			return CategoryFile
		}
	}
	for _, kw := range []string{"agent", "delegate", "llm_", "chain", "subagent", "model_call"} {
		if strings.Contains(lower, kw) {
			return CategoryAgent
		}
	}
	return CategoryHTTP
}
