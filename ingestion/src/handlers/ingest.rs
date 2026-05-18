//! `POST /ingest` — validate, rate-limit, WAL append, enqueue for Kafka.

use std::sync::Arc;
use std::time::Instant;

use anyhow::Context;
use axum::extract::State;
use axum::http::{HeaderMap, HeaderValue, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::Json;
use bytes::Bytes;
use serde::{Deserialize, Serialize};
use serde_json::json;
use tokio::sync::{mpsc, Mutex};
use uuid::Uuid;

use crate::config::Config;
use crate::kafka::ProduceMessage;
use crate::metrics::{
    BACKPRESSURE_EVENTS_TOTAL, BATCH_SIZE_EVENTS, INGESTION_LATENCY_MS,
    INGESTION_VALIDATION_ERRORS_TOTAL,
};
use crate::rate_limit::{RateLimitResult, RateLimiter};
use crate::wal::WalWriter;

const TENANT_HEADER: &str = "x-tenant-id";

/// Canonical inference event (see README / DESIGN.md).
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct InferenceEvent {
    pub event_id: Option<String>,
    pub tenant_id: String,
    pub model_id: String,
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

/// Shared application state (cheaply cloned via inner `Arc`s).
#[derive(Clone)]
pub struct AppState {
    pub config: Arc<Config>,
    pub kafka_tx: mpsc::Sender<ProduceMessage>,
    pub wal_writer: Arc<Mutex<WalWriter>>,
    pub rate_limiter: Arc<RateLimiter>,
}

/// Validation failure returned before durable writes.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ValidationError {
    EmptyBatch,
    BatchTooLarge { max: usize },
    InvalidLatency { event_id: String },
    InvalidCost,
    EventTooOld,
}

impl ValidationError {
    fn metric_label(&self) -> &'static str {
        match self {
            ValidationError::EmptyBatch => "empty_batch",
            ValidationError::BatchTooLarge { .. } => "batch_too_large",
            ValidationError::InvalidLatency { .. } => "invalid_latency",
            ValidationError::InvalidCost => "invalid_cost",
            ValidationError::EventTooOld => "event_too_old",
        }
    }

    fn into_response(self) -> Response {
        INGESTION_VALIDATION_ERRORS_TOTAL
            .with_label_values(&[self.metric_label()])
            .inc();
        let (status, body) = match self {
            ValidationError::EmptyBatch => (
                StatusCode::BAD_REQUEST,
                json!({"error": "empty_batch"}),
            ),
            ValidationError::BatchTooLarge { max } => (
                StatusCode::BAD_REQUEST,
                json!({"error": "batch_too_large", "max": max}),
            ),
            ValidationError::InvalidLatency { event_id } => (
                StatusCode::BAD_REQUEST,
                json!({"error": "invalid_latency", "event_id": event_id}),
            ),
            ValidationError::InvalidCost => (
                StatusCode::BAD_REQUEST,
                json!({"error": "invalid_cost"}),
            ),
            ValidationError::EventTooOld => (
                StatusCode::BAD_REQUEST,
                json!({"error": "event_too_old"}),
            ),
        };
        (status, Json(body)).into_response()
    }
}

/// Validate batch size, per-event fields, and event age (no mutation).
pub fn validate_events(
    events: &[InferenceEvent],
    max_batch_size: usize,
    max_event_age_ms: u64,
    now_unix_ms: u64,
) -> Result<(), ValidationError> {
    if events.is_empty() {
        return Err(ValidationError::EmptyBatch);
    }
    if events.len() > max_batch_size {
        return Err(ValidationError::BatchTooLarge {
            max: max_batch_size,
        });
    }
    let min_timestamp = now_unix_ms.saturating_sub(max_event_age_ms);
    for event in events {
        if event.latency_ms == 0 {
            let event_id = event
                .event_id
                .clone()
                .unwrap_or_else(|| "unknown".to_string());
            return Err(ValidationError::InvalidLatency { event_id });
        }
        if event.cost_usd < 0.0 {
            return Err(ValidationError::InvalidCost);
        }
        if event.timestamp_unix_ms < min_timestamp {
            return Err(ValidationError::EventTooOld);
        }
    }
    Ok(())
}

