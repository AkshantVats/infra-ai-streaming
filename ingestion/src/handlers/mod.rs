//! HTTP handlers for the ingestion API.

pub mod ingest;

pub use ingest::{
    handle_ingest, tenant_from_events_json, AppState, InferenceEvent, IngestRequest,
    IngestResponse,
};
