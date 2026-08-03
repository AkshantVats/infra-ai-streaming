// SPDX-License-Identifier: MIT

// Package prompthash computes the sha256-of-normalized-text hash DESIGN.md
// §2 stores as semantic_cache_entries.prompt_hash -- the exact-dup fast
// path a lookup checks before falling back to the embedding similarity
// search (§1), and the idempotency key the embedding worker (pkg/worker)
// upserts on so re-embedding the same prompt twice is a no-op.
package prompthash

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Normalize collapses a prompt to the form Hash operates on: leading and
// trailing whitespace trimmed, internal runs of whitespace collapsed to a
// single space, and case folded. Two prompts that only differ in
// incidental whitespace or capitalization ("Summarize this doc" vs.
// "summarize   this doc") are the same request for the purposes of the
// exact-dup fast path, even though they are not byte-identical.
func Normalize(prompt string) string {
	fields := strings.Fields(prompt)
	return strings.ToLower(strings.Join(fields, " "))
}

// Hash returns the hex-encoded sha256 of Normalize(prompt), matching the
// prompt_hash column DESIGN.md §2 defines. It is deterministic and
// collision-resistant enough that two different prompts are never treated
// as the same cache entry, which is what makes prompt_hash safe as half of
// semantic_cache_entries' primary key alongside tenant_id.
func Hash(prompt string) string {
	sum := sha256.Sum256([]byte(Normalize(prompt)))
	return hex.EncodeToString(sum[:])
}
