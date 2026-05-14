//! Per-tenant distributed token bucket using Redis + Lua.

use anyhow::Context;

use crate::metrics;

const LUA: &str = r#"
local key = "ratelimit:" .. KEYS[1]
local now = tonumber(ARGV[1])
local default_rps = tonumber(ARGV[2])
local burst_mult = tonumber(ARGV[3])
local cost = tonumber(ARGV[4])
local capacity = math.floor(default_rps * burst_mult)

local data = redis.call("HMGET", key, "tokens", "last_refill")
local tokens = tonumber(data[1]) or capacity
local last_refill = tonumber(data[2]) or now

local elapsed_ms = now - last_refill
local refilled = (elapsed_ms / 1000.0) * default_rps
local new_tokens = math.min(capacity, tokens + refilled)

if new_tokens >= cost then
  redis.call("HMSET", key, "tokens", new_tokens - cost, "last_refill", now)
  redis.call("EXPIRE", key, 60)
  return {1, math.floor(new_tokens - cost)}
else
  local needed = cost - new_tokens
  local retry_ms = math.ceil((needed / default_rps) * 1000)
  return {0, retry_ms}
end
"#;

/// Outcome of a single rate-limit check.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum RateLimitResult {
    Allowed { remaining: u32 },
    Denied { retry_after_ms: u64 },
}

/// Redis-backed token bucket shared across ingestion replicas.
pub struct RateLimiter {
    redis_client: redis::Client,
    default_rps: u32,
    burst_multiplier: f32,
}

impl RateLimiter {
    pub fn new(redis_url: &str, default_rps: u32, burst_multiplier: f32) -> anyhow::Result<Self> {
        let redis_client = redis::Client::open(redis_url)
            .with_context(|| format!("invalid redis url {redis_url:?}"))?;
        Ok(Self {
            redis_client,
            default_rps,
            burst_multiplier,
        })
    }

    /// Consume `cost` tokens for `tenant_id`. On Redis errors, fail open (allowed).
    pub async fn check_and_consume(
        &self,
        tenant_id: &str,
        cost: u32,
    ) -> anyhow::Result<RateLimitResult> {
        let mut conn = match self.redis_client.get_multiplexed_async_connection().await {
            Ok(c) => c,
            Err(e) => {
                tracing::warn!(error = %e, tenant_id, "redis unavailable; rate limit fail-open");
                return Ok(RateLimitResult::Allowed { remaining: 0 });
            }
        };

        let now_ms = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_default()
            .as_millis() as i64;

        let script = redis::Script::new(LUA);
        let mut invocation = script.key(tenant_id);
        invocation
            .arg(now_ms)
            .arg(self.default_rps as i64)
            .arg(f64::from(self.burst_multiplier))
            .arg(cost as i64);
        let res: redis::RedisResult<(i64, i64)> = invocation.invoke_async(&mut conn).await;

        match res {
            Ok((flag, second)) => {
                if flag == 1 {
                    Ok(RateLimitResult::Allowed {
                        remaining: second.max(0) as u32,
                    })
                } else {
                    metrics::RATE_LIMITED_REQUESTS_TOTAL
                        .with_label_values(&[tenant_id])
                        .inc();
                    Ok(RateLimitResult::Denied {
                        retry_after_ms: second.max(0) as u64,
                    })
                }
            }
            Err(e) => {
                tracing::warn!(error = %e, tenant_id, "redis rate limit script failed; fail-open");
                Ok(RateLimitResult::Allowed { remaining: 0 })
            }
        }
    }
}
