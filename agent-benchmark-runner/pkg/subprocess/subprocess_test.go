// SPDX-License-Identifier: MIT
package subprocess

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/akshantvats/agent-benchmark-runner/pkg/task"
)

func testTask(timeoutSeconds int) task.Task {
	return task.Task{
		TaskID:         "t1",
		Seed:           42,
		Prompt:         "do the thing",
		TimeoutSeconds: timeoutSeconds,
		SuccessCriteria: []task.Criterion{
			{Type: task.FinalOutputContains, Value: "ok"},
		},
	}
}

func TestAgentFuncDecodesOutcome(t *testing.T) {
	fn := AgentFunc(`printf '%s' '{"final_output":"it worked ok","tool_call_sequence":["search","write"]}'`)

	outcome, err := fn(context.Background(), testTask(5), 1)
	if err != nil {
		t.Fatalf("AgentFunc() error = %v", err)
	}
	if outcome.FinalOutput != "it worked ok" {
		t.Errorf("FinalOutput = %q, want %q", outcome.FinalOutput, "it worked ok")
	}
	if len(outcome.ToolCallSequence) != 2 || outcome.ToolCallSequence[0] != "search" {
		t.Errorf("ToolCallSequence = %v, want [search write]", outcome.ToolCallSequence)
	}
}

func TestAgentFuncNonZeroExit(t *testing.T) {
	fn := AgentFunc(`exit 3`)

	_, err := fn(context.Background(), testTask(5), 1)
	if err == nil {
		t.Fatal("AgentFunc() error = nil, want non-nil for a failing command")
	}
}

func TestAgentFuncBadJSON(t *testing.T) {
	fn := AgentFunc(`printf 'not json'`)

	_, err := fn(context.Background(), testTask(5), 1)
	if err == nil {
		t.Fatal("AgentFunc() error = nil, want non-nil for undecodable stdout")
	}
	if !strings.Contains(err.Error(), "decode stdout") {
		t.Errorf("error = %v, want it to mention decode stdout", err)
	}
}

func TestAgentFuncTimeout(t *testing.T) {
	fn := AgentFunc(`sleep 5`)

	start := time.Now()
	_, err := fn(context.Background(), testTask(1), 1)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("AgentFunc() error = nil, want non-nil when the task timeout elapses")
	}
	if elapsed > 3*time.Second {
		t.Errorf("AgentFunc() took %v, want it killed well before the 5s sleep completes", elapsed)
	}
}

func TestAgentFuncReceivesStdinPayload(t *testing.T) {
	// cat echoes stdin back; assert the marshaled task_id and seed both
	// made it onto the subprocess's stdin, then produce valid JSON output
	// after that.
	fn := AgentFunc(`cat > /tmp/subprocess_test_stdin_$$.json; printf '%s' '{"final_output":"ok","tool_call_sequence":[]}'; grep -q '"task_id":"t1"' /tmp/subprocess_test_stdin_$$.json && grep -q '"seed":7' /tmp/subprocess_test_stdin_$$.json; rm -f /tmp/subprocess_test_stdin_$$.json`)

	_, err := fn(context.Background(), testTask(5), 7)
	if err != nil {
		t.Fatalf("AgentFunc() error = %v, want stdin payload to contain task_id and seed", err)
	}
}
