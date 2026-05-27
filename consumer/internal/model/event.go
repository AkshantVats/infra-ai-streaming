package model

// InferenceEvent matches the Rust ingestion schema (ingestion/src/handlers/ingest.rs).
type InferenceEvent struct {
	EventID          *string `json:"event_id,omitempty"`
	TenantID         string  `json:"tenant_id"`
	ModelID          string  `json:"model_id"`
	TimestampUnixMs  uint64  `json:"timestamp_unix_ms"`
	LatencyMs        uint32  `json:"latency_ms"`
	PrefillLatencyMs *uint32 `json:"prefill_latency_ms,omitempty"`
	DecodeLatencyMs  *uint32 `json:"decode_latency_ms,omitempty"`
	PromptTokens     uint32  `json:"prompt_tokens"`
	CompletionTokens uint32  `json:"completion_tokens"`
	CostUSD          float64 `json:"cost_usd"`
	Status           *string `json:"status,omitempty"`
	ErrorCode        *string `json:"error_code,omitempty"`
	RequestID        *string `json:"request_id,omitempty"`
}

// IngestBatch is the Kafka record envelope produced by Rust ingestion.
type IngestBatch struct {
	Events []InferenceEvent `json:"events"`
}
