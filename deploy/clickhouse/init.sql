-- Applied by the `clickhouse-init` service in `deploy/docker-compose.yml`
-- (`clickhouse-client --multiquery < /init.sql`). Placeholder MergeTree for local dev.
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
