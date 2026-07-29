-- SPDX-License-Identifier: MIT
-- tool_calls source table: receives normalized ToolCall records from a Kafka consumer or HTTP insert.
-- ENGINE: MergeTree for single-node OSS. Replace with ReplicatedMergeTree in production clusters.
CREATE TABLE IF NOT EXISTS tool_calls (
    trace_id          String,
    tool_id           String,
    tool_name         LowCardinality(String),
    vendor            LowCardinality(String),
    category          LowCardinality(String),
    model_name        LowCardinality(String),
    input_tokens      UInt32,
    output_tokens     UInt32,
    cost_usd          Float64,
    duration_ms       UInt64,
    trace_duration_ms UInt64,
    has_error         UInt8,
    status            LowCardinality(String),
    timestamp         DateTime64(3, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (tool_name, vendor, timestamp)
TTL timestamp + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;
