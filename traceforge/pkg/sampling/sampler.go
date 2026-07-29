// SPDX-License-Identifier: MIT
// Package sampling implements head and tail sampling strategies for TraceForge.
// Head sampling keeps a random fraction of all spans; tail sampling ensures
// every error span is retained regardless of the head decision.
package sampling

import (
	"sync/atomic"

	"github.com/AkshantVats/infra-ai-streaming/traceforge/pkg/schema"
)

// Decision is the sampling verdict for a single span batch.
type Decision int

const (
	// Drop means the span should not be forwarded downstream.
	Drop Decision = iota
	// Keep means the span should be forwarded downstream.
	Keep
)

// Sampler decides whether to keep or drop a span.
type Sampler interface {
	Sample(s schema.Span) Decision
}

// HeadSampler keeps every span with probability Rate (0.0–1.0) regardless
// of the span's content. Errors are not given special treatment.
type HeadSampler struct {
	// Rate is the fraction of spans to keep (e.g. 0.10 = 10%).
	Rate float64
}

// Sample applies head sampling. The TraceID is used to make the decision
// deterministic per trace so all spans in a trace share the same fate.
func (h *HeadSampler) Sample(s schema.Span) Decision {
	if h.Rate <= 0 {
		return Drop
	}
	if h.Rate >= 1 {
		return Keep
	}
	// Use the TraceID's first 8 bytes for a trace-stable hash.
	// rand.Float64 on its own would split spans within the same trace;
	// hashing the ID keeps all spans for a trace together.
	hash := traceHash(s.TraceID)
	threshold := uint64(h.Rate * float64(^uint64(0)))
	if hash < threshold {
		return Keep
	}
	return Drop
}

// traceHash converts a TraceID string to a uint64 for threshold comparison.
// It is intentionally cheap and non-cryptographic.
func traceHash(traceID string) uint64 {
	var h uint64 = 14695981039346656037 // FNV-1a offset basis
	for i := 0; i < len(traceID); i++ {
		h ^= uint64(traceID[i])
		h *= 1099511628211 // FNV prime
	}
	return h
}

// ErrorTailSampler always keeps spans that represent errors, retries, or
// timeouts. Healthy spans are dropped. Use this alongside HeadSampler in
// a CombinedSampler so that error traces are never head-sampled away.
type ErrorTailSampler struct{}

// Sample keeps any span whose status signals a non-OK outcome.
func (e *ErrorTailSampler) Sample(s schema.Span) Decision {
	switch s.Status {
	case schema.StatusError, schema.StatusRetry, schema.StatusTimeout:
		return Keep
	default:
		return Drop
	}
}

// CombinedSampler implements the recommended production strategy:
//   - Head samples the happy path (ok / cancelled spans)
//   - Tail samples all error, retry, and timeout spans unconditionally
//
// Any sampler that returns Keep wins; all must return Drop to drop the span.
type CombinedSampler struct {
	Head Sampler
	Tail Sampler
}

// Sample returns Keep if either the head or tail sampler decides to keep the span.
func (c *CombinedSampler) Sample(s schema.Span) Decision {
	if c.Head.Sample(s) == Keep {
		return Keep
	}
	if c.Tail.Sample(s) == Keep {
		return Keep
	}
	return Drop
}

// Stats tracks cumulative sampling decisions for observability.
type Stats struct {
	kept    atomic.Uint64
	dropped atomic.Uint64
}

// Record updates counters based on a sampling decision.
func (st *Stats) Record(d Decision) {
	if d == Keep {
		st.kept.Add(1)
	} else {
		st.dropped.Add(1)
	}
}

// Kept returns the total number of spans kept.
func (st *Stats) Kept() uint64 { return st.kept.Load() }

// Dropped returns the total number of spans dropped.
func (st *Stats) Dropped() uint64 { return st.dropped.Load() }

// EffectiveRate returns the fraction of spans that were kept.
func (st *Stats) EffectiveRate() float64 {
	k := st.kept.Load()
	d := st.dropped.Load()
	total := k + d
	if total == 0 {
		return 0
	}
	return float64(k) / float64(total)
}

// FilterBatch applies sampler to a slice of spans and returns only those
// that the sampler decides to keep, updating stats along the way.
func FilterBatch(sampler Sampler, spans []schema.Span, stats *Stats) []schema.Span {
	out := make([]schema.Span, 0, len(spans))
	for _, s := range spans {
		d := sampler.Sample(s)
		if stats != nil {
			stats.Record(d)
		}
		if d == Keep {
			out = append(out, s)
		}
	}
	return out
}

// DefaultCombined returns a CombinedSampler configured for production:
// 10% head sampling + 100% error/retry/timeout tail sampling.
func DefaultCombined() *CombinedSampler {
	return &CombinedSampler{
		Head: &HeadSampler{Rate: 0.10},
		Tail: &ErrorTailSampler{},
	}
}
