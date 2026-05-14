-- Local / first-boot initialization (official clickhouse-server image runs
-- scripts in /docker-entrypoint-initdb.d when the data directory is empty).

CREATE DATABASE IF NOT EXISTS infra_ai;

CREATE TABLE IF NOT EXISTS infra_ai.inference_events
(
    tenant_id String,
    model_id String,
    timestamp DateTime64(3)
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(timestamp)
ORDER BY (tenant_id, model_id, timestamp);
