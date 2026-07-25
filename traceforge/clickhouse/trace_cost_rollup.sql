-- SPDX-License-Identifier: MIT
-- TraceForge: per-trace cost materialized view
-- Updates in near real-time as spans arrive from Kafka → agent_spans

-- ============================================================
-- Destination table: one row per trace
-- AggregatingMergeTree merges partial aggregates on background.
-- ============================================================
CREATE TABLE IF NOT EXISTS trace_cost_rollup
(
    trace_id          String,
    first_span_time   SimpleAggregateFunction(min, DateTime64(3, 'UTC')),
    last_span_time    SimpleAggregateFunction(max, DateTime64(3, 'UTC')),
    span_count        SimpleAggregateFunction(sum, UInt64),
    total_tokens      SimpleAggregateFunction(sum, UInt64),
    cost_usd          SimpleAggregateFunction(sum, Float64),
    error_count       SimpleAggregateFunction(sum, UInt64)
)
ENGINE = AggregatingMergeTree()
ORDER BY trace_id;

-- ============================================================
-- Materialized view: fires on every INSERT into agent_spans.
-- Groups by trace_id and accumulates partial aggregates.
-- ============================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS trace_cost_rollup_mv
TO trace_cost_rollup
AS SELECT
    trace_id,
    min(start_time)           AS first_span_time,
    max(start_time)           AS last_span_time,
    count()                   AS span_count,
    sum(total_tokens)         AS total_tokens,
    sum(cost_usd)             AS cost_usd,
    countIf(status = 'ERROR') AS error_count
FROM agent_spans
GROUP BY trace_id;
