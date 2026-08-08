// SPDX-License-Identifier: MIT

package judge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/akshantvats/model-quality-scorer/pkg/rubric"
)

func testRubric() rubric.JudgeRubric {
	return rubric.JudgeRubric{
		TaskType: "summarization",
		Version:  1,
		Criteria: []rubric.Criterion{
			{Name: "factual_grounding", Weight: 0.6, Description: "grounded in source"},
			{Name: "conciseness", Weight: 0.4, Description: "shorter than source"},
		},
	}
}

func testSample() Sample {
	return Sample{
		TenantID:      "tenant-a",
		TaskType:      "summarization",
		ModelID:       "gpt-4o-mini",
		RubricVersion: 1,
		Prompt:        "summarize this article",
		Response:      "a short summary",
	}
}

type stubCaller struct {
	calls    int
	response string
	err      error
}

func (s *stubCaller) Complete(ctx context.Context, prompt string) (string, error) {
	s.calls++
	if s.err != nil {
		return "", s.err
	}
	return s.response, nil
}

func TestScore_success(t *testing.T) {
	caller := &stubCaller{response: `{"scores":{"factual_grounding":10,"conciseness":5},"rationale":"solid"}`}
	b := NewBreaker(NewFakeClock(time.Unix(0, 0)))
	j := NewHaikuJudge(caller, b)

	result, err := j.Score(context.Background(), testRubric(), testSample())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Rationale != "solid" {
		t.Fatalf("unexpected rationale: %q", result.Rationale)
	}
	if caller.calls != 1 {
		t.Fatalf("expected exactly 1 call on success, got %d", caller.calls)
	}
}

func TestScore_retriesOnceOnFailure(t *testing.T) {
	calls := 0
	caller := callerFunc(func(ctx context.Context, prompt string) (string, error) {
		calls++
		if calls == 1 {
			return "", errors.New("transient blip")
		}
		return `{"scores":{"factual_grounding":8,"conciseness":8},"rationale":"ok on retry"}`, nil
	})
	b := NewBreaker(NewFakeClock(time.Unix(0, 0)))
	j := NewHaikuJudge(caller, b)

	result, err := j.Score(context.Background(), testRubric(), testSample())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected exactly 2 calls (1 retry), got %d", calls)
	}
	if result.Rationale != "ok on retry" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestScore_failsAfterOneRetry_notALoop(t *testing.T) {
	caller := &stubCaller{err: errors.New("judge down")}
	b := NewBreaker(NewFakeClock(time.Unix(0, 0)))
	j := NewHaikuJudge(caller, b)

	_, err := j.Score(context.Background(), testRubric(), testSample())
	if !errors.Is(err, ErrJudgeUnavailable) {
		t.Fatalf("expected ErrJudgeUnavailable, got %v", err)
	}
	if caller.calls != 2 {
		t.Fatalf("expected exactly 2 calls total (1 initial + 1 bounded retry), got %d", caller.calls)
	}
}

func TestScore_malformedJSONResponseIsUnavailable(t *testing.T) {
	caller := &stubCaller{response: `not json at all`}
	b := NewBreaker(NewFakeClock(time.Unix(0, 0)))
	j := NewHaikuJudge(caller, b)

	_, err := j.Score(context.Background(), testRubric(), testSample())
	if err == nil {
		t.Fatal("expected error for malformed judge response")
	}
}

func TestScore_missingRationaleIsRejected(t *testing.T) {
	caller := &stubCaller{response: `{"scores":{"factual_grounding":10,"conciseness":5},"rationale":""}`}
	b := NewBreaker(NewFakeClock(time.Unix(0, 0)))
	j := NewHaikuJudge(caller, b)

	_, err := j.Score(context.Background(), testRubric(), testSample())
	if err == nil {
		t.Fatal("expected error for missing rationale")
	}
}

func TestScore_circuitOpenSkipsCall(t *testing.T) {
	caller := &stubCaller{response: `{"scores":{"factual_grounding":10,"conciseness":5},"rationale":"solid"}`}
	clock := NewFakeClock(time.Unix(0, 0))
	b := NewBreaker(clock)
	for i := 0; i < 10; i++ {
		b.Record("summarization", false)
	}
	j := NewHaikuJudge(caller, b)

	_, err := j.Score(context.Background(), testRubric(), testSample())
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	if caller.calls != 0 {
		t.Fatalf("expected no calls while circuit is open, got %d", caller.calls)
	}
}

func TestScore_neverAveragesUnavailableAsAScore(t *testing.T) {
	// DESIGN.md §5: a judge_unavailable sample must never be averaged
	// in as a passing or failing score — Score must return a zero
	// Result alongside the error so a caller can't accidentally treat
	// a partially-populated Result as real.
	caller := &stubCaller{err: errors.New("down")}
	b := NewBreaker(NewFakeClock(time.Unix(0, 0)))
	j := NewHaikuJudge(caller, b)

	result, err := j.Score(context.Background(), testRubric(), testSample())
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Scores != nil || result.Rationale != "" {
		t.Fatalf("expected zero-value Result on failure, got %+v", result)
	}
}

func TestBuildPrompt_embedsDescriptionsVerbatim(t *testing.T) {
	prompt := BuildPrompt(testRubric(), testSample())
	if !contains(prompt, "grounded in source") || !contains(prompt, "shorter than source") {
		t.Fatalf("expected criterion descriptions embedded verbatim, got: %s", prompt)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}

type callerFunc func(ctx context.Context, prompt string) (string, error)

func (f callerFunc) Complete(ctx context.Context, prompt string) (string, error) {
	return f(ctx, prompt)
}
