// SPDX-License-Identifier: MIT
package eval

import (
	"encoding/json"
	"hash/fnv"
)

// FlagValue is a typed flag value ready for evaluation.
type FlagValue struct {
	FlagName  string
	Type      string // "bool", "string", "int", "float"
	ValueJSON string // JSON-encoded scalar
}

// Evaluate parses the JSON value in fv and returns the typed Go value.
// requestKey is reserved for future percentage-rollout support.
func Evaluate(fv FlagValue, _ string) interface{} {
	switch fv.Type {
	case "bool":
		var v bool
		if err := json.Unmarshal([]byte(fv.ValueJSON), &v); err != nil {
			return nil
		}
		return v
	case "string":
		var v string
		if err := json.Unmarshal([]byte(fv.ValueJSON), &v); err != nil {
			return nil
		}
		return v
	case "int":
		var v int64
		if err := json.Unmarshal([]byte(fv.ValueJSON), &v); err != nil {
			return nil
		}
		return v
	case "float":
		var v float64
		if err := json.Unmarshal([]byte(fv.ValueJSON), &v); err != nil {
			return nil
		}
		return v
	default:
		return fv.ValueJSON
	}
}

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
