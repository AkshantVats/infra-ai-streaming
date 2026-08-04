// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunAgainstMemStoreFallback(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--qps=200", "--duration=100ms", "--concurrency=16", "--dsn="}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "in-memory simulated store") {
		t.Fatalf("stdout = %q, want a note about the in-memory fallback", out)
	}
	if !strings.Contains(out, "requests=") || !strings.Contains(out, "p50=") {
		t.Fatalf("stdout = %q, want a requests= and p50= summary line", out)
	}
}

func TestRunRejectsInvalidFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--qps=0", "--duration=100ms"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run with --qps=0 exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "QPS") {
		t.Fatalf("stderr = %q, want a QPS validation error", stderr.String())
	}
}

func TestRunUnparseableFlagsReturnsUsageCode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--not-a-real-flag"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run with an unknown flag exit code = %d, want 2", code)
	}
}

func TestSyntheticEmbeddingHasEmbedderDimension(t *testing.T) {
	v := syntheticEmbedding()
	if len(v) != 1536 {
		t.Fatalf("syntheticEmbedding length = %d, want 1536", len(v))
	}
}

func TestRedactDSN(t *testing.T) {
	cases := []struct{ in, want string }{
		{"postgres://user:pass@localhost:5432/db", "postgres://user:***@localhost:5432/db"},
		{"postgres://localhost:5432/db", "postgres://localhost:5432/db"},
		{"not-a-dsn", "not-a-dsn"},
		{"postgres://user@localhost:5432/db", "postgres://user@localhost:5432/db"},
	}
	for _, tc := range cases {
		if got := redactDSN(tc.in); got != tc.want {
			t.Errorf("redactDSN(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
