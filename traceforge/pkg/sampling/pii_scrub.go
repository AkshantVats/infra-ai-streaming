// SPDX-License-Identifier: MIT
// Package sampling — PII scrubber processor for TraceForge spans.
// Redacts email addresses, phone numbers, and credit card patterns found in
// span Metadata before spans are forwarded downstream or stored.
package sampling

import (
	"regexp"

	"github.com/AkshantVats/infra-ai-streaming/traceforge/pkg/schema"
)

// redactionMark replaces matched PII in span metadata.
const redactionMark = "[REDACTED]"

var (
	// emailRe matches RFC 5322-ish email addresses.
	emailRe = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

	// phoneRe matches common phone formats: +1-800-555-1234, (800) 555 1234, etc.
	phoneRe = regexp.MustCompile(`(?:\+?\d{1,3}[\s\-.])?(?:\(?\d{3}\)?[\s\-.])\d{3}[\s\-.]\d{4}`)

	// cardRe matches 13–16 digit sequences that look like credit card numbers.
	// Requires groups of 4 digits optionally separated by spaces or dashes.
	cardRe = regexp.MustCompile(`\b(?:\d{4}[\s\-]){3}\d{4}\b|\b\d{13,16}\b`)
)

// PIIScrubber strips personally identifiable information from span metadata.
// It mutates a copy of each span so the original slice is left unchanged.
type PIIScrubber struct{}

// Scrub returns a copy of s with PII redacted from s.Metadata.
func (p *PIIScrubber) Scrub(s schema.Span) schema.Span {
	if s.Metadata == "" {
		return s
	}
	s.Metadata = redact(s.Metadata)
	return s
}

// ScrubBatch returns a new slice where each span's Metadata has been scrubbed.
func (p *PIIScrubber) ScrubBatch(spans []schema.Span) []schema.Span {
	out := make([]schema.Span, len(spans))
	for i, s := range spans {
		out[i] = p.Scrub(s)
	}
	return out
}

// redact applies all PII patterns to input and returns the sanitised string.
func redact(input string) string {
	s := emailRe.ReplaceAllString(input, redactionMark)
	s = phoneRe.ReplaceAllString(s, redactionMark)
	s = cardRe.ReplaceAllString(s, redactionMark)
	return s
}

// ScrubAndFilter is a convenience function that scrubs PII from spans and then
// applies the sampler, returning only kept spans. PII scrubbing happens first
// so that even dropped spans never touch downstream storage with raw PII.
func ScrubAndFilter(scrubber *PIIScrubber, sampler Sampler, spans []schema.Span, stats *Stats) []schema.Span {
	scrubbed := scrubber.ScrubBatch(spans)
	return FilterBatch(sampler, scrubbed, stats)
}
