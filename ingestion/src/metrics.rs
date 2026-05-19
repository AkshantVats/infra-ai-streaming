//! Prometheus metrics for the ingestion process (scrape via `/metrics` in a later binary).

use lazy_static::lazy_static;
use prometheus::{
    register_counter_vec, register_histogram_vec, register_int_counter,
    register_int_counter_vec, register_int_gauge, CounterVec, Encoder, HistogramVec, IntCounter,
    IntCounterVec, IntGauge, TextEncoder,
};

lazy_static! {
    pub static ref INGESTION_LATENCY_MS: HistogramVec = register_histogram_vec!(
        "ingestion_latency_ms",
        "HTTP ingest handler latency in milliseconds",
        &["tenant_id", "status"],
        vec![
            1.0, 5.0, 10.0, 25.0, 50.0, 100.0, 250.0, 500.0, 1000.0,
        ],
    )
    .expect("register ingestion_latency_ms");

    pub static ref KAFKA_PRODUCE_ERRORS_TOTAL: CounterVec = register_counter_vec!(
        "kafka_produce_errors_total",
        "Total Kafka produce errors by tenant and error type",
        &["tenant_id", "error_type"],
    )
    .expect("register kafka_produce_errors_total");

    pub static ref BATCH_SIZE_EVENTS: HistogramVec = register_histogram_vec!(
        "batch_size_events",
        "Number of events per ingest batch",
        &["tenant_id"],
        vec![1.0, 10.0, 50.0, 100.0, 250.0, 500.0, 1000.0],
    )
    .expect("register batch_size_events");

    pub static ref WAL_SEGMENTS_PENDING: IntGauge =
        register_int_gauge!("wal_segments_pending", "Number of WAL segments with un-acked entries",)
            .expect("register wal_segments_pending");

    pub static ref WAL_REPLAY_EVENTS_TOTAL: IntCounter = register_int_counter!(
        "wal_replay_events_total",
        "Total events replayed from WAL on startup",
    )
    .expect("register wal_replay_events_total");

    pub static ref RATE_LIMITED_REQUESTS_TOTAL: IntCounterVec = register_int_counter_vec!(
        "rate_limited_requests_total",
        "Requests rejected due to rate limiting",
        &["tenant_id"],
    )
    .expect("register rate_limited_requests_total");

    pub static ref BACKPRESSURE_EVENTS_TOTAL: IntCounter = register_int_counter!(
        "backpressure_events_total",
        "Requests rejected due to full internal channel (backpressure)",
    )
    .expect("register backpressure_events_total");

    pub static ref INGESTION_VALIDATION_ERRORS_TOTAL: IntCounterVec = register_int_counter_vec!(
        "ingestion_validation_errors_total",
        "Ingest requests rejected by schema validation",
        &["error"],
    )
    .expect("register ingestion_validation_errors_total");

    pub static ref REDIS_RATE_LIMIT_DEGRADED: IntCounter = register_int_counter!(
        "redis_rate_limit_degraded_total",
        "Rate limit checks that fell back to fail-open due to Redis unavailability",
    )
    .expect("register redis_rate_limit_degraded_total");
}

/// Prometheus exposition format for `GET /metrics`.
pub fn gather_metrics() -> anyhow::Result<String> {
    let mut buf = Vec::new();
    let encoder = TextEncoder::new();
    encoder.encode(&prometheus::gather(), &mut buf)?;
    Ok(String::from_utf8(buf)?)
}
