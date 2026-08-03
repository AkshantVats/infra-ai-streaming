// SPDX-License-Identifier: MIT

package cachestore

import "testing"

func TestVectorLiteralFormat(t *testing.T) {
	got := vectorLiteral([]float32{0.1, -0.25, 3})
	want := "[0.1,-0.25,3]"
	if got != want {
		t.Fatalf("vectorLiteral: got %q, want %q", got, want)
	}
}

func TestVectorLiteralEmpty(t *testing.T) {
	if got := vectorLiteral(nil); got != "[]" {
		t.Fatalf("vectorLiteral(nil): got %q, want %q", got, "[]")
	}
}
