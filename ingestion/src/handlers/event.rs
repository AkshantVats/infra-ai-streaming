//! Request/response types for the ingest API.

use serde::{Deserialize, Serialize};

/// Canonical inference event (see README / DESIGN.md).
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct InferenceEvent {
    pub event_id: Option<String>,
    pub tenant_id: String,
    pub model_id: String,
    /// Fully-qualified model version resolved by the flagd sidecar at ingest time.
    /// Enables per-model-version cost attribution in ClickHouse even when model_id
    /// uses a logical alias (e.g. "gpt-4") rather than a versioned ID.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub resolved_model_id: Option<String>,
    pub timestamp_unix_ms: u64,
    pub latency_ms: u32,
    pub prefill_latency_ms: Option<u32>,
    pub decode_latency_ms: Option<u32>,
    pub prompt_tokens: u32,
    pub completion_tokens: u32,
    pub cost_usd: f64,
    pub status: Option<String>,
    pub error_code: Option<String>,
    pub request_id: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct IngestRequest {
    pub events: Vec<InferenceEvent>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct IngestResponse {
    pub batch_id: String,
    pub event_count: usize,
    pub accepted_at_unix_ms: u64,
}
