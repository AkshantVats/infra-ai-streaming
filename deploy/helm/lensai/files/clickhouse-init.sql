-- Applied by the `clickhouse-init` service in `deploy/docker-compose.yml`
-- (`clickhouse-client --multiquery < /init.sql`). Full InferenceEvent schema for Day 5 writer.
CREATE DATABASE IF NOT EXISTS infra_ai;

DROP TABLE IF EXISTS infra_ai.inference_events;

CREATE TABLE infra_ai.inference_events
(
    event_id UUID,
    tenant_id LowCardinality(String),
    model_id LowCardinality(String),
    timestamp DateTime64(3),
    latency_ms UInt32,
    prefill_latency_ms Nullable(UInt32),
    decode_latency_ms Nullable(UInt32),
    prompt_tokens UInt32,
    completion_tokens UInt32,
    cost_usd Float64,
    status LowCardinality(String),
    error_code Nullable(String),
    request_id Nullable(String)
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(timestamp)
ORDER BY (tenant_id, model_id, timestamp);
