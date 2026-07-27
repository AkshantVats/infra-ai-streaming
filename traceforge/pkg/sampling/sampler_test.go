// SPDX-License-Identifier: MIT
package sampling_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/AkshantVats/infra-ai-streaming/traceforge/pkg/sampling"
	"github.com/AkshantVats/infra-ai-streaming/traceforge/pkg/schema"
)

// makeSpan creates a minimal test span with the given traceID and status.
func makeSpan(traceID, status string) schema.Span {
	return schema.Span{
		TraceID:   traceID,
		SpanID:    "span-" + traceID,
		ToolName:  "llm_call",
		Status:    status,
		LatencyMs: 100,
	}
}

func TestHeadSampler_Zero(t *testing.T) {
	s := &sampling.HeadSampler{Rate: 0.0}
	for i := 0; i < 100; i++ {
		span := makeSpan(fmt.Sprintf("trace-%d", i), schema.StatusOK)
		if s.Sample(span) != sampling.Drop {
			t.Fatal("rate=0 should always drop")
		}
	}
}

func TestHeadSampler_One(t *testing.T) {
	s := &sampling.HeadSampler{Rate: 1.0}
	for i := 0; i < 100; i++ {
		span := makeSpan(fmt.Sprintf("trace-%d", i), schema.StatusOK)
		if s.Sample(span) != sampling.Keep {
			t.Fatal("rate=1.0 should always keep")
		}
	}
}

func TestHeadSampler_Rate_Approximate(t *testing.T) {
	// At 10% rate over 10,000 unique trace IDs, the kept fraction
	// should be within 3% of 10% (3-sigma tolerance for FNV-hashed IDs).
	s := &sampling.HeadSampler{Rate: 0.10}
	const n = 10_000
	kept := 0
	for i := 0; i < n; i++ {
		span := makeSpan(fmt.Sprintf("trace-%05d-unique", i), schema.StatusOK)
		if s.Sample(span) == sampling.Keep {
			kept++
		}
	}
	rate := float64(kept) / float64(n)
	if math.Abs(rate-0.10) > 0.03 {
		t.Errorf("head sampling rate %.4f deviates from 0.10 by >3%%", rate)
	}
}

func TestHeadSampler_TraceDeterminism(t *testing.T) {
	// Same trace ID must always produce the same decision.
	s := &sampling.HeadSampler{Rate: 0.50}
	span := makeSpan("stable-trace-id-xyz", schema.StatusOK)
	first := s.Sample(span)
	for i := 0; i < 20; i++ {
		if s.Sample(span) != first {
			t.Fatal("head sampler must be deterministic for a given trace ID")
		}
	}
}

func TestErrorTailSampler_KeepsErrors(t *testing.T) {
	s := &sampling.ErrorTailSampler{}
	errorStatuses := []string{schema.StatusError, schema.StatusRetry, schema.StatusTimeout}
	for _, st := range errorStatuses {
		span := makeSpan("err-trace", st)
		if s.Sample(span) != sampling.Keep {
			t.Errorf("tail sampler should keep status %q", st)
		}
	}
}

func TestErrorTailSampler_DropsOK(t *testing.T) {
	s := &sampling.ErrorTailSampler{}
	okStatuses := []string{schema.StatusOK, schema.StatusCancelled}
	for _, st := range okStatuses {
		span := makeSpan("ok-trace", st)
		if s.Sample(span) != sampling.Drop {
			t.Errorf("tail sampler should drop status %q", st)
		}
	}
}

func TestCombinedSampler_ErrorAlwaysKept(t *testing.T) {
	// Even with head rate=0, errors must be kept by tail.
	combined := &sampling.CombinedSampler{
		Head: &sampling.HeadSampler{Rate: 0.0},
		Tail: &sampling.ErrorTailSampler{},
	}
	span := makeSpan("err-combined", schema.StatusError)
	if combined.Sample(span) != sampling.Keep {
		t.Fatal("combined sampler must keep errors even when head rate=0")
	}
}

