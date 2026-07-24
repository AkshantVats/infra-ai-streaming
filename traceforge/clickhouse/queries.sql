-- SPDX-License-Identifier: MIT
-- TraceForge: reference queries for Grafana + ad-hoc debugging

-- Q1: Waterfall — all spans for a trace, ordered for timeline render
--   Primary key range scan: O(k) where k = spans in this trace.
SELECT
    span_id,
    parent_span_id,
    tool_name,
    tool_kind,
    status,
    start_time,
    latency_ms,
    total_tokens,
    cost_usd,
    error_message
FROM agent_spans
WHERE trace_id = {trace_id:String}
ORDER BY start_time ASC;

-- Q2: Recent traces — last N trace IDs with summary
SELECT
    trace_id,
    minMerge(first_span_time)   AS started_at,
    sumMerge(span_count)        AS spans,
    sumMerge(total_tokens)      AS tokens,
    sumMerge(cost_usd)          AS cost,
    sumMerge(error_count)       AS errors
FROM trace_cost_rollup
GROUP BY trace_id
ORDER BY started_at DESC
LIMIT {limit:UInt32};

-- Q3: Slowest spans across all traces (uses proj_slow_spans projection)
SELECT
    trace_id,
    span_id,
    tool_name,
    latency_ms,
    cost_usd
FROM agent_spans
ORDER BY latency_ms DESC
LIMIT 20;

-- Q4: Error rate by tool in the last hour
SELECT
    tool_name,
    countIf(status = 'ERROR')  AS errors,
    count()                    AS total,
    round(100.0 * errors / total, 2) AS error_pct
FROM agent_spans
WHERE start_time >= now() - INTERVAL 1 HOUR
GROUP BY tool_name
ORDER BY error_pct DESC;

-- Q5: Token spend by model in the last 24h
SELECT
    model,
    sum(total_tokens)  AS tokens,
    sum(cost_usd)      AS cost_usd
FROM agent_spans
WHERE start_time >= now() - INTERVAL 24 HOUR
  AND model != ''
GROUP BY model
ORDER BY cost_usd DESC;
