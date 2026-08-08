-- SPDX-License-Identifier: MIT
-- model-quality-scorer: one row per judged sample (DESIGN.md §6).
-- Aggregates are computed at query time by slicing tenant_id/task_type/
-- model_id — never pre-collapsed into a running average at write time,
-- the same lesson the P99-per-tenant work (Day 60, Day 77) already
-- applies to latency percentiles.

CREATE TABLE IF NOT EXISTS infra_ai.quality_scores
(
    tenant_id       LowCardinality(String)  COMMENT 'Tenant the judged sample belongs to',
    task_type       LowCardinality(String)  COMMENT 'Rubric key — summarization, extraction, etc.',
    model_id        LowCardinality(String)  COMMENT 'The model whose response was judged (not the judge model)',
    rubric_version  UInt16                  COMMENT 'JudgeRubric.Version the score was computed under',
    score           Float64                 COMMENT 'Weighted 0-100 score (rubric.WeightedScore output)',
    rationale       String                  COMMENT 'Judge''s short free-text justification for the score',
    scored_at       DateTime64(3)           COMMENT 'When the judge call completed'
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(scored_at)
ORDER BY (tenant_id, task_type, scored_at);
