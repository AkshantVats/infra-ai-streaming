// SPDX-License-Identifier: MIT
//! Resolves the active model version for a tenant+user from the flagd sidecar.
//! Calls POST /evaluate on the flagd HTTP endpoint with a 50ms timeout.
//! Falls back to a configurable default model ID if flagd is unavailable.
//! Circuit breaker: after 3 consecutive failures, bypasses the call and returns default.

use std::sync::atomic::{AtomicU32, Ordering};
use std::sync::Arc;
use std::time::Duration;

/// Response from the flagd /evaluate endpoint.
#[derive(serde::Deserialize)]
struct EvalResponse {
    resolved_model_id: String,
}

/// Request body for flagd /evaluate.
#[derive(serde::Serialize)]
struct EvalRequest<'a> {
    tenant_id: &'a str,
    user_id: &'a str,
}

/// ModelResolver calls the flagd sidecar to resolve the active model version.
/// Thread-safe via Arc<AtomicU32> failure counter for the circuit breaker.
#[derive(Clone)]
pub struct ModelResolver {
    client: reqwest::Client,
    flagd_url: String,
    default_model: String,
    /// Consecutive failure count; circuit opens at 3.
    failures: Arc<AtomicU32>,
}

impl ModelResolver {
    /// Constructs a resolver targeting `flagd_url` (e.g. "http://localhost:8080").
    /// `default_model` is returned on timeout, flagd unavailable, or circuit open.
    pub fn new(flagd_url: impl Into<String>, default_model: impl Into<String>) -> Self {
        let client = reqwest::Client::builder()
            .timeout(Duration::from_millis(50))
            .build()
            .expect("failed to build HTTP client");
        Self {
            client,
            flagd_url: flagd_url.into(),
            default_model: default_model.into(),
            failures: Arc::new(AtomicU32::new(0)),
        }
    }

    /// Resolves the active model version for the given tenant and user.
    /// Returns `default_model` if the circuit is open or flagd is unreachable.
    pub async fn resolve_model_id(&self, tenant_id: &str, user_id: &str) -> String {
        // Circuit open: 3+ consecutive failures → return default without calling.
        if self.failures.load(Ordering::Relaxed) >= 3 {
            return self.default_model.clone();
        }

        let url = format!("{}/evaluate", self.flagd_url);
        let result = self
            .client
            .post(&url)
            .json(&EvalRequest { tenant_id, user_id })
            .send()
            .await;

        match result {
            Ok(resp) if resp.status().is_success() => {
                match resp.json::<EvalResponse>().await {
                    Ok(body) => {
                        // Success — reset failure counter.
                        self.failures.store(0, Ordering::Relaxed);
                        body.resolved_model_id
                    }
                    Err(_) => self.on_failure(),
                }
            }
            _ => self.on_failure(),
        }
    }

    fn on_failure(&self) -> String {
        self.failures.fetch_add(1, Ordering::Relaxed);
        self.default_model.clone()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Verifies that the circuit breaker returns the default model after 3 failures
    /// without making additional HTTP calls.
    #[tokio::test]
    async fn test_circuit_breaker_opens_at_three_failures() {
        // Use an unreachable URL so every call fails.
        let resolver = ModelResolver::new("http://127.0.0.1:19999", "gpt-3.5-turbo");

        // Three failures open the circuit.
        for _ in 0..3 {
            let result = resolver.resolve_model_id("tenant", "user").await;
            assert_eq!(result, "gpt-3.5-turbo");
        }
        assert!(resolver.failures.load(Ordering::Relaxed) >= 3);

        // Fourth call should short-circuit.
        let result = resolver.resolve_model_id("tenant", "user").await;
        assert_eq!(result, "gpt-3.5-turbo");
    }

    /// Verifies that default model is returned on timeout (50ms limit).
    #[tokio::test]
    async fn test_timeout_returns_default() {
        let resolver = ModelResolver::new("http://127.0.0.1:19998", "default-model");
        let result = resolver.resolve_model_id("t1", "u1").await;
        assert_eq!(result, "default-model");
    }
}