/// Assign `event_id` and default `status` per plan.
pub fn normalize_events(events: &mut [InferenceEvent]) {
    for event in events.iter_mut() {
        if event.event_id.is_none() {
            event.event_id = Some(Uuid::new_v4().to_string());
        }
        if event.status.is_none() {
            event.status = Some("success".to_string());
        }
    }
}

fn unix_now_ms() -> anyhow::Result<u64> {
    Ok(std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .context("system clock before unix epoch")?
        .as_millis() as u64)
}

fn tenant_from_header(headers: &HeaderMap) -> Option<String> {
    headers
        .get(TENANT_HEADER)
        .and_then(|v| v.to_str().ok())
        .map(|s| s.trim().to_string())
        .filter(|s| !s.is_empty())
}

/// Extract partition key from WAL `events_json` (`{"events":[...]}`).
pub fn tenant_from_events_json(events_json: &str) -> anyhow::Result<String> {
    #[derive(Deserialize)]
    struct Payload {
        events: Vec<InferenceEvent>,
    }
    let payload: Payload =
        serde_json::from_str(events_json).context("parse WAL events_json for tenant")?;
    payload
        .events
        .first()
        .map(|e| e.tenant_id.clone())
        .context("WAL events_json has no events for partition key")
}

#[tracing::instrument(skip(state, body))]
pub async fn handle_ingest(
    State(state): State<AppState>,
    headers: HeaderMap,
    Json(mut body): Json<IngestRequest>,
) -> Response {
    let start = Instant::now();
    let tenant_id = match tenant_from_header(&headers) {
        Some(t) => t,
        None => {
            return (
                StatusCode::BAD_REQUEST,
                Json(json!({
                    "error": "missing_header",
                    "header": "X-Tenant-ID"
                })),
            )
                .into_response();
        }
    };

    let now_ms = match unix_now_ms() {
        Ok(ms) => ms,
        Err(e) => {
            tracing::error!(error = %e, "clock error");
            return (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(json!({"error": "internal_error"})),
            )
                .into_response();
        }
    };

    if let Err(err) = validate_events(
        &body.events,
        state.config.max_batch_size,
        state.config.max_event_age_ms,
        now_ms,
    ) {
        return err.into_response();
    }

    normalize_events(&mut body.events);

    let rate_result = match state
        .rate_limiter
        .check_and_consume(&tenant_id, body.events.len() as u32)
        .await
    {
        Ok(r) => r,
        Err(e) => {
            tracing::error!(error = %e, tenant_id, "rate limiter error");
            return (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(json!({"error": "internal_error"})),
            )
                .into_response();
        }
    };

    if let RateLimitResult::Denied { retry_after_ms } = rate_result {
        let retry_secs = (retry_after_ms / 1000).saturating_add(1);
        let mut response = (
            StatusCode::TOO_MANY_REQUESTS,
            Json(json!({
                "error": "rate_limit_exceeded",
                "retry_after_ms": retry_after_ms
            })),
        )
            .into_response();
        if let Ok(value) = HeaderValue::from_str(&retry_secs.to_string()) {
            response
                .headers_mut()
                .insert("retry-after", value);
        }
        return response;
    }

    let events_json = match serde_json::to_string(&json!({ "events": body.events })) {
        Ok(s) => s,
        Err(e) => {
            tracing::error!(error = %e, "serialize events for WAL");
            return (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(json!({"error": "internal_error"})),
            )
                .into_response();
        }
    };

    let batch_id = Uuid::new_v4().to_string();
    let event_count = body.events.len();

    let wal_entry_id = match state
        .wal_writer
        .lock()
        .await
        .append(&batch_id, &events_json)
    {
        Ok(id) => id,
        Err(e) => {
            tracing::error!(error = %e, batch_id = %batch_id, "WAL append failed");
            return (
                StatusCode::SERVICE_UNAVAILABLE,
                Json(json!({"error": "wal_failure"})),
            )
                .into_response();
        }
    };

    let msg = ProduceMessage {
        batch_id: batch_id.clone(),
        partition_key: tenant_id.clone(),
        payload: Bytes::from(events_json),
        wal_entry_id,
    };

    match state.kafka_tx.try_send(msg) {
        Ok(()) => {}
        Err(mpsc::error::TrySendError::Full(_)) => {
            BACKPRESSURE_EVENTS_TOTAL.inc();
            let mut response = (
                StatusCode::SERVICE_UNAVAILABLE,
                Json(json!({"error": "backpressure"})),
            )
                .into_response();
            response
                .headers_mut()
                .insert("retry-after", HeaderValue::from_static("1"));
            return response;
        }
        Err(mpsc::error::TrySendError::Closed(_)) => {
            tracing::error!("kafka channel closed");
            return (
                StatusCode::SERVICE_UNAVAILABLE,
                Json(json!({"error": "backpressure"})),
            )
                .into_response();
        }
    }

    INGESTION_LATENCY_MS
        .with_label_values(&[tenant_id.as_str(), "success"])
        .observe(start.elapsed().as_millis() as f64);
    BATCH_SIZE_EVENTS
        .with_label_values(&[tenant_id.as_str()])
        .observe(event_count as f64);

    (
        StatusCode::ACCEPTED,
        Json(IngestResponse {
            batch_id,
            event_count,
            accepted_at_unix_ms: now_ms,
        }),
    )
        .into_response()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn sample_event(ts: u64, latency: u32, cost: f64) -> InferenceEvent {
        InferenceEvent {
            event_id: None,
            tenant_id: "t1".into(),
            model_id: "gpt-4o".into(),
            timestamp_unix_ms: ts,
            latency_ms: latency,
            prefill_latency_ms: None,
            decode_latency_ms: None,
            prompt_tokens: 1,
            completion_tokens: 1,
            cost_usd: cost,
            status: None,
            error_code: None,
            request_id: None,
        }
    }

    #[test]
    fn validate_rejects_empty_batch() {
        let err = validate_events(&[], 1000, 3600_000, 1_000_000).unwrap_err();
        assert_eq!(err, ValidationError::EmptyBatch);
    }

    #[test]
    fn validate_rejects_oversized_batch() {
        let events: Vec<_> = (0..3).map(|_| sample_event(1_000_000, 1, 0.0)).collect();
        let err = validate_events(&events, 2, 3600_000, 1_000_000).unwrap_err();
        assert_eq!(
            err,
            ValidationError::BatchTooLarge { max: 2 }
        );
    }

    #[test]
    fn validate_rejects_zero_latency() {
        let events = vec![sample_event(1_000_000, 0, 0.0)];
        let err = validate_events(&events, 1000, 3600_000, 1_000_000).unwrap_err();
        assert!(matches!(err, ValidationError::InvalidLatency { .. }));
    }

    #[test]
    fn validate_rejects_negative_cost() {
        let events = vec![sample_event(1_000_000, 1, -0.01)];
        let err = validate_events(&events, 1000, 3600_000, 1_000_000).unwrap_err();
        assert_eq!(err, ValidationError::InvalidCost);
    }

    #[test]
    fn validate_rejects_stale_timestamp() {
        let events = vec![sample_event(1_000, 1, 0.0)];
        let err = validate_events(&events, 1000, 3600_000, 4_000_000).unwrap_err();
        assert_eq!(err, ValidationError::EventTooOld);
    }

    #[test]
    fn normalize_assigns_event_id_and_status() {
        let mut events = vec![sample_event(1_000_000, 1, 0.0)];
        normalize_events(&mut events);
        assert!(events[0].event_id.is_some());
        assert_eq!(events[0].status.as_deref(), Some("success"));
    }

    #[test]
    fn tenant_from_events_json_reads_first_event() {
        let json = r#"{"events":[{"tenant_id":"acme","model_id":"m","timestamp_unix_ms":1,"latency_ms":1,"prompt_tokens":0,"completion_tokens":0,"cost_usd":0.0}]}"#;
        assert_eq!(tenant_from_events_json(json).unwrap(), "acme");
    }
}
