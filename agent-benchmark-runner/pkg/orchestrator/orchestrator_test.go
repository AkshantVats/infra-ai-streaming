// SPDX-License-Identifier: MIT

package orchestrator

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/akshantvats/agent-benchmark-runner/pkg/criteria"
	"github.com/akshantvats/agent-benchmark-runner/pkg/task"
)

func sampleTask() task.Task {
	return task.Task{
		TaskID:         "checkout-happy-path",
		Seed:           42,
		Prompt:         "Complete a checkout.",
		TimeoutSeconds: 30,
		SuccessCriteria: []task.Criterion{
			{Type: task.MaxToolCalls, Max: 5},
		},
	}
}

func alwaysPassAgent(_ context.Context, _ task.Task, _ int64) (criteria.RunOutcome, error) {
	return criteria.RunOutcome{FinalOutput: "order confirmed", ToolCallSequence: []string{"check_inventory"}}, nil
}

func TestRun_ProducesOneResultPerRepetition(t *testing.T) {
	cfg := Config{Task: sampleTask(), AgentName: "agent-a", Repetitions: 7, MaxParallel: 3}

	results, err := Run(context.Background(), cfg, alwaysPassAgent)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(results) != 7 {
		t.Fatalf("expected 7 results, got %d", len(results))
	}
	for i, r := range results {
		if r.RepetitionIndex != i {
			t.Errorf("results[%d].RepetitionIndex = %d, want %d", i, r.RepetitionIndex, i)
		}
		if r.Err != nil {
			t.Errorf("results[%d].Err = %v, want nil", i, r.Err)
		}
		if !r.Passed {
			t.Errorf("results[%d].Passed = false, want true", i)
		}
		if r.StepCount != 1 {
			t.Errorf("results[%d].StepCount = %d, want 1", i, r.StepCount)
		}
	}
}

// TestRun_RespectsMaxParallel verifies that no more than cfg.MaxParallel
// repetitions are ever in flight simultaneously, by tracking a high-water
// mark of concurrent agentFn invocations.
func TestRun_RespectsMaxParallel(t *testing.T) {
	const maxParallel = 4
	var current, highWater int64

	blockingAgent := func(_ context.Context, _ task.Task, _ int64) (criteria.RunOutcome, error) {
		n := atomic.AddInt64(&current, 1)
		for {
			hw := atomic.LoadInt64(&highWater)
			if n <= hw || atomic.CompareAndSwapInt64(&highWater, hw, n) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		atomic.AddInt64(&current, -1)
		return criteria.RunOutcome{}, nil
	}

	cfg := Config{Task: sampleTask(), AgentName: "agent-a", Repetitions: 20, MaxParallel: maxParallel}
	if _, err := Run(context.Background(), cfg, blockingAgent); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := atomic.LoadInt64(&highWater); got > maxParallel {
		t.Fatalf("high-water mark of concurrent runs = %d, want <= %d", got, maxParallel)
	}
}

func TestDeriveSeed_DeterministicAndDistinctPerRepetition(t *testing.T) {
	const base = int64(42)

	first := make([]int64, 10)
	for i := range first {
		first[i] = deriveSeed(base, i)
	}

	// Reproducibility: deriving the same (base, i) again gives the same seed.
	for i := range first {
		if got := deriveSeed(base, i); got != first[i] {
			t.Errorf("deriveSeed(%d, %d) = %d on second call, want %d (same as first call)", base, i, got, first[i])
		}
	}

	// Distinctness: no two repetitions of the same base seed collide.
	seen := make(map[int64]int)
	for i, s := range first {
		if prev, ok := seen[s]; ok {
			t.Errorf("deriveSeed(%d, %d) collided with deriveSeed(%d, %d) = %d", base, i, base, prev, s)
		}
		seen[s] = i
	}

	// A different base seed produces a different stream of derived seeds.
	otherBase := deriveSeed(43, 0)
	if otherBase == first[0] {
		t.Errorf("deriveSeed(43, 0) = deriveSeed(42, 0) = %d, want different bases to diverge", otherBase)
	}
}

func TestRun_PartialFailure_OtherRepetitionsStillComplete(t *testing.T) {
	const failOn = 2

	var mu sync.Mutex
	seen := map[int]bool{}
	agent := func(_ context.Context, _ task.Task, _ int64) (criteria.RunOutcome, error) {
		mu.Lock()
		idx := len(seen)
		seen[idx] = true
		mu.Unlock()
		if idx == failOn {
			return criteria.RunOutcome{}, errors.New("simulated transport error")
		}
		return criteria.RunOutcome{FinalOutput: "order confirmed", ToolCallSequence: []string{"check_inventory"}}, nil
	}

	cfg := Config{Task: sampleTask(), AgentName: "agent-a", Repetitions: 5, MaxParallel: 1}
	results, err := Run(context.Background(), cfg, agent)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	errored := 0
	for _, r := range results {
		if r.Err != nil {
			errored++
		}
	}
	if errored != 1 {
		t.Fatalf("expected exactly 1 errored repetition, got %d", errored)
	}
}

func TestRun_ValidatesConfig(t *testing.T) {
	base := Config{Task: sampleTask(), AgentName: "agent-a", Repetitions: 1, MaxParallel: 1}

	cases := []struct {
		name string
		cfg  Config
		fn   AgentFunc
	}{
		{"zero repetitions", func() Config { c := base; c.Repetitions = 0; return c }(), alwaysPassAgent},
		{"negative repetitions", func() Config { c := base; c.Repetitions = -1; return c }(), alwaysPassAgent},
		{"zero max parallel", func() Config { c := base; c.MaxParallel = 0; return c }(), alwaysPassAgent},
		{"nil agentFn", base, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Run(context.Background(), tc.cfg, tc.fn); err == nil {
				t.Fatalf("Run(%s): expected error, got nil", tc.name)
			}
		})
	}
}

func TestRun_ContextCancellation_UnstartedRepetitionsGetCtxErr(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var started int64
	blockUntilCancelled := func(ctx context.Context, _ task.Task, _ int64) (criteria.RunOutcome, error) {
		atomic.AddInt64(&started, 1)
		<-ctx.Done()
		return criteria.RunOutcome{}, ctx.Err()
	}

	cfg := Config{Task: sampleTask(), AgentName: "agent-a", Repetitions: 50, MaxParallel: 2}

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	results, err := Run(ctx, cfg, blockUntilCancelled)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(results) != 50 {
		t.Fatalf("expected 50 results, got %d", len(results))
	}

	var withErr int
	for _, r := range results {
		if r.Err != nil {
			withErr++
		}
	}
	if withErr == 0 {
		t.Fatalf("expected at least one repetition to observe cancellation, got 0")
	}
}
