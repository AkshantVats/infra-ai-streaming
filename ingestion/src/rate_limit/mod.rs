//! Distributed per-tenant rate limiting (Redis token bucket).

pub mod token_bucket;

pub use token_bucket::{RateLimitResult, RateLimiter};
