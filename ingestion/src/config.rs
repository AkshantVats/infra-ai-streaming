//! Environment-backed configuration for the ingestion service.
//!
//! Variable names and local-dev placeholders are listed in `deploy/.env.example`
//! (copy to `deploy/.env` for Docker Compose). Rust defaults in `Config::from_env`
//! match localhost-oriented development only — not production.

use anyhow::Context;

/// Runtime configuration loaded from environment variables with documented defaults.
#[derive(Debug, Clone)]
pub struct Config {
    pub kafka_brokers: String,
    pub kafka_topic: String,
    pub kafka_dlq_topic: String,
    pub redis_url: String,
    pub http_port: u16,
    pub wal_dir: String,
    pub rate_limit_default_rps: u32,
    pub rate_limit_burst_multiplier: f32,
    /// Optional path to a JSON file with per-tenant rate limits.
    /// When set, per-tenant `max_events_per_sec` / `burst_multiplier` override
    /// the global defaults. See `deploy/tenant-limits.example.json`.
    pub tenant_limits_path: Option<String>,
    /// Bounded channel between HTTP handlers and Kafka drain task (see DESIGN.md §4).
    pub batch_channel_capacity: usize,
    pub max_batch_size: usize,
    pub max_event_age_ms: u64,
    /// Max in-flight HTTP requests (`ConcurrencyLimitLayer`).
    pub max_concurrent_requests: usize,
}

fn env_var(key: &str, default: &str) -> anyhow::Result<String> {
    match std::env::var(key) {
        Ok(v) if !v.is_empty() => Ok(v),
        _ => Ok(default.to_string()),
    }
}

fn parse_u16(key: &str, raw: &str) -> anyhow::Result<u16> {
    raw.parse::<u16>()
        .with_context(|| format!("{key} must be a valid u16 (got {raw:?})"))
}

fn parse_u32(key: &str, raw: &str) -> anyhow::Result<u32> {
    raw.parse::<u32>()
        .with_context(|| format!("{key} must be a valid u32 (got {raw:?})"))
}

fn parse_f32(key: &str, raw: &str) -> anyhow::Result<f32> {
    raw.parse::<f32>()
        .with_context(|| format!("{key} must be a valid f32 (got {raw:?})"))
}

fn parse_usize(key: &str, raw: &str) -> anyhow::Result<usize> {
    raw.parse::<usize>()
        .with_context(|| format!("{key} must be a valid usize (got {raw:?})"))
}

fn parse_u64(key: &str, raw: &str) -> anyhow::Result<u64> {
    raw.parse::<u64>()
        .with_context(|| format!("{key} must be a valid u64 (got {raw:?})"))
}

impl Config {
    /// Load configuration from process environment. Missing vars use defaults from DESIGN.md / build plan.
    pub fn from_env() -> anyhow::Result<Self> {
        let kafka_brokers = env_var("KAFKA_BROKERS", "localhost:9092")?;
        let kafka_topic = env_var("KAFKA_TOPIC", "ai_inference_events")?;
        let kafka_dlq_topic = env_var("KAFKA_DLQ_TOPIC", "ai_inference_dlq")?;
        let redis_url = env_var("REDIS_URL", "redis://localhost:6379")?;
        let http_port_raw = env_var("HTTP_PORT", "8080")?;
        let http_port = parse_u16("HTTP_PORT", &http_port_raw)?;
        let wal_dir = env_var("WAL_DIR", "/tmp/wal")?;
        let rate_limit_default_rps = parse_u32(
            "RATE_LIMIT_DEFAULT_RPS",
            &env_var("RATE_LIMIT_DEFAULT_RPS", "10000")?,
        )?;
        let rate_limit_burst_multiplier = parse_f32(
            "RATE_LIMIT_BURST_MULTIPLIER",
            &env_var("RATE_LIMIT_BURST_MULTIPLIER", "2.0")?,
        )?;
        // Default 10_000 aligns with DESIGN.md §4 (bounded channel).
        let batch_channel_capacity = parse_usize(
            "BATCH_CHANNEL_CAPACITY",
            &env_var("BATCH_CHANNEL_CAPACITY", "10000")?,
        )?;
        let max_batch_size = parse_usize("MAX_BATCH_SIZE", &env_var("MAX_BATCH_SIZE", "1000")?)?;
        let max_event_age_ms =
            parse_u64("MAX_EVENT_AGE_MS", &env_var("MAX_EVENT_AGE_MS", "3600000")?)?;
        let max_concurrent_requests = parse_usize(
            "MAX_CONCURRENT_REQUESTS",
            &env_var("MAX_CONCURRENT_REQUESTS", "1000")?,
        )?;
        let tenant_limits_path = std::env::var("TENANT_LIMITS_PATH")
            .ok()
            .filter(|v| !v.is_empty());

        Ok(Self {
            kafka_brokers,
            kafka_topic,
            kafka_dlq_topic,
            redis_url,
            http_port,
            wal_dir,
            rate_limit_default_rps,
            rate_limit_burst_multiplier,
            tenant_limits_path,
            batch_channel_capacity,
            max_batch_size,
            max_event_age_ms,
            max_concurrent_requests,
        })
    }
}
