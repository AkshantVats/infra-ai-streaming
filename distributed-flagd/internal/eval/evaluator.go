// SPDX-License-Identifier: MIT
package eval

import "hash/fnv"

// PercentageVariant is one bucket in a percentage-rollout flag.
// Weights are integers 0-100; they must sum to 100.
type PercentageVariant struct {
	Value  string
	Weight int
}

// EvaluatePercentage hashes flagName+":"+hashKey via FNV-1a and maps
// the result to a variant. Bucket space is 10000 so weight precision
// is 0.01%. Deterministic: same inputs always return the same variant.
func EvaluatePercentage(flagName, hashKey string, variants []PercentageVariant) string {
	h := fnv.New32a()
	h.Write([]byte(flagName + ":" + hashKey))
	bucket := int(h.Sum32() % 10000)
	cumulative := 0
	for _, v := range variants {
		cumulative += v.Weight * 100
		if bucket < cumulative {
			return v.Value
		}
	}
	if len(variants) > 0 {
		return variants[len(variants)-1].Value
	}
	return ""
}
