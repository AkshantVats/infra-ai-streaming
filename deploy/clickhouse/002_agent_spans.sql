-- SPDX-License-Identifier: MIT
-- TraceForge: agent execution spans.
-- One row per tool call / sub-agent invocation / model call in an agent run.
-- ORDER BY (started_at, trace_id, span_id) optimises time-range queries and
-- waterfall assembly. PARTITION BY month keeps hot partitions small.

CREATE DATABASE IF NOT EXISTS lensai;

CREATE TABLE IF NOT EXISTS lensai.agent_spans
(
    trace_id        String                  COMMENT 'Root identifier for one agent run (hex128)',
    span_id         String                  COMMENT 'Identifier for this step (hex64)',
    parent_span_id  String   DEFAULT ''     COMMENT 'Empty string for root spans',
    tool_name       String                  COMMENT 'e.g. "llm_call", "bash_exec", "web_search"',
    tool_category   LowCardinality(String)  COMMENT 'retrieval|execution|memory|generation|unknown',
    model           String   DEFAULT ''     COMMENT 'Model ID e.g. "claude-sonnet-4-6"; empty for non-LLM tools',
    tokens          UInt32   DEFAULT 0      COMMENT 'Total tokens (input + output) for LLM spans',
    cost_usd        Float64  DEFAULT 0.0   COMMENT 'Inferred USD cost for this span',
    status          LowCardinality(String)  COMMENT 'ok|error|retry|timeout|cancelled',
    latency_ms      UInt32                  COMMENT 'Wall-clock duration in milliseconds',
    error_message   String   DEFAULT '',
    agent_id        String   DEFAULT ''     COMMENT 'Agent class identifier',
    tenant_id       String   DEFAULT ''     COMMENT 'Tenant or workspace scope',
    pipeline_version LowCardinality(String) DEFAULT '1' COMMENT 'Pipeline version from OTel processor',
    started_at      DateTime64(3)           COMMENT 'Span start time, millisecond precision',
    ingested_at     DateTime DEFAULT now()  COMMENT 'Row insertion time'
)
ENGINE = MergeTree()
ORDER BY (started_at, trace_id, span_id)
PARTITION BY toYYYYMM(started_at)
SETTINGS index_granularity = 8192;

-- Per-trace cost rollup table (populated by materialised view below).
CREATE TABLE IF NOT EXISTS lensai.agent_trace_cost
(
    trace_id        String,
    started_at      DateTime64(3),
    total_tokens    UInt64   DEFAULT 0,
    total_cost_usd  Float64  DEFAULT 0.0,
    span_count      UInt32   DEFAULT 0
)
ENGINE = SummingMergeTree()
ORDER BY (started_at, trace_id);

-- Materialised view that populates the rollup on every insert.
CREATE MATERIALIZED VIEW IF NOT EXISTS lensai.agent_trace_cost_mv
TO lensai.agent_trace_cost
AS SELECT
    trace_id,
    min(started_at)   AS started_at,
    sum(tokens)       AS total_tokens,
    sum(cost_usd)     AS total_cost_usd,
    count()           AS span_count
FROM lensai.agent_spans
GROUP BY trace_id;
