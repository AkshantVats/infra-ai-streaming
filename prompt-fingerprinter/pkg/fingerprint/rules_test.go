// SPDX-License-Identifier: MIT

package fingerprint

import "testing"

func TestRules_ZeroValueIsNoOp(t *testing.T) {
	req := PromptRequest{
		Messages: []Message{{Role: "user", Content: "Hello, World!  "}},
		Model:    "gpt-4o",
	}
	got := Rules{}.Apply(req)
	if got.Messages[0].Content != req.Messages[0].Content {
		t.Errorf("zero-value Rules changed content: got %q, want unchanged %q", got.Messages[0].Content, req.Messages[0].Content)
	}
}

func TestRules_ZeroValueFingerprintMatchesBaseline(t *testing.T) {
	req := PromptRequest{Messages: []Message{{Role: "user", Content: "Summarize this."}}, Model: "gpt-4o"}
	if got, want := Fingerprint(Rules{}.Apply(req)), Fingerprint(req); got != want {
		t.Errorf("Rules{}.Apply must not change the fingerprint: got %s, want %s", got, want)
	}
}

func TestRules_StripPunctuation(t *testing.T) {
	req := PromptRequest{Messages: []Message{{Role: "user", Content: "summarize this, please!"}}, Model: "gpt-4o"}
	r := Rules{StripPunctuation: true}
	got := r.Apply(req).Messages[0].Content
	want := "summarize this please"
	if got != want {
		t.Errorf("StripPunctuation: got %q, want %q", got, want)
	}
}

func TestRules_Lowercase(t *testing.T) {
	req := PromptRequest{Messages: []Message{{Role: "user", Content: "Summarize THIS Document"}}, Model: "gpt-4o"}
	r := Rules{Lowercase: true}
	got := r.Apply(req).Messages[0].Content
	want := "summarize this document"
	if got != want {
		t.Errorf("Lowercase: got %q, want %q", got, want)
	}
}

func TestRules_MaxPromptBytesTruncates(t *testing.T) {
	req := PromptRequest{Messages: []Message{{Role: "user", Content: "abcdefghij"}}, Model: "gpt-4o"}
	r := Rules{MaxPromptBytes: 5}
	got := r.Apply(req).Messages[0].Content
	if len(got) != 5 || got != "abcde" {
		t.Errorf("MaxPromptBytes=5: got %q (len %d), want %q (len 5)", got, len(got), "abcde")
	}
}

func TestRules_MaxPromptBytesZeroIsUnbounded(t *testing.T) {
	long := "this content should never be truncated because MaxPromptBytes is zero, meaning unbounded, the Day 70-75 default"
	req := PromptRequest{Messages: []Message{{Role: "user", Content: long}}, Model: "gpt-4o"}
	got := Rules{MaxPromptBytes: 0}.Apply(req).Messages[0].Content
	if got != long {
		t.Errorf("MaxPromptBytes=0 must not truncate: got %q", got)
	}
}

func TestRules_MaxPromptBytesRespectsUTF8Boundary(t *testing.T) {
	// "café" is c-a-f-é where é is 2 bytes in UTF-8 (0xC3 0xA9), so byte
	// index 4 lands mid-rune. Truncating there must back off to a valid
	// boundary rather than emit an invalid tail.
	req := PromptRequest{Messages: []Message{{Role: "user", Content: "café"}}, Model: "gpt-4o"}
	got := Rules{MaxPromptBytes: 4}.Apply(req).Messages[0].Content
	if got != "caf" {
		t.Errorf("UTF-8 boundary truncation: got %q, want %q", got, "caf")
	}
}

func TestRules_OrderIsStripThenLowercaseThenTruncate(t *testing.T) {
	// "Hi, World!" — strip punctuation first ("Hi World"), then lowercase
	// ("hi world"), then truncate to 5 bytes ("hi wo"). If truncation ran
	// before stripping, the result would differ (punctuation would still
	// occupy some of the byte budget).
	req := PromptRequest{Messages: []Message{{Role: "user", Content: "Hi, World!"}}, Model: "gpt-4o"}
	r := Rules{StripPunctuation: true, Lowercase: true, MaxPromptBytes: 5}
	got := r.Apply(req).Messages[0].Content
	want := "hi wo"
	if got != want {
		t.Errorf("combined rule order: got %q, want %q", got, want)
	}
}

func TestRules_ExpandsDuplicateDetection(t *testing.T) {
	// The integration payoff: two prompts differing only in case and
	// punctuation fingerprint differently by default, but identically
	// once a tenant opts into StripPunctuation+Lowercase.
	a := PromptRequest{Messages: []Message{{Role: "user", Content: "Summarize this doc!"}}, Model: "gpt-4o"}
	b := PromptRequest{Messages: []Message{{Role: "user", Content: "summarize this doc"}}, Model: "gpt-4o"}

	if Fingerprint(a) == Fingerprint(b) {
		t.Fatal("precondition failed: a and b should NOT collide under default (zero-value) rules")
	}

	r := Rules{StripPunctuation: true, Lowercase: true}
	if got, want := Fingerprint(r.Apply(a)), Fingerprint(r.Apply(b)); got != want {
		t.Errorf("with StripPunctuation+Lowercase, a and b should collide: got %s, want %s", got, want)
	}
}
