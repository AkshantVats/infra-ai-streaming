// SPDX-License-Identifier: MIT

package fingerprint

import (
	"strings"
	"unicode"
)

// Rules is a tenant's optional normalization overrides, configurable via
// the admin API (Day 76, PUT /tenants/{id}/fingerprint-rules) and layered
// on top of Normalize's fixed §1 contract rather than replacing it — trim,
// whitespace-collapse, and canonical-JSON re-serialization still run on
// every request regardless of Rules. This preserves DESIGN.md §1's "one
// function, every call path" invariant for the base contract: Rules only
// changes what bytes reach Normalize, and a given tenant's Rules value
// changes those bytes the same way for every one of that tenant's
// requests, so two prompts that should still collide for that tenant do.
//
// The zero value is today's default behavior exactly: Rules{}.Apply(req)
// returns req unchanged, so a tenant that has never called the admin API
// fingerprints exactly as every tenant did before Day 76 shipped.
type Rules struct {
	StripPunctuation bool `json:"strip_punctuation"`
	Lowercase        bool `json:"lowercase"`
	// MaxPromptBytes caps each message's content length in bytes before
	// normalization. Zero means unbounded — the Day 70-75 default, where
	// no rule has ever truncated a prompt.
	MaxPromptBytes int `json:"max_prompt_bytes"`
}

// Apply returns req with r's overrides applied to each message's content,
// in a fixed order — strip punctuation, then lowercase, then truncate to
// MaxPromptBytes — so the same Rules value always produces the same
// output for the same input regardless of caller. Apply runs before
// Normalize, not instead of it: Apply changes what bytes go in: Normalize
// still owns canonicalizing them (whitespace collapse, key sorting).
func (r Rules) Apply(req PromptRequest) PromptRequest {
	if r == (Rules{}) {
		return req
	}

	out := PromptRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		Messages:    make([]Message, len(req.Messages)),
	}
	for i, m := range req.Messages {
		content := m.Content
		if r.StripPunctuation {
			content = stripPunctuation(content)
		}
		if r.Lowercase {
			content = strings.ToLower(content)
		}
		if r.MaxPromptBytes > 0 {
			content = truncateUTF8(content, r.MaxPromptBytes)
		}
		out.Messages[i] = Message{Role: m.Role, Content: content}
	}
	return out
}

// stripPunctuation removes every Unicode punctuation rune, collapsing
// "summarize this, please!" and "summarize this please" to the same
// text — the admin-configurable counterpart to Normalize's whitespace
// handling, for tenants whose duplicate traffic differs mainly in
// punctuation rather than whitespace.
func stripPunctuation(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsPunct(r) {
			return -1
		}
		return r
	}, s)
}

// truncateUTF8 cuts s to at most maxBytes bytes without splitting a
// multi-byte rune in half, which a naive s[:maxBytes] slice could do and
// would leave an invalid UTF-8 tail for Normalize's later JSON marshaling
// to choke on.
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	b := s[:maxBytes]
	for len(b) > 0 && !utf8ValidTail(b) {
		b = b[:len(b)-1]
	}
	return b
}

// utf8ValidTail reports whether b's final byte is not the start of a
// multi-byte rune sequence that got cut short — i.e. b is safe to use
// as-is. It only needs to check the trailing bytes since everything
// before them came from a valid UTF-8 string to begin with.
func utf8ValidTail(b string) bool {
	return strings.ToValidUTF8(b, "") == b
}
