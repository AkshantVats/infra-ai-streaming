-- SPDX-License-Identifier: MIT
-- tool_duration_alerts: captures tool calls that consumed >40% of their trace's wall time.
-- WHERE guard on trace_duration_ms > 0 prevents division by zero.
CREATE TABLE IF NOT EXISTS tool_duration_alerts (
    timestamp         DateTime64(3, 'UTC'),
    trace_id          String,
    tool_id           String,
    tool_name         LowCardinality(String),
    vendor            LowCardinality(String),
    duration_ms       UInt64,
    trace_duration_ms UInt64,
    pct_of_trace      Float64
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp, trace_id, tool_name)
TTL timestamp + INTERVAL 7 DAY;

CREATE MATERIALIZED VIEW IF NOT EXISTS tool_duration_alert_mv
TO tool_duration_alerts
AS SELECT
    timestamp,
    trace_id,
    tool_id,
    tool_name,
    vendor,
    duration_ms,
    trace_duration_ms,
    (toFloat64(duration_ms) / toFloat64(trace_duration_ms)) * 100.0 AS pct_of_trace
FROM tool_calls
WHERE trace_duration_ms > 0
  AND (toFloat64(duration_ms) / toFloat64(trace_duration_ms)) * 100.0 > 40.0;
