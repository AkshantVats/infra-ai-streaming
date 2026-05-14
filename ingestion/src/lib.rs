//! Rust ingestion engine: config, metrics, WAL, and distributed rate limiting.
//!
//! HTTP server and Kafka producer are wired in a later milestone.

pub mod config;
pub mod metrics;
pub mod rate_limit;
pub mod wal;

pub use config::Config;
pub use metrics::gather_metrics;
pub use rate_limit::{RateLimitResult, RateLimiter};
pub use wal::{WalEntry, WalWriter};
