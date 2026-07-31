// SPDX-License-Identifier: MIT
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFixtureLog writes a 7-step recorded run across two trace_ids so
// FilterByTraceID has something to prove: only trace-a's events should
// be replayed even though trace-b's are interleaved in the same file.
func writeFixtureLog(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "run.jsonl")

	lines := []string{
		`{"seq_num":1,"trace_id":"trace-a","kind":"prompt","payload":{"text":"roll out config v9"}}`,
		`{"seq_num":1,"trace_id":"trace-b","kind":"prompt","payload":{"text":"unrelated run"}}`,
	}
	for i := 1; i <= 7; i++ {
		n := i*2 + 1
		lines = append(lines,
			`{"seq_num":`+itoa(n)+`,"trace_id":"trace-a","kind":"tool_call","tool_name":"deploy_shard","input_hash":"hash-`+itoa(i)+`","payload":{"shard":`+itoa(i)+`}}`,
			`{"seq_num":`+itoa(n+1)+`,"trace_id":"trace-a","kind":"tool_response","tool_name":"deploy_shard","input_hash":"hash-`+itoa(i)+`","payload":{"status":"ok"}}`,
		)
	}
	lines = append(lines, `{"seq_num":18,"trace_id":"trace-a","kind":"final_output","payload":{"text":"rolled out 7 shards"}}`)

	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func TestReplayRunsToCompletion(t *testing.T) {
	path := writeFixtureLog(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{"replay", "--log", path, "--trace-id", "trace-a"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "steps run: 7") {
		t.Errorf("stdout missing steps run: 7:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "output: rolled out 7 shards") {
		t.Errorf("stdout missing final output:\n%s", stdout.String())
	}
}

func TestReplayStopsAtStepSix(t *testing.T) {
	path := writeFixtureLog(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{"replay", "--log", path, "--trace-id", "trace-a", "--stop-at-step", "6"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "steps run: 6") {
		t.Errorf("stdout missing steps run: 6:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "stopped early") {
		t.Errorf("stdout missing stopped-early notice:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "output:") {
		t.Errorf("stdout should not report final output when stopped early:\n%s", stdout.String())
	}
}

func TestReplayInjectTimeoutFailsAtConfiguredStep(t *testing.T) {
	path := writeFixtureLog(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{"replay", "--log", path, "--trace-id", "trace-a", "--inject-timeout", "4"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout: %s", code, stdout.String())
	}
	if !strings.Contains(stderr.String(), "injected timeout") {
		t.Errorf("stderr missing injected-timeout message:\n%s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "steps run: 3") {
		t.Errorf("stdout missing steps run: 3 (steps before the injected step 4):\n%s", stdout.String())
	}
}

func TestReplayInjectTimeoutBeyondTraceLengthNeverFires(t *testing.T) {
	path := writeFixtureLog(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{"replay", "--log", path, "--trace-id", "trace-a", "--inject-timeout", "100"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "steps run: 7") {
		t.Errorf("stdout missing steps run: 7:\n%s", stdout.String())
	}
}

func TestReplayUnknownTraceIDFails(t *testing.T) {
	path := writeFixtureLog(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{"replay", "--log", path, "--trace-id", "trace-does-not-exist"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "no events found") {
		t.Errorf("stderr missing no-events message:\n%s", stderr.String())
	}
}

func TestReplayMissingRequiredFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"replay"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestReplayFiltersOtherTraceIDs(t *testing.T) {
	// trace-b only has a prompt event, no tool calls — if FilterByTraceID
	// ever leaked trace-a's calls into a trace-b replay this would run to
	// completion instead of erroring on an empty trace.
	path := writeFixtureLog(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{"replay", "--log", path, "--trace-id", "trace-b"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout: %s", code, stdout.String())
	}
	if !strings.Contains(stderr.String(), "no tool_call events") {
		t.Errorf("stderr missing empty-trace message:\n%s", stderr.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"bogus"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

// writeDiffFixtureLog writes two traces that agree on their first two
// tool calls (geocode, route_eta) and diverge on the third — rider-a's
// route takes the highway, rider-b's takes a surface street.
func writeDiffFixtureLog(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ab-run.jsonl")

	lines := []string{
		`{"seq_num":1,"span_id":"a-geo","trace_id":"rider-a","kind":"tool_call","tool_name":"geocode","input_hash":"hash-pickup","payload":{}}`,
		`{"seq_num":2,"span_id":"b-geo","trace_id":"rider-b","kind":"tool_call","tool_name":"geocode","input_hash":"hash-pickup","payload":{}}`,
		`{"seq_num":3,"span_id":"a-route1","trace_id":"rider-a","kind":"tool_call","tool_name":"route_eta","input_hash":"hash-route-1","payload":{}}`,
		`{"seq_num":4,"span_id":"b-route1","trace_id":"rider-b","kind":"tool_call","tool_name":"route_eta","input_hash":"hash-route-1","payload":{}}`,
		`{"seq_num":5,"span_id":"a-route2","trace_id":"rider-a","kind":"tool_call","tool_name":"route_eta","input_hash":"hash-highway","payload":{}}`,
		`{"seq_num":6,"span_id":"b-route2","trace_id":"rider-b","kind":"tool_call","tool_name":"route_eta","input_hash":"hash-surface-street","payload":{}}`,
	}

	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestDiffFindsFirstDivergingStep(t *testing.T) {
	path := writeDiffFixtureLog(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{"diff", "--log", path, "--trace-a", "rider-a", "--trace-b", "rider-b"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "first divergence at step 3") {
		t.Errorf("stdout missing step 3 divergence:\n%s", out)
	}
	if !strings.Contains(out, "reason: input_hash") {
		t.Errorf("stdout missing input_hash reason:\n%s", out)
	}
	if !strings.Contains(out, "a-route2") || !strings.Contains(out, "b-route2") {
		t.Errorf("stdout missing diverging span_ids:\n%s", out)
	}
}

func TestDiffNoDivergenceWhenTracesMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "match.jsonl")
	lines := []string{
		`{"seq_num":1,"trace_id":"t1","kind":"tool_call","tool_name":"geocode","input_hash":"hash-1","payload":{}}`,
		`{"seq_num":2,"trace_id":"t2","kind":"tool_call","tool_name":"geocode","input_hash":"hash-1","payload":{}}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{"diff", "--log", path, "--trace-a", "t1", "--trace-b", "t2"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no divergence") {
		t.Errorf("stdout missing no-divergence message:\n%s", stdout.String())
	}
}

func TestDiffUnknownTraceIDFails(t *testing.T) {
	path := writeDiffFixtureLog(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{"diff", "--log", path, "--trace-a", "rider-a", "--trace-b", "trace-does-not-exist"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "no events found") {
		t.Errorf("stderr missing no-events message:\n%s", stderr.String())
	}
}

func TestDiffMissingRequiredFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"diff", "--log", "somewhere.jsonl"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}
