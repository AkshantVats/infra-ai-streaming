// SPDX-License-Identifier: MIT

// Package subprocess implements orchestrator.AgentFunc by invoking the
// agent under test as an external command instead of a linked-in Go
// function. This is what makes agent-benchmark-runner containerizable
// and language-agnostic: the agent under test can be any executable that
// speaks the stdin/stdout JSON contract below, not just a Go package this
// module imports. See DESIGN.md's "Agent Under Test as a Subprocess".
package subprocess

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/akshantvats/agent-benchmark-runner/pkg/criteria"
	"github.com/akshantvats/agent-benchmark-runner/pkg/orchestrator"
	"github.com/akshantvats/agent-benchmark-runner/pkg/task"
)

// killDelay bounds how long Wait() keeps reading the command's stdout and
// stderr pipes after a kill signal. Without it, a timed-out "sh -c" whose
// script forked a grandchild that inherited those pipes (e.g. "sleep 5"
// running under a non-exec-optimizing shell) would keep the pipes open
// for as long as the grandchild runs, even though sh itself was already
// killed — see exec.Cmd.WaitDelay's doc comment for this exact scenario.
const killDelay = 5 * time.Second

// stdinPayload is what a subprocess agent receives on stdin: the task
// plus the seed for this specific repetition, so one command serves
// every repetition of a batch instead of needing per-repetition wiring.
type stdinPayload struct {
	Task task.Task `json:"task"`
	Seed int64     `json:"seed"`
}

// AgentFunc returns an orchestrator.AgentFunc that runs command through
// "sh -c" once per repetition: it writes a stdinPayload as JSON to the
// command's stdin and decodes a criteria.RunOutcome as JSON from its
// stdout. The command is killed if t.TimeoutSeconds elapses or ctx is
// cancelled first, whichever comes sooner.
//
// A run whose command exits non-zero or whose stdout does not decode as
// a RunOutcome is reported as an AgentFunc error (the run itself did not
// complete) — it is not graded as a failing run the way a completed
// outcome that fails success criteria is.
func AgentFunc(command string) orchestrator.AgentFunc {
	return func(ctx context.Context, t task.Task, seed int64) (criteria.RunOutcome, error) {
		timeout := time.Duration(t.TimeoutSeconds) * time.Second
		runCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		payload, err := json.Marshal(stdinPayload{Task: t, Seed: seed})
		if err != nil {
			return criteria.RunOutcome{}, fmt.Errorf("subprocess: marshal stdin: %w", err)
		}

		cmd := exec.CommandContext(runCtx, "sh", "-c", command)
		cmd.Stdin = bytes.NewReader(payload)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		// Run the command in its own process group and kill the whole
		// group (not just the "sh" PID) on cancellation, so a script that
		// forks its own children can't outlive the timeout.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		cmd.WaitDelay = killDelay

		if err := cmd.Run(); err != nil {
			return criteria.RunOutcome{}, fmt.Errorf("subprocess: %q: %w (stderr: %s)", command, err, stderr.String())
		}

		var outcome criteria.RunOutcome
		if err := json.Unmarshal(stdout.Bytes(), &outcome); err != nil {
			return criteria.RunOutcome{}, fmt.Errorf("subprocess: decode stdout as RunOutcome: %w (stdout: %s)", err, stdout.String())
		}
		return outcome, nil
	}
}
