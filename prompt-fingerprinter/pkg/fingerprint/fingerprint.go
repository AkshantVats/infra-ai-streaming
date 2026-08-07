// SPDX-License-Identifier: MIT

// Package fingerprint implements DESIGN.md's exact-match cache key:
// normalize a prompt request into canonical bytes (§1), hash those
// bytes with SHA-256 (§2), and scope the result to a tenant's Redis
// keyspace (§3). Normalize is the single call path both this module
// and semantic-cache-engine's embedding pipeline are meant to share —
// two independent normalizers would let the same prompt collide under
// one path and not the other, which DESIGN.md §1 calls out as a
// correctness bug, not a cosmetic one.
package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// Message is one turn in a prompt request, matching the shape
// semantic-cache-engine's embedding pipeline already consumes.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// PromptRequest is the request DESIGN.md §1 normalizes before hashing.
// Fields beyond Messages/Model/Temperature are intentionally omitted
// here — this type only carries what normalization and fingerprinting
// need, not the full gateway request envelope.
type PromptRequest struct {
	Messages    []Message `json:"messages"`
	Model       string    `json:"model"`
	Temperature float64   `json:"temperature"`
}

// collapseWhitespace trims leading/trailing whitespace and collapses
// every internal run of whitespace to a single space, per DESIGN.md
// §1 steps 1-2: "a retried request with different line-wrapping is
// still the same prompt."
func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// Normalize applies DESIGN.md §1's canonicalization contract and
// returns the exact bytes Fingerprint hashes. It is the only place
// this logic lives; callers must not re-derive normalized bytes any
// other way, or two "equivalent" prompts stop being guaranteed to
// collide.
//
// Step 1-2: trim and collapse whitespace on each message's content
// (the envelope's Model/Temperature fields are not free-text and are
// left as-is). Step 3: re-serialize as canonical JSON — decode into a
// map[string]any and let encoding/json's map marshaling sort keys
// lexicographically, rather than hand-rolling a serializer that could
// drift from Go's own canonical ordering.
func Normalize(req PromptRequest) []byte {
	cleaned := PromptRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		Messages:    make([]Message, len(req.Messages)),
	}
	for i, m := range req.Messages {
		cleaned.Messages[i] = Message{
			Role:    m.Role,
			Content: collapseWhitespace(m.Content),
		}
	}

	// Round-trip through map[string]any so struct field order never
	// leaks into the canonical form — only encoding/json's sorted-key
	// map marshaling decides byte order.
	raw, err := json.Marshal(cleaned)
	if err != nil {
		// PromptRequest contains only strings, a float64, and a slice
		// of the same — every value is JSON-marshalable by
		// construction, so this path is unreachable in practice.
		panic(fmt.Sprintf("fingerprint: marshal cleaned request: %v", err))
	}

	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		panic(fmt.Sprintf("fingerprint: unmarshal into canonical map: %v", err))
	}

	canonical, err := json.Marshal(generic)
	if err != nil {
		panic(fmt.Sprintf("fingerprint: marshal canonical map: %v", err))
	}
	return canonical
}

// Fingerprint returns the hex-encoded SHA-256 digest of req's
// normalized form (DESIGN.md §2). SHA-256 is chosen for collision
// resistance, not speed: a collision here would serve one tenant's
// cached response for a different prompt, so the hash's cost is paid
// once per request in exchange for that failure mode being
// cryptographically implausible rather than merely unlikely.
func Fingerprint(req PromptRequest) string {
	sum := sha256.Sum256(Normalize(req))
	return hex.EncodeToString(sum[:])
}

// RedisKey returns DESIGN.md §2/§3's tenant-scoped cache key. Scoping
// by tenant in the key itself — not just the value — means a
// fingerprint collision (already vanishingly unlikely under SHA-256)
// would stay contained to one tenant's keyspace rather than crossing
// the tenant boundary the rest of the stack treats as inviolable.
func RedisKey(tenantID, fp string) string {
	return fmt.Sprintf("fingerprint:%s:%s", tenantID, fp)
}
