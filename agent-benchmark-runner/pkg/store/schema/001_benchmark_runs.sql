-- SPDX-License-Identifier: MIT
-- benchmark_runs: one row per orchestrator.RunResult repetition, not one row
-- per batch. A pre-aggregated per-batch row would need to be recomputed
-- every time a later repetition's data changes the median/P95; a raw
-- per-repetition row lets any consumer (this package's Summarize, a
-- Grafana panel, an ad-hoc `SELECT quantile(0.95)(step_count)`) compute its
-- own statistic instead of trusting a pre-aggregated one. See DESIGN.md.
CREATE TABLE IF NOT EXISTS benchmark_runs (
    task_id          String,
    agent_name       LowCardinality(String),
    repetition_index UInt32,
    seed             Int64,
    passed           UInt8,
    step_count       UInt32,
    error_message    String,
    timestamp        DateTime64(3, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (task_id, agent_name, timestamp)
SETTINGS index_granularity = 8192;
