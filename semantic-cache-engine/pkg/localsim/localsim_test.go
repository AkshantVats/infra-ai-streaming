// SPDX-License-Identifier: MIT

package localsim

import (
	"math"
	"testing"
)

// floatTolerance absorbs floating-point summation error from map
// iteration order, which is intentionally randomized in Go -- two
// mathematically-equal cosine similarities can differ in the last bit or
// two depending on summation order.
const floatTolerance = 1e-9

func TestTokenCosineSimilarityIdenticalText(t *testing.T) {
	got := TokenCosineSimilarity("summarize this document", "summarize this document")
	if math.Abs(got-1.0) > floatTolerance {
		t.Errorf("similarity of identical text = %v, want 1.0", got)
	}
}

func TestTokenCosineSimilarityIsCaseAndPunctuationInsensitive(t *testing.T) {
	got := TokenCosineSimilarity("Summarize THIS document!", "summarize this document")
	if math.Abs(got-1.0) > floatTolerance {
		t.Errorf("similarity = %v, want 1.0 for case/punctuation-only difference", got)
	}
}

func TestTokenCosineSimilarityPartialOverlap(t *testing.T) {
	got := TokenCosineSimilarity("summarize this document for me", "summarize this document")
	if got <= 0 || got >= 1.0 {
		t.Errorf("similarity = %v, want strictly between 0 and 1 for partial overlap", got)
	}
}

func TestTokenCosineSimilarityNoOverlap(t *testing.T) {
	got := TokenCosineSimilarity("summarize this document", "translate that paragraph")
	if got != 0 {
		t.Errorf("similarity = %v, want 0 for disjoint vocabularies", got)
	}
}

func TestTokenCosineSimilarityEmptyText(t *testing.T) {
	if got := TokenCosineSimilarity("", "something"); got != 0 {
		t.Errorf("similarity with empty text = %v, want 0", got)
	}
	if got := TokenCosineSimilarity("something", ""); got != 0 {
		t.Errorf("similarity with empty text = %v, want 0", got)
	}
}
