// SPDX-License-Identifier: MIT

// Package rollup builds the query-time 1h/24h aggregation SQL for
// infra_ai.quality_scores. DESIGN.md §6 already committed
// model-quality-scorer to computing aggregates at query time by slicing
// tenant_id/task_type/model_id — never pre-collapsed into a running
// average at write time. This package holds to that commitment: there
// is no rollup table and no materialized view here, only parameterized
// SQL a caller (Grafana, or a future batch job) runs on demand.
package rollup

import "fmt"

// Window is a rollup bucket width.
type Window string

const (
	Window1h  Window = "1h"
	Window24h Window = "24h"
)

// bucketExpr returns the ClickHouse expression that buckets scored_at
// into w-wide windows.
func bucketExpr(w Window) (string, error) {
	switch w {
	case Window1h:
		return "toStartOfHour(scored_at)", nil
	case Window24h:
		return "toStartOfDay(scored_at)", nil
	default:
		return "", fmt.Errorf("rollup: unknown window %q", w)
	}
}

// Query returns the SQL that rolls infra_ai.quality_scores up into per
// (window, model_id, task_type) buckets: a sample count and the
// normalized-score mean and population stddev a caller combines with
// StandardError/LowConfidence to judge how much to trust that mean.
// It aggregates normalized_score, not the raw score column — score
// (0-100) mixes rubrics with different criteria counts and weight
// distributions, which is exactly what normalize.Score exists to make
// comparable before anything averages across a model_id×task_type
// boundary.
func Query(w Window) (string, error) {
	bucket, err := bucketExpr(w)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`SELECT
  %s AS window,
  model_id,
  task_type,
  count() AS sample_count,
  avg(normalized_score) AS avg_normalized_score,
  stddevPop(normalized_score) AS stddev_normalized_score
FROM infra_ai.quality_scores
WHERE $__timeFilter(scored_at)
GROUP BY window, model_id, task_type
ORDER BY window`, bucket), nil
}

// Row is one result row of Query.
type Row struct {
	ModelID               string
	TaskType              string
	SampleCount           int
	AvgNormalizedScore    float64
	StddevNormalizedScore float64
}

// StandardError returns r's standard error of the mean (StandardError,
// applied to this row's own stddev and sample count).
func (r Row) StandardError() float64 {
	return StandardError(r.StddevNormalizedScore, r.SampleCount)
}

// LowConfidence reports whether r sits below the statistical noise
// floor (LowConfidence, applied to this row's own sample count).
func (r Row) LowConfidence() bool {
	return LowConfidence(r.SampleCount)
}
