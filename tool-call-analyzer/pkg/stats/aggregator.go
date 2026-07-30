// SPDX-License-Identifier: MIT
// Package stats provides pure-Go per-tool statistics computation.
// Mirrors the ClickHouse MV logic so unit tests can verify correctness
// without a running ClickHouse instance.
package stats

import (
	"sort"

	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
)

// CallRecord pairs a ToolCall with the outcome data that ToolCall itself doesn't carry
// (duration and error status live on the span, not the normalized payload) -- the same
// split used by clickhouse.Writer.Insert.
type CallRecord struct {
	Tool       types.ToolCall
	DurationMs uint64
	HasError   bool
}

// ToolStats holds aggregated statistics for a single (tool_name, vendor) group.
type ToolStats struct {
	ToolName   string
	Vendor     string
	CallCount  int
	ErrorCount int
	CostUSDSum float64
	P99Ms      uint64
}

// Aggregator computes per-tool statistics from batches of ToolCall records.
type Aggregator struct {
	// AlertThresholdPct is the tool-duration-as-percentage-of-trace that triggers an alert.
	// Default: 40.0 (40%). Mirrors the ClickHouse alert MV threshold.
	AlertThresholdPct float64
}

// New returns an Aggregator with a 40% alert threshold.
func New() *Aggregator {
	return &Aggregator{AlertThresholdPct: 40.0}
}

// Aggregate computes per-(tool_name, vendor) stats from records.
func (a *Aggregator) Aggregate(records []CallRecord) []ToolStats {
	type key struct{ name, vendor string }
	type bucket struct {
		stats     ToolStats
		latencies []uint64
	}

	buckets := make(map[key]*bucket)
	for _, rec := range records {
		k := key{rec.Tool.Name, rec.Tool.Vendor}
		b, ok := buckets[k]
		if !ok {
			b = &bucket{stats: ToolStats{ToolName: rec.Tool.Name, Vendor: rec.Tool.Vendor}}
			buckets[k] = b
		}
		b.stats.CallCount++
		b.stats.CostUSDSum += rec.Tool.Cost.CostUSD
		if rec.HasError {
			b.stats.ErrorCount++
		}
		b.latencies = append(b.latencies, rec.DurationMs)
	}

	result := make([]ToolStats, 0, len(buckets))
	for _, b := range buckets {
		b.stats.P99Ms = P99(b.latencies)
		result = append(result, b.stats)
	}
	return result
}

// P99 returns the 99th percentile of durations (milliseconds).
// Returns 0 for nil or empty slices. Does not mutate the input.
func P99(durations []uint64) uint64 {
	if len(durations) == 0 {
		return 0
	}
	sorted := make([]uint64, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * 0.99)
	return sorted[idx]
}

// IsAlertThreshold returns true when durationMs exceeds AlertThresholdPct percent
// of traceDurationMs. Returns false when traceDurationMs is zero (division guard).
func (a *Aggregator) IsAlertThreshold(durationMs, traceDurationMs uint64) bool {
	if traceDurationMs == 0 {
		return false
	}
	pct := (float64(durationMs) / float64(traceDurationMs)) * 100.0
	return pct > a.AlertThresholdPct
}
