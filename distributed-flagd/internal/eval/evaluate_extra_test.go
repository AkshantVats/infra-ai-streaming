// SPDX-License-Identifier: MIT
package eval

import (
	"testing"
)

// TestEvaluateBool verifies bool type parsing in Evaluate.
func TestEvaluateBool(t *testing.T) {
	fv := FlagValue{FlagName: "f", Type: "bool", ValueJSON: "true"}
	got := Evaluate(fv, "")
	v, ok := got.(bool)
	if !ok {
		t.Fatalf("expected bool, got %T", got)
	}
	if !v {
		t.Error("expected true")
	}
}

// TestEvaluateBoolFalse verifies false is parsed correctly.
func TestEvaluateBoolFalse(t *testing.T) {
	fv := FlagValue{FlagName: "f", Type: "bool", ValueJSON: "false"}
	got := Evaluate(fv, "")
	v, ok := got.(bool)
	if !ok || v {
		t.Errorf("expected false bool, got %v (%T)", got, got)
	}
}

// TestEvaluateString verifies string type parsing.
func TestEvaluateString(t *testing.T) {
	fv := FlagValue{FlagName: "f", Type: "string", ValueJSON: `"hello"`}
	got := Evaluate(fv, "")
	v, ok := got.(string)
	if !ok || v != "hello" {
		t.Errorf("expected string hello, got %v (%T)", got, got)
	}
}

// TestEvaluateInt verifies int64 type parsing.
func TestEvaluateInt(t *testing.T) {
	fv := FlagValue{FlagName: "f", Type: "int", ValueJSON: "42"}
	got := Evaluate(fv, "")
	v, ok := got.(int64)
	if !ok || v != 42 {
		t.Errorf("expected int64(42), got %v (%T)", got, got)
	}
}

// TestEvaluateFloat verifies float64 type parsing.
func TestEvaluateFloat(t *testing.T) {
	fv := FlagValue{FlagName: "f", Type: "float", ValueJSON: "3.14"}
	got := Evaluate(fv, "")
	v, ok := got.(float64)
	if !ok || v != 3.14 {
		t.Errorf("expected float64(3.14), got %v (%T)", got, got)
	}
}

// TestEvaluateUnknownTypeReturnsRaw verifies unknown types return the raw JSON string.
func TestEvaluateUnknownTypeReturnsRaw(t *testing.T) {
	fv := FlagValue{FlagName: "f", Type: "json", ValueJSON: `{"key":"val"}`}
	got := Evaluate(fv, "")
	s, ok := got.(string)
	if !ok || s != `{"key":"val"}` {
		t.Errorf("expected raw string, got %v (%T)", got, got)
	}
}

// TestEvaluateInvalidJSONReturnsNil verifies that bad JSON for typed fields returns nil.
func TestEvaluateInvalidJSONReturnsNil(t *testing.T) {
	cases := []struct {
		typ string
		val string
	}{
		{"bool", "notabool"},
		{"string", "notastring"},
		{"int", "notanint"},
		{"float", "notafloat"},
	}
	for _, tc := range cases {
		fv := FlagValue{FlagName: "f", Type: tc.typ, ValueJSON: tc.val}
		got := Evaluate(fv, "key")
		if got != nil {
			t.Errorf("type=%s invalid JSON: expected nil, got %v (%T)", tc.typ, got, got)
		}
	}
}

// TestEvaluatePercentageEmptyVariants verifies empty slice returns "".
func TestEvaluatePercentageEmptyVariants(t *testing.T) {
	got := EvaluatePercentage("flag", "user", nil)
	if got != "" {
		t.Errorf("expected empty string for empty variants, got %q", got)
	}
}

// TestEvaluatePercentageSingleVariant verifies a single 100-weight variant is always returned.
func TestEvaluatePercentageSingleVariant(t *testing.T) {
	variants := []PercentageVariant{{Value: "only", Weight: 100}}
	for _, key := range []string{"a", "b", "c", "user-1234", "tenant:user"} {
		got := EvaluatePercentage("flag", key, variants)
		if got != "only" {
			t.Errorf("key=%q: expected only, got %q", key, got)
		}
	}
}

// TestEvaluatePercentageZeroWeightFallback verifies that if all weights are 0
// the last variant is returned as fallback.
func TestEvaluatePercentageZeroWeightFallback(t *testing.T) {
	variants := []PercentageVariant{
		{Value: "first", Weight: 0},
		{Value: "last", Weight: 0},
	}
	// With zero weights, cumulative never exceeds bucket so last variant is the fallback.
	got := EvaluatePercentage("flag", "any-key", variants)
	if got != "last" {
		t.Errorf("expected last (fallback), got %q", got)
	}
}
