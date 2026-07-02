// SPDX-License-Identifier: MIT
//! Rust ingestion engine: config, metrics, WAL, rate limiting, and Kafka produce.

pub mod build_info;
pub mod config;
pub mod handlers;
pub mod kafka;
pub mod metrics;
pub mod model_resolver;
pub mod rate_limit;
pub mod server;
pub mod wal;

pub use config::Config;
pub use kafka::{KafkaProducer, ProduceMessage};
pub use metrics::gather_metrics;
pub use rate_limit::{RateLimitResult, RateLimiter, TenantLimitsConfig};
pub use wal::{WalEntry, WalWriter};
