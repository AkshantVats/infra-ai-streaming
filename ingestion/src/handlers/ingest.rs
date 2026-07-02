// SPDX-License-Identifier: MIT
//! `POST /ingest` — validate, rate-limit, WAL append, enqueue for Kafka.

use std::sync::Arc;
use std::time::Instant;

use anyhow::Context;
use axum::extract::State;
use axum::http::{HeaderMap, HeaderValue, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::Json;
use bytes::Bytes;
use serde::Deserialize;
use serde_json::json;
use tokio::sync::{mpsc, Mutex};

use crate::config::Config;
use crate::kafka::ProduceMessage;
use crate::metrics::{BACKPRESSURE_EVENTS_TOTAL, BATCH_SIZE_EVENTS, INGESTION_LATENCY_MS};
use crate::rate_limit::{RateLimitResult, RateLimiter};
use crate::wal::WalWriter;

use super::event::{InferenceEvent, IngestRequest, IngestResponse};
use super::validate::{normalize_events, validate_events};

const TENANT_HEADER: &str = "x-tenant-id";

/// Shared application state (cheaply cloned via inner `Arc`s).
#[derive(Clone)]
pub struct AppState {
    pub config: Arc<Config>,
    pub kafka_tx: mpsc::Sender<ProduceMessage>,
    pub wal_writer: Arc<Mutex<WalWriter>>,
    pub rate_limiter: Arc<RateLimiter>,
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
            response.headers_mut().insert("retry-after", value);
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

    let batch_id = uuid::Uuid::new_v4().to_string();
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

    #[test]
    fn tenant_from_events_json_reads_first_event() {
        let json = r#"{"events":[{"tenant_id":"acme","model_id":"m","timestamp_unix_ms":1,"latency_ms":1,"prompt_tokens":0,"completion_tokens":0,"cost_usd":0.0}]}"#;
        assert_eq!(tenant_from_events_json(json).unwrap(), "acme");
    }
}
