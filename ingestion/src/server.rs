//! Axum router, middleware, and HTTP server lifecycle.

use std::net::SocketAddr;
use std::sync::Arc;
use std::time::Duration;

use anyhow::Context;
use axum::http::{header, StatusCode};
use axum::routing::{get, post};
use axum::{Json, Router};
use serde_json::json;
use tower::limit::ConcurrencyLimitLayer;
use tower_http::timeout::TimeoutLayer;
use tower_http::trace::TraceLayer;

use crate::config::Config;
use crate::handlers::{handle_ingest, AppState};
use crate::metrics::gather_metrics;

/// Build the ingestion HTTP router with concurrency, timeout, and tracing layers.
pub fn build_router(state: AppState) -> Router {
    let max_concurrent = state.config.max_concurrent_requests;

    Router::new()
        .route("/ingest", post(handle_ingest))
        .route(
            "/health",
            get(|| async { (StatusCode::OK, Json(json!({"status": "ok"}))) }),
        )
        .route(
            "/metrics",
            get(|| async {
                match gather_metrics() {
                    Ok(body) => (
                        [(header::CONTENT_TYPE, "text/plain; version=0.0.4")],
                        body,
                    ),
                    Err(e) => {
                        tracing::error!(error = %e, "gather metrics failed");
                        (
                            [(header::CONTENT_TYPE, "text/plain; version=0.0.4")],
                            format!("# metrics error: {e}\n"),
                        )
                    }
                }
            }),
        )
        .layer(TraceLayer::new_for_http())
        .layer(TimeoutLayer::new(Duration::from_secs(30)))
        .layer(ConcurrencyLimitLayer::new(max_concurrent))
        .with_state(state)
}

/// Bind and serve until graceful shutdown (SIGINT / SIGTERM).
pub async fn serve(config: Arc<Config>, state: AppState) -> anyhow::Result<()> {
    let addr = SocketAddr::from(([0, 0, 0, 0], config.http_port));
    let router = build_router(state);
    let listener = tokio::net::TcpListener::bind(addr)
        .await
        .with_context(|| format!("bind 0.0.0.0:{}", config.http_port))?;
    tracing::info!(port = config.http_port, "ingestion engine listening");

    axum::serve(listener, router)
        .with_graceful_shutdown(shutdown_signal())
        .await
        .context("axum serve")?;
    Ok(())
}

async fn shutdown_signal() {
    let ctrl_c = async {
        tokio::signal::ctrl_c()
            .await
            .expect("failed to install Ctrl+C handler");
    };

    #[cfg(unix)]
    let terminate = async {
        tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())
            .expect("failed to install SIGTERM handler")
            .recv()
            .await;
    };

    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        () = ctrl_c => tracing::info!("SIGINT received, shutting down"),
        () = terminate => tracing::info!("SIGTERM received, shutting down"),
    }
}

#[cfg(test)]
mod tests {
    use std::sync::Arc;

    use axum::body::Body;
    use axum::http::{Request, StatusCode};
    use tower::ServiceExt;

    use crate::config::Config;
    use crate::handlers::AppState;
    use crate::kafka::ProduceMessage;
    use crate::rate_limit::{RateLimiter, TenantLimitsConfig};
    use crate::wal::WalWriter;

    use super::build_router;

    fn test_config() -> Arc<Config> {
        Arc::new(Config {
            kafka_brokers: "localhost:9092".into(),
            kafka_topic: "ai_inference_events".into(),
            kafka_dlq_topic: "ai_inference_dlq".into(),
            redis_url: "redis://localhost:6379".into(),
            http_port: 0,
            wal_dir: "/tmp/wal-test-unused".into(),
            rate_limit_default_rps: 10_000,
            rate_limit_burst_multiplier: 2.0,
            tenant_limits_path: None,
            batch_channel_capacity: 8,
            max_batch_size: 1000,
            max_event_age_ms: 3_600_000,
            max_concurrent_requests: 100,
        })
    }

    #[tokio::test]
    async fn health_returns_ok() {
        let dir = tempfile::tempdir().expect("tempdir");
        let wal = WalWriter::new(dir.path().to_str().expect("path")).expect("wal");
        let (tx, _rx) = tokio::sync::mpsc::channel::<ProduceMessage>(4);
        let config = test_config();
        let state = AppState {
            config: Arc::clone(&config),
            kafka_tx: tx,
            wal_writer: Arc::new(tokio::sync::Mutex::new(wal)),
            rate_limiter: Arc::new(
                RateLimiter::new(
                    &config.redis_url,
                    TenantLimitsConfig::from_defaults(config.rate_limit_default_rps, 2.0),
                )
                .expect("rate limiter"),
            ),
        };
        let app = build_router(state);
        let response = app
            .oneshot(
                Request::builder()
                    .uri("/health")
                    .body(Body::empty())
                    .expect("request"),
            )
            .await
            .expect("response");
        assert_eq!(response.status(), StatusCode::OK);
    }
}
