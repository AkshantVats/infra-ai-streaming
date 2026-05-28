//! Ingestion engine binary: WAL + Kafka + HTTP `/ingest`.

use std::sync::Arc;

use anyhow::Context;
use bytes::Bytes;
use ingestion::{
    handlers::{tenant_from_events_json, AppState},
    kafka::{KafkaProducer, ProduceMessage},
    rate_limit::{RateLimiter, TenantLimitsConfig},
    server, Config, WalEntry, WalWriter,
};
use tracing_subscriber::EnvFilter;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let config = Arc::new(Config::from_env().context("failed to load config from environment")?);

    init_tracing();

    tracing::info!(
        version = ingestion::build_info::VERSION,
        git_sha = ingestion::build_info::GIT_SHA,
        build_time = ingestion::build_info::BUILD_TIME,
        "ingestion starting"
    );

    ping_redis(&config).await?;

    let wal = WalWriter::new(&config.wal_dir).context("init WAL")?;
    let unacked = wal.replay_unacked().context("WAL replay_unacked")?;
    let replay_count = unacked.len();
    tracing::info!(replayed = replay_count, "WAL replay complete");
    let wal = Arc::new(tokio::sync::Mutex::new(wal));

    let producer = Arc::new(
        KafkaProducer::new(
            &config.kafka_brokers,
            &config.kafka_topic,
            &config.kafka_dlq_topic,
        )
        .context("init Kafka producer")?,
    );

    let (kafka_tx, mut kafka_rx) =
        tokio::sync::mpsc::channel::<ProduceMessage>(config.batch_channel_capacity);

    for entry in unacked {
        match wal_entry_to_produce_message(entry) {
            Ok(msg) => {
                if kafka_tx.send(msg).await.is_err() {
                    tracing::warn!("kafka channel closed during WAL replay enqueue");
                    break;
                }
            }
            Err(e) => {
                tracing::warn!(error = %e, "skipping WAL replay entry (cannot build ProduceMessage)")
            }
        }
    }

    let producer_clone = Arc::clone(&producer);
    let wal_clone = Arc::clone(&wal);
    tokio::spawn(async move {
        while let Some(msg) = kafka_rx.recv().await {
            if let Err(e) = producer_clone.produce(msg, Arc::clone(&wal_clone)).await {
                tracing::error!(error = ?e, "Kafka produce error in drain task");
            }
        }
    });

    let tenant_limits = match &config.tenant_limits_path {
        Some(path) => {
            let cfg = TenantLimitsConfig::from_file(std::path::Path::new(path))
                .with_context(|| format!("load tenant limits from {path}"))?;
            tracing::info!(path = %path, "loaded per-tenant rate limits from file");
            cfg
        }
        None => {
            tracing::info!(
                default_rps = config.rate_limit_default_rps,
                burst = config.rate_limit_burst_multiplier,
                "no TENANT_LIMITS_PATH; using global defaults for all tenants"
            );
            TenantLimitsConfig::from_defaults(
                config.rate_limit_default_rps,
                config.rate_limit_burst_multiplier,
            )
        }
    };

    let rate_limiter =
        Arc::new(RateLimiter::new(&config.redis_url, tenant_limits).context("init rate limiter")?);

    let state = AppState {
        config: Arc::clone(&config),
        kafka_tx,
        wal_writer: wal,
        rate_limiter,
    };

    server::serve(config, state).await
}

fn init_tracing() {
    let filter = EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("info"));

    if let Ok(endpoint) = std::env::var("OTEL_EXPORTER_OTLP_ENDPOINT") {
        if !endpoint.is_empty() {
            tracing::warn!(
                endpoint = %endpoint,
                "OTEL_EXPORTER_OTLP_ENDPOINT is set; OTLP export wiring is deferred — structured logs only"
            );
        }
    }

    if std::env::var("LOG_FORMAT").unwrap_or_default() == "json" {
        tracing_subscriber::fmt()
            .json()
            .with_env_filter(filter)
            .init();
    } else {
        tracing_subscriber::fmt()
            .pretty()
            .with_env_filter(filter)
            .init();
    }
}

async fn ping_redis(config: &Config) -> anyhow::Result<()> {
    let redis_client =
        redis::Client::open(config.redis_url.as_str()).context("invalid REDIS_URL")?;
    let mut conn = redis_client
        .get_multiplexed_async_connection()
        .await
        .context("Redis unreachable — is Redis running?")?;
    redis::cmd("PING")
        .query_async::<String>(&mut conn)
        .await
        .context("Redis PING failed")?;
    tracing::info!("Redis connected");
    Ok(())
}

fn wal_entry_to_produce_message(entry: WalEntry) -> anyhow::Result<ProduceMessage> {
    let partition_key = tenant_from_events_json(&entry.events_json)?;
    Ok(ProduceMessage {
        batch_id: entry.batch_id,
        partition_key,
        payload: Bytes::from(entry.events_json),
        wal_entry_id: entry.entry_id,
    })
}
