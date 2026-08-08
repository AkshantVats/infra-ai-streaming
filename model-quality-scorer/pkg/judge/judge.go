// SPDX-License-Identifier: MIT

// Package judge scores a sampled request/response pair against a
// JudgeRubric using a cheap judge model (Claude Haiku — DESIGN.md §1: the
// judge grades against a fixed rubric, a narrower task than generating
// the original response, so a smaller model graded consistently beats a
// larger one graded inconsistently). It owns the failure modes DESIGN.md
// §5 commits to: a fixed 5s timeout, one bounded retry, and a
// trailing-window circuit breaker per task_type.
package judge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/akshantvats/model-quality-scorer/pkg/rubric"
)

// Timeout is DESIGN.md §5's stated commitment: generous relative to
// Haiku's typical latency, bounded so one slow call can't stall a
// worker indefinitely.
const Timeout = 5 * time.Second

// ErrJudgeUnavailable is returned when both the initial call and its one
// bounded retry fail or time out. Callers route this to the DLQ tagged
// judge_unavailable (DESIGN.md §5) — it is a distinct outcome, not a
// score of zero and not a silently dropped sample.
var ErrJudgeUnavailable = errors.New("judge: unavailable after retry")

// ErrCircuitOpen is returned when the breaker has tripped for a
// task_type and Score refuses to spend a call on a judge that has
// already shown it isn't answering.
var ErrCircuitOpen = errors.New("judge: circuit open for task_type")

// Sample is one request/response pair selected for grading.
type Sample struct {
	TenantID      string
	TaskType      string
	ModelID       string
	RubricVersion int
	Prompt        string
	Response      string
}

// Result is a completed grading: the raw per-criterion scores plus the
// judge's short free-text justification (DESIGN.md §6 — rationale makes
// a low score debuggable, not just a bare integer).
type Result struct {
	Scores    map[string]float64
	Rationale string
}

// Caller sends one prompt to the judge model and returns its raw text
// response. Production wires this to the Haiku API; tests use a stub —
// no real network call happens in this package's own test suite.
type Caller interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

// judgeResponse is the JSON contract the judge prompt asks Haiku to
// reply in: one 0-10 score per criterion plus a short rationale.
type judgeResponse struct {
	Scores    map[string]float64 `json:"scores"`
	Rationale string             `json:"rationale"`
}

// HaikuJudge implements Score by calling Caller under DESIGN.md §5's
// timeout/retry/breaker policy.
type HaikuJudge struct {
	caller  Caller
	timeout time.Duration
	breaker *Breaker
}

// NewHaikuJudge constructs a HaikuJudge. breaker may be shared across
// multiple HaikuJudge instances (e.g. one per consumer worker) so the
// failure rate it tracks reflects the whole pool, not one worker's view.
func NewHaikuJudge(caller Caller, breaker *Breaker) *HaikuJudge {
	return &HaikuJudge{caller: caller, timeout: Timeout, breaker: breaker}
}

// Score grades sample against r. It never grades its own output: the
// judge model is fixed per deployment (Caller), never selected per
// sample's routing target (DESIGN.md §1).
func (j *HaikuJudge) Score(ctx context.Context, r rubric.JudgeRubric, s Sample) (Result, error) {
	if !j.breaker.Allow(s.TaskType) {
		return Result{}, ErrCircuitOpen
	}

	prompt := BuildPrompt(r, s)

	raw, err := j.callOnce(ctx, prompt)
	if err != nil {
		// One bounded retry, not a loop (DESIGN.md §5.1): a transient
		// blip gets one more chance, a genuinely down judge does not
		// get retried into a growing backlog.
		raw, err = j.callOnce(ctx, prompt)
	}
	if err != nil {
		j.breaker.Record(s.TaskType, false)
		return Result{}, fmt.Errorf("%w: %v", ErrJudgeUnavailable, err)
	}

	result, err := parseResponse(r, raw)
	if err != nil {
		j.breaker.Record(s.TaskType, false)
		return Result{}, fmt.Errorf("judge: parse response: %w", err)
	}

	j.breaker.Record(s.TaskType, true)
	return result, nil
}

func (j *HaikuJudge) callOnce(ctx context.Context, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, j.timeout)
	defer cancel()
	return j.caller.Complete(ctx, prompt)
}

// BuildPrompt embeds each Criterion.Description verbatim and asks for a
// 0-10 score per criterion, per DESIGN.md §2's "JSON schema, not
// freeform grading instructions" contract.
func BuildPrompt(r rubric.JudgeRubric, s Sample) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are grading a %s response against a fixed rubric. ", s.TaskType)
	b.WriteString("Score each criterion from 0 to 10 based on its description, then give a short rationale.\n\n")
	fmt.Fprintf(&b, "Prompt:\n%s\n\nResponse:\n%s\n\n", s.Prompt, s.Response)
	b.WriteString("Criteria:\n")
	for _, c := range r.Criteria {
		fmt.Fprintf(&b, "- %s: %s\n", c.Name, c.Description)
	}
	b.WriteString("\nReply with exactly one JSON object: ")
	b.WriteString(`{"scores": {"<criterion_name>": <0-10>, ...}, "rationale": "<short justification>"}`)
	return b.String()
}

func parseResponse(r rubric.JudgeRubric, raw string) (Result, error) {
	var resp judgeResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return Result{}, fmt.Errorf("decode judge response: %w", err)
	}
	if _, err := r.WeightedScore(resp.Scores); err != nil {
		return Result{}, err
	}
	if resp.Rationale == "" {
		return Result{}, errors.New("judge response missing rationale")
	}
	return Result{Scores: resp.Scores, Rationale: resp.Rationale}, nil
}
