//! Batch validation and normalization before durable writes.

use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::Json;
use serde_json::json;
use uuid::Uuid;

use crate::metrics::INGESTION_VALIDATION_ERRORS_TOTAL;

use super::event::InferenceEvent;

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

    pub fn into_response(self) -> Response {
        INGESTION_VALIDATION_ERRORS_TOTAL
            .with_label_values(&[self.metric_label()])
            .inc();
        let (status, body) = match self {
            ValidationError::EmptyBatch => {
                (StatusCode::BAD_REQUEST, json!({"error": "empty_batch"}))
            }
            ValidationError::BatchTooLarge { max } => (
                StatusCode::BAD_REQUEST,
                json!({"error": "batch_too_large", "max": max}),
            ),
            ValidationError::InvalidLatency { event_id } => (
                StatusCode::BAD_REQUEST,
                json!({"error": "invalid_latency", "event_id": event_id}),
            ),
            ValidationError::InvalidCost => {
                (StatusCode::BAD_REQUEST, json!({"error": "invalid_cost"}))
            }
            ValidationError::EventTooOld => {
                (StatusCode::BAD_REQUEST, json!({"error": "event_too_old"}))
            }
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

/// Assign `event_id` and default `status` when absent.
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
        let err = validate_events(&[], 1000, 3_600_000, 1_000_000).unwrap_err();
        assert_eq!(err, ValidationError::EmptyBatch);
    }

    #[test]
    fn validate_rejects_oversized_batch() {
        let events: Vec<_> = (0..3).map(|_| sample_event(1_000_000, 1, 0.0)).collect();
        let err = validate_events(&events, 2, 3_600_000, 1_000_000).unwrap_err();
        assert_eq!(err, ValidationError::BatchTooLarge { max: 2 });
    }

    #[test]
    fn validate_rejects_zero_latency() {
        let events = vec![sample_event(1_000_000, 0, 0.0)];
        let err = validate_events(&events, 1000, 3_600_000, 1_000_000).unwrap_err();
        assert!(matches!(err, ValidationError::InvalidLatency { .. }));
    }

    #[test]
    fn validate_rejects_negative_cost() {
        let events = vec![sample_event(1_000_000, 1, -0.01)];
        let err = validate_events(&events, 1000, 3_600_000, 1_000_000).unwrap_err();
        assert_eq!(err, ValidationError::InvalidCost);
    }

    #[test]
    fn validate_rejects_stale_timestamp() {
        let events = vec![sample_event(1_000, 1, 0.0)];
        let err = validate_events(&events, 1000, 3_600_000, 4_000_000).unwrap_err();
        assert_eq!(err, ValidationError::EventTooOld);
    }

    #[test]
    fn normalize_assigns_event_id_and_status() {
        let mut events = vec![sample_event(1_000_000, 1, 0.0)];
        normalize_events(&mut events);
        assert!(events[0].event_id.is_some());
        assert_eq!(events[0].status.as_deref(), Some("success"));
    }
}
