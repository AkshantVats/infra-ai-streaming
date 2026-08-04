// SPDX-License-Identifier: MIT

// Package localsim is a local, deterministic, dependency-free stand-in for
// pkg/embedder's cosine similarity computation, used only by
// cmd/threshold-sweep. It exists because this sandbox's OPENAI_API_KEY is
// at its billing quota limit (confirmed by a live probe against
// api.openai.com/v1/embeddings: HTTP 429 insufficient_quota, the same
// constraint Day 62 hit for DALL-E cover generation), so
// pkg/embedder.OpenAIEmbedder cannot be exercised end-to-end here.
//
// This is explicitly NOT a substitute for real embeddings in the lookup
// path itself (pkg/lookup always uses pkg/embedder.Embedder) -- bag-of-
// words term-frequency cosine similarity captures lexical overlap, not
// semantic intent, so it systematically under-scores paraphrases that
// share no words ("how much did that cost" vs "what was the price") and
// over-scores lexically similar but intent-different prompts ("cancel my
// subscription" vs "how do I cancel my subscription", which do share
// almost every token). BENCHMARKS.md states this limitation plainly next
// to every number this package produces.
package localsim

import (
	"math"
	"strings"
	"unicode"
)

// tokenize lowercases s and splits it into words, treating any run of
// non-letter/non-digit runes as a separator -- simple enough to have no
// dependency, sufficient for the small held-out fixture this package's
// only caller (cmd/threshold-sweep) sweeps over.
func tokenize(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// termFreq builds a term-frequency map from tokens.
func termFreq(tokens []string) map[string]float64 {
	freq := make(map[string]float64, len(tokens))
	for _, tok := range tokens {
		freq[tok]++
	}
	return freq
}

// TokenCosineSimilarity returns the cosine similarity between a and b's
// term-frequency vectors over their tokenized text: dot(a, b) / (||a|| *
// ||b||). Returns 0 if either text tokenizes to nothing.
func TokenCosineSimilarity(a, b string) float64 {
	fa := termFreq(tokenize(a))
	fb := termFreq(tokenize(b))
	if len(fa) == 0 || len(fb) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for term, freqA := range fa {
		normA += freqA * freqA
		if freqB, ok := fb[term]; ok {
			dot += freqA * freqB
		}
	}
	for _, freqB := range fb {
		normB += freqB * freqB
	}

	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}
