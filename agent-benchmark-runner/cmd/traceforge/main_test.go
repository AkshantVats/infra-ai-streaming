// SPDX-License-Identifier: MIT
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akshantvats/agent-benchmark-runner/pkg/report"
)

// passingAgentCmd ignores the task/seed on stdin and always reports the
// checkout-happy-path fixture's passing outcome.
const passingAgentCmd = `printf '%s' '{"final_output":"order confirmed","tool_call_sequence":["check_inventory","charge_payment"]}'`

// divergingAgentCmd stops one tool call short, so a two-agent run finds a
// divergence at step 1.
const divergingAgentCmd = `printf '%s' '{"final_output":"order confirmed","tool_call_sequence":["check_inventory"]}'`

func TestRunSingleAgentPasses(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"run",
		"--task", "../../testdata/checkout-happy-path.yaml",
		"--agent-a-cmd", passingAgentCmd,
		"--repetitions", "3",
		"--max-parallel", "2",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run() code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "3/3 passed") {
		t.Errorf("stdout missing pass count:\n%s", stdout.String())
	}
}

func TestRunMissingRequiredFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"run", "--task", "x.yaml"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("run() code = %d, want 2 for missing --agent-a-cmd", code)
	}
}

func TestRunTwoAgentsWritesDivergenceReport(t *testing.T) {
	var stdout, stderr bytes.Buffer
	outDir := t.TempDir()

	code := run([]string{
		"run",
		"--task", "../../testdata/checkout-happy-path.yaml",
		"--agent-a-name", "gpt",
		"--agent-a-cmd", passingAgentCmd,
		"--agent-b-name", "claude",
		"--agent-b-cmd", divergingAgentCmd,
		"--repetitions", "1",
		"--max-parallel", "1",
		"--out", outDir,
	}, &stdout, &stderr)

	// agent B fails max_tool_calls-independent criteria (tool_call_sequence),
	// so the overall exit code should be non-zero even though the CLI still
	// runs to completion and writes the comparison report.
	if code != 1 {
		t.Fatalf("run() code = %d, want 1 (agent B fails a criterion); stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "diverged at step 1") {
		t.Errorf("stdout missing divergence line:\n%s", stdout.String())
	}

	jsonPath := filepath.Join(outDir, "checkout-happy-path-report.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", jsonPath, err)
	}
	var rep report.Report
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatalf("Unmarshal report JSON: %v", err)
	}
	if rep.AgentA != "gpt" || rep.AgentB != "claude" {
		t.Errorf("report agents = %s/%s, want gpt/claude", rep.AgentA, rep.AgentB)
	}
	if rep.SequenceMatch {
		t.Error("report SequenceMatch = true, want false")
	}

	mdPath := filepath.Join(outDir, "checkout-happy-path-report.md")
	if _, err := os.Stat(mdPath); err != nil {
		t.Errorf("expected markdown report at %s: %v", mdPath, err)
	}
}

func TestRunUnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"bogus"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("run() code = %d, want 2 for unknown subcommand", code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Errorf("stderr missing usage text:\n%s", stderr.String())
	}
}
