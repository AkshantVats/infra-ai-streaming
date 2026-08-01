// SPDX-License-Identifier: MIT

// Package orchestrator runs a single Task N times against an agent under a
// bounded concurrency budget and grades every repetition. A single run
// against a Task is an anecdote, not a benchmark — see DESIGN.md's "Running
// a Task N Times" section for why N repetitions and bounded parallelism are
// both required, not just one or the other.
package orchestrator

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"

	"github.com/akshantvats/agent-benchmark-runner/pkg/criteria"
	"github.com/akshantvats/agent-benchmark-runner/pkg/task"
)

// AgentFunc invokes the agent under test once, against t, using seed for
// whatever randomness the agent's own implementation draws on. It returns
// an error only when the run itself failed to complete (timeout, transport
// error) — a run that completes but fails its success criteria is not an
// AgentFunc error, it is graded normally and shows up as Passed == false.
type AgentFunc func(ctx context.Context, t task.Task, seed int64) (criteria.RunOutcome, error)

// Config controls one orchestrated batch: how many times to repeat Task
// against AgentName, and how much concurrency to allow.
type Config struct {
	Task        task.Task
	AgentName   string
	Repetitions int
	// MaxParallel bounds how many repetitions run at once. Unbounded
	// fan-out would turn a benchmark batch into a self-inflicted burst
	// against the agent's own downstream dependencies (model provider,
	// tools) — see DESIGN.md's "Why Bounded Parallelism".
	MaxParallel int
}

// RunResult is the outcome of one repetition: its derived seed, the graded
// criteria results, and how many tool calls it took — or Err set if the
// run itself did not complete.
type RunResult struct {
	RepetitionIndex int
	Seed            int64
	Outcome         criteria.RunOutcome
	Results         []criteria.Result
	Passed          bool
	StepCount       int
	Err             error
}

// Run executes cfg.Repetitions repetitions of cfg.Task against agentFn,
// running at most cfg.MaxParallel at a time, and returns one RunResult per
// repetition, indexed by RepetitionIndex regardless of completion order.
//
// Run itself only returns an error for a malformed Config; an individual
// repetition whose agentFn call errors is still represented in the
// returned slice with Err set, so a caller can distinguish "N-1 of N
// completed" from "the batch itself couldn't run at all."
func Run(ctx context.Context, cfg Config, agentFn AgentFunc) ([]RunResult, error) {
	if cfg.Repetitions <= 0 {
		return nil, fmt.Errorf("orchestrator: repetitions must be > 0, got %d", cfg.Repetitions)
	}
	if cfg.MaxParallel <= 0 {
		return nil, fmt.Errorf("orchestrator: max parallel must be > 0, got %d", cfg.MaxParallel)
	}
	if agentFn == nil {
		return nil, fmt.Errorf("orchestrator: agentFn is required")
	}

	results := make([]RunResult, cfg.Repetitions)
	sem := make(chan struct{}, cfg.MaxParallel)
	var wg sync.WaitGroup

	for i := 0; i < cfg.Repetitions; i++ {
		seed := deriveSeed(cfg.Task.Seed, i)

		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			results[i] = RunResult{RepetitionIndex: i, Seed: seed, Err: ctx.Err()}
			continue
		}

		wg.Add(1)
		go func(i int, seed int64) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = runOne(ctx, cfg, agentFn, i, seed)
		}(i, seed)
	}

	wg.Wait()
	return results, nil
}

func runOne(ctx context.Context, cfg Config, agentFn AgentFunc, i int, seed int64) RunResult {
	outcome, err := agentFn(ctx, cfg.Task, seed)
	if err != nil {
		return RunResult{RepetitionIndex: i, Seed: seed, Err: err}
	}

	graded := criteria.EvaluateAll(cfg.Task.SuccessCriteria, outcome)
	return RunResult{
		RepetitionIndex: i,
		Seed:            seed,
		Outcome:         outcome,
		Results:         graded,
		Passed:          criteria.AllPassed(graded),
		StepCount:       len(outcome.ToolCallSequence),
	}
}

// deriveSeed produces a distinct, reproducible seed for repetition i from
// one shared base seed, so an N-repetition batch reproduces byte-for-byte
// from a single recorded base seed while no two repetitions draw from
// correlated streams. A simple base+i offset was rejected: several common
// PRNGs correlate poorly across seeds that differ by a small constant,
// which would bias the very run-to-run variance Day 52 exists to measure
// honestly. Hashing (base, i) decorrelates the derived seeds instead.
func deriveSeed(base int64, repetitionIndex int) int64 {
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%d:%d", base, repetitionIndex) // hash.Hash.Write never errors
	return int64(h.Sum64())
}