func TestCombinedSampler_Default(t *testing.T) {
	combined := sampling.DefaultCombined()
	// Errors must always be kept.
	errSpan := makeSpan("err-trace-default", schema.StatusError)
	if combined.Sample(errSpan) != sampling.Keep {
		t.Fatal("DefaultCombined must always keep error spans")
	}
}

func TestFilterBatch(t *testing.T) {
	spans := []schema.Span{
		makeSpan("t1", schema.StatusOK),
		makeSpan("t2", schema.StatusError),
		makeSpan("t3", schema.StatusOK),
	}
	// Use ErrorTailSampler so only the error span is kept.
	var stats sampling.Stats
	kept := sampling.FilterBatch(&sampling.ErrorTailSampler{}, spans, &stats)
	if len(kept) != 1 || kept[0].Status != schema.StatusError {
		t.Fatalf("expected 1 error span, got %d", len(kept))
	}
	if stats.Kept() != 1 || stats.Dropped() != 2 {
		t.Fatalf("stats: kept=%d dropped=%d, want kept=1 dropped=2", stats.Kept(), stats.Dropped())
	}
}

func TestStats_EffectiveRate(t *testing.T) {
	var st sampling.Stats
	st.Record(sampling.Keep)
	st.Record(sampling.Keep)
	st.Record(sampling.Drop)
	st.Record(sampling.Drop)
	if st.EffectiveRate() != 0.5 {
		t.Fatalf("expected 0.5, got %.2f", st.EffectiveRate())
	}
}

func TestPIIScrubber_Email(t *testing.T) {
	s := &sampling.PIIScrubber{}
	span := makeSpan("pii-trace", schema.StatusOK)
	span.Metadata = `{"user":"alice@example.com","query":"show me results"}`
	scrubbed := s.Scrub(span)
	if contains(scrubbed.Metadata, "alice@example.com") {
		t.Fatal("email not redacted")
	}
	if !contains(scrubbed.Metadata, "[REDACTED]") {
		t.Fatal("redaction mark not present")
	}
}

func TestPIIScrubber_Phone(t *testing.T) {
	s := &sampling.PIIScrubber{}
	span := makeSpan("pii-phone", schema.StatusOK)
	span.Metadata = `{"contact":"call me at (800) 555-1234 please"}`
	scrubbed := s.Scrub(span)
	if contains(scrubbed.Metadata, "800") && contains(scrubbed.Metadata, "555") {
		t.Fatal("phone number not redacted")
	}
}

func TestPIIScrubber_CreditCard(t *testing.T) {
	s := &sampling.PIIScrubber{}
	span := makeSpan("pii-card", schema.StatusOK)
	span.Metadata = `{"payment":"4111 1111 1111 1111","amount":"99.99"}`
	scrubbed := s.Scrub(span)
	if contains(scrubbed.Metadata, "4111 1111 1111 1111") {
		t.Fatal("credit card number not redacted")
	}
}

func TestPIIScrubber_EmptyMetadata(t *testing.T) {
	s := &sampling.PIIScrubber{}
	span := makeSpan("empty-meta", schema.StatusOK)
	span.Metadata = ""
	scrubbed := s.Scrub(span)
	if scrubbed.Metadata != "" {
		t.Fatal("empty metadata should remain empty after scrub")
	}
}

func TestScrubAndFilter(t *testing.T) {
	spans := []schema.Span{
		{TraceID: "t1", SpanID: "s1", ToolName: "llm_call", Status: schema.StatusOK, Metadata: `{"email":"test@corp.com"}`},
		{TraceID: "t2", SpanID: "s2", ToolName: "llm_call", Status: schema.StatusError, Metadata: `{"email":"bad@corp.com"}`},
	}
	scrubber := &sampling.PIIScrubber{}
	// Keep only errors — the error span must have PII scrubbed.
	kept := sampling.ScrubAndFilter(scrubber, &sampling.ErrorTailSampler{}, spans, nil)
	if len(kept) != 1 {
		t.Fatalf("expected 1 span, got %d", len(kept))
	}
	if contains(kept[0].Metadata, "bad@corp.com") {
		t.Fatal("PII not scrubbed from kept error span")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
