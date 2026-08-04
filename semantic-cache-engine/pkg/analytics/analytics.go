// SPDX-License-Identifier: MIT

// Package analytics turns the cache_hit / cache_miss / cache_feedback
// event stream (pkg/lensai, DESIGN.md §5 and §8's Day 63 notes) into the
// three numbers DESIGN.md §4 and Day 63's plan item ask for: hit rate,
// a false-positive rate proxy, and an estimated dollar amount saved. The
// arithmetic lives here, unit-tested against plain counts, because the
// counts themselves come from ClickHouse (deploy/grafana/provisioning/
// dashboards/semantic-cache-analytics.json queries infra_ai.inference_events
// directly) and there is no live ClickHouse in this sandbox to query --
// same gap already logged for Days 61-62's pgvector integration tests.
//
// The exported *Query constants are the literal ClickHouse SQL the
// Grafana dashboard's panels use, kept here as the documented source of
// truth so a panel edited directly in Grafana can be checked against what
// this package says the numbers mean, rather than drifting silently.
package analytics

import "fmt"

// HitRateQuery computes hit rate for a tenant and time range. cache_miss
// (DESIGN.md §8 / pkg/lensai.SourceCacheMiss) is the denominator's other
// half -- without it, a hit-rate query over cache_hit alone has no
// "attempted lookups" to divide by, and would trend toward 100% by
// construction since only successes would ever be in the table.
const HitRateQuery = `SELECT
  countIf(source = 'cache_hit') AS hits,
  countIf(source = 'cache_miss') AS misses,
  countIf(source = 'cache_hit') / countIf(source IN ('cache_hit', 'cache_miss')) AS hit_rate
FROM infra_ai.inference_events
WHERE tenant_id = '${tenant_id}' AND $__timeFilter(timestamp)`

// FalsePositiveProxyQuery computes the thumbs-down-per-hit ratio DESIGN.md
// §4 calls the false-positive rate's measurement path, using
// cache_feedback (pkg/lensai.SourceCacheFeedback) events as the numerator.
// It is a lower bound, not the true rate: it only counts hits a user
// noticed was wrong and bothered to flag through the thumbs-down webhook
// (pkg/feedback) -- see FalsePositiveRateProxy's doc comment for the same
// caveat applied to the Go-side computation.
const FalsePositiveProxyQuery = `SELECT
  countIf(source = 'cache_feedback' AND status = 'thumbs_down') AS thumbs_down,
  countIf(source = 'cache_hit') AS hits,
  countIf(source = 'cache_feedback' AND status = 'thumbs_down') / countIf(source = 'cache_hit') AS false_positive_proxy
FROM infra_ai.inference_events
WHERE tenant_id = '${tenant_id}' AND $__timeFilter(timestamp)`

// CostSavedQuery estimates dollars saved by the cache: each cache_hit
// event costs $0 (pkg/lensai.EmitCacheHit always sets cost_usd=0, since a
// hit performs no model call), so the query instead multiplies the hit
// count by the tenant's own average real-inference cost over the same
// window -- what each of those hits would have cost had it missed.
const CostSavedQuery = `SELECT
  (SELECT countIf(source = 'cache_hit') FROM infra_ai.inference_events
    WHERE tenant_id = '${tenant_id}' AND $__timeFilter(timestamp))
  *
  (SELECT avgIf(cost_usd, source = '' OR source = 'inference') FROM infra_ai.inference_events
    WHERE tenant_id = '${tenant_id}' AND $__timeFilter(timestamp) AND cost_usd > 0)
  AS estimated_cost_saved_usd`

// HitRate returns hits / (hits + misses) for a tenant, or 0 when there
// have been no lookups yet (0/0 would otherwise be NaN, which is a worse
// "no data" signal for a Grafana stat panel than a plain 0).
func HitRate(hits, misses int64) float64 {
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

// FalsePositiveRateProxy returns thumbsDown / hits, DESIGN.md §4's
// false-positive rate approximated by real user reports instead of the
// full sampled human/LLM-judge review pass. It is a lower bound on the
// true rate -- a wrong hit a user didn't notice, or noticed but didn't
// bother to flag, is invisible to this proxy -- so a caller presenting
// this number should label it as a floor, not a measured rate. Returns 0
// when hits is 0 (nothing to have a false-positive rate over).
func FalsePositiveRateProxy(thumbsDown, hits int64) float64 {
	if hits == 0 {
		return 0
	}
	return float64(thumbsDown) / float64(hits)
}

// EstimatedCostSaved returns hits * avgInferenceCostUSD: what those hits
// would have cost had each one missed and fallen through to a real
// inference at the tenant's own average per-call cost. avgInferenceCostUSD
// should come from the same tenant and time window as hits (CostSavedQuery
// computes both together) so the estimate reflects that tenant's actual
// model mix rather than a global average.
func EstimatedCostSaved(hits int64, avgInferenceCostUSD float64) float64 {
	return float64(hits) * avgInferenceCostUSD
}

// Summary is the three Day 63 numbers for one tenant and time window,
// bundled for a caller (e.g. a future CLI or API handler) that wants all
// three together instead of calling each function separately.
type Summary struct {
	Hits, Misses, ThumbsDown int64
	AvgInferenceCostUSD      float64
}

// String renders a Summary as a one-line human-readable report, e.g. for
// cmd/threshold-sweep-style CLI output or a log line.
func (s Summary) String() string {
	return fmt.Sprintf(
		"hit_rate=%.4f false_positive_proxy=%.4f estimated_cost_saved_usd=%.4f (hits=%d misses=%d thumbs_down=%d)",
		HitRate(s.Hits, s.Misses),
		FalsePositiveRateProxy(s.ThumbsDown, s.Hits),
		EstimatedCostSaved(s.Hits, s.AvgInferenceCostUSD),
		s.Hits, s.Misses, s.ThumbsDown,
	)
}
