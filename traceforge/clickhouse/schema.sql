-- SPDX-License-Identifier: MIT
-- TraceForge: agent_spans schema
-- Day 34 — Trace Storage Layout

-- ============================================================
-- Kafka ingestion table (raw, string columns for JSON parsing)
-- ============================================================
CREATE TABLE IF NOT EXISTS agent_spans_kafka
(
    raw String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'localhost:9092',
    kafka_topic_list  = 'agent-spans',
    kafka_group_name  = 'traceforge-clickhouse',
    kafka_format      = 'RawBLOB',
    kafka_num_consumers = 2;

-- ============================================================
-- Main MergeTree table
-- ORDER BY (trace_id, start_time) optimises the primary query pattern:
--   "give me all spans for trace X, in chronological order"
-- Secondary pattern (slow spans across all traces) served by projection.
-- ============================================================
CREATE TABLE IF NOT EXISTS agent_spans
(
    -- Trace identity
    trace_id        String,
    span_id         String,
    parent_span_id  String     DEFAULT '',

    -- What ran
    tool_name       String,
    tool_kind       LowCardinality(String),
    model           LowCardinality(String)    DEFAULT '',

    -- Outcome
    status          LowCardinality(String),
    error_message   String                    DEFAULT '',

    -- Timing
    start_time      DateTime64(3, 'UTC'),
    latency_ms      Int64,

    -- Cost
    input_tokens    UInt32                    DEFAULT 0,
    output_tokens   UInt32                    DEFAULT 0,
    total_tokens    UInt32                    DEFAULT 0,
    cost_usd        Float64                   DEFAULT 0.0,

    -- Free-form attributes stored as JSON string
    attributes      String                    DEFAULT '{}'
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(start_time)
ORDER BY (trace_id, start_time)
TTL toDateTime(start_time) + INTERVAL 30 DAY
SETTINGS
    index_granularity = 8192,
    min_bytes_for_wide_part = 10485760,
    compress_by = 'ZSTD(3)';

-- ============================================================
-- Materialized view: Kafka → agent_spans
-- Parses each raw JSON message and inserts into agent_spans.
-- ============================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS agent_spans_mv
TO agent_spans
AS SELECT
    JSONExtractString(raw, 'trace_id')       AS trace_id,
    JSONExtractString(raw, 'span_id')        AS span_id,
    JSONExtractString(raw, 'parent_span_id') AS parent_span_id,
    JSONExtractString(raw, 'tool_name')      AS tool_name,
    JSONExtractString(raw, 'tool_kind')      AS tool_kind,
    JSONExtractString(raw, 'model')          AS model,
    JSONExtractString(raw, 'status')         AS status,
    JSONExtractString(raw, 'error_message')  AS error_message,
    parseDateTimeBestEffort(JSONExtractString(raw, 'start_time')) AS start_time,
    JSONExtractInt(raw, 'latency_ms')        AS latency_ms,
    JSONExtractUInt(raw, 'input_tokens')     AS input_tokens,
    JSONExtractUInt(raw, 'output_tokens')    AS output_tokens,
    JSONExtractUInt(raw, 'total_tokens')     AS total_tokens,
    JSONExtractFloat(raw, 'cost_usd')        AS cost_usd,
    ifNull(JSONExtractString(raw, 'attributes'), '{}') AS attributes
FROM agent_spans_kafka;

-- ============================================================
-- Projection: slow-span lookup across all traces
-- Stores a secondary sort (latency_ms DESC) so the slowest-spans
-- query avoids a full table scan over the trace_id-sorted data.
-- ============================================================
ALTER TABLE agent_spans
    ADD PROJECTION IF NOT EXISTS proj_slow_spans
    (
        SELECT trace_id, span_id, tool_name, latency_ms, cost_usd, start_time
        ORDER BY (latency_ms DESC, trace_id)
    );

MATERIALIZE PROJECTION proj_slow_spans IN TABLE agent_spans;
