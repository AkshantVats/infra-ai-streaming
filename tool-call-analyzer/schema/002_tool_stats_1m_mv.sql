-- SPDX-License-Identifier: MIT
-- tool_stats_1m: per-tool P99 latency, error rate, cost rollup -- 1-minute windows.
-- AggregatingMergeTree + quantileState gives a correct P99 merge across multiple inserts.
CREATE TABLE IF NOT EXISTS tool_stats_1m (
    window              DateTime,
    tool_name           LowCardinality(String),
    vendor               LowCardinality(String),
    model_name          LowCardinality(String),
    call_count          UInt64,
    error_count         UInt64,
    cost_usd_sum        Float64,
    latency_p99_state   AggregateFunction(quantile(0.99), UInt64)
) ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMMDD(window)
ORDER BY (window, tool_name, vendor, model_name)
TTL window + INTERVAL 90 DAY;

CREATE MATERIALIZED VIEW IF NOT EXISTS tool_stats_1m_mv
TO tool_stats_1m
AS SELECT
    toStartOfMinute(timestamp)       AS window,
    tool_name,
    vendor,
    model_name,
    count()                          AS call_count,
    countIf(has_error = 1)           AS error_count,
    sum(cost_usd)                    AS cost_usd_sum,
    quantileState(0.99)(duration_ms) AS latency_p99_state
FROM tool_calls
GROUP BY window, tool_name, vendor, model_name;

-- Query helper: use quantileMerge to read P99 back out.
-- SELECT window, tool_name, quantileMerge(0.99)(latency_p99_state) AS p99_ms
-- FROM tool_stats_1m GROUP BY window, tool_name ORDER BY window DESC;
