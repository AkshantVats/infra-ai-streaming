-- SPDX-License-Identifier: MIT
-- tool_cost_rollup: total cost by vendor + model per minute, for cost attribution dashboards.
-- SummingMergeTree deduplicates partial sums on background merges.
CREATE TABLE IF NOT EXISTS tool_cost_rollup (
    window       DateTime,
    vendor       LowCardinality(String),
    model_name   LowCardinality(String),
    call_count   UInt64,
    cost_usd_sum Float64
) ENGINE = SummingMergeTree((call_count, cost_usd_sum))
PARTITION BY toYYYYMMDD(window)
ORDER BY (window, vendor, model_name)
TTL window + INTERVAL 90 DAY;

CREATE MATERIALIZED VIEW IF NOT EXISTS tool_cost_rollup_mv
TO tool_cost_rollup
AS SELECT
    toStartOfMinute(timestamp) AS window,
    vendor,
    model_name,
    count()                    AS call_count,
    sum(cost_usd)              AS cost_usd_sum
FROM tool_calls
GROUP BY window, vendor, model_name;
