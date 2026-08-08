-- SPDX-License-Identifier: MIT
-- model-quality-scorer: add normalized_score to quality_scores (Day 79).
-- Still one row per judged sample, not a rollup table — DESIGN.md §6's
-- "never pre-collapsed into a running average at write time" commitment
-- covers aggregation, not an additive per-row column. score (0-100) mixes
-- rubrics with different criteria counts and weight distributions;
-- normalized_score (0-1, score/100) is the comparable unit a 1h/24h
-- rollup query aggregates instead (pkg/rollup.Query).

ALTER TABLE infra_ai.quality_scores
    ADD COLUMN IF NOT EXISTS normalized_score Float64
    COMMENT 'score/100 — comparable unit across rubrics with different weight distributions'
    AFTER score;
