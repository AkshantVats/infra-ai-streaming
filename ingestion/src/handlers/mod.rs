// SPDX-License-Identifier: MIT
//! HTTP handlers for the ingestion API.

pub mod event;
pub mod ingest;
pub mod validate;

pub use event::{InferenceEvent, IngestRequest, IngestResponse};
pub use ingest::{handle_ingest, tenant_from_events_json, AppState};
pub use validate::{normalize_events, validate_events, ValidationError};
