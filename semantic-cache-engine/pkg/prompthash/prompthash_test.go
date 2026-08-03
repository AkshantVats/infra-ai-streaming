// SPDX-License-Identifier: MIT

package prompthash

import "testing"

func TestHashStableForIdenticalInput(t *testing.T) {
	a := Hash("summarize this document")
	b := Hash("summarize this document")
	if a != b {
		t.Fatalf("Hash is not deterministic: %q != %q", a, b)
	}
}

func TestHashSameAcrossIncidentalWhitespaceAndCase(t *testing.T) {
	a := Hash("Summarize this doc")
	b := Hash("  summarize   this   doc  ")
	if a != b {
		t.Fatalf("Hash should ignore incidental whitespace/case: %q != %q", a, b)
	}
}

func TestHashDiffersForDifferentPrompts(t *testing.T) {
	a := Hash("summarize this document")
	b := Hash("translate this document")
	if a == b {
		t.Fatalf("different prompts hashed to the same value: %q", a)
	}
}

func TestHashIsHexSHA256Length(t *testing.T) {
	h := Hash("anything")
	if len(h) != 64 {
		t.Fatalf("expected 64 hex chars (sha256), got %d: %q", len(h), h)
	}
}
