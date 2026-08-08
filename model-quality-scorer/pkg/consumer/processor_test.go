// SPDX-License-Identifier: MIT

package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/akshantvats/model-quality-scorer/pkg/dlq"
	"github.com/akshantvats/model-quality-scorer/pkg/judge"
	"github.com/akshantvats/model-quality-scorer/pkg/rubric"
	"github.com/akshantvats/model-quality-scorer/pkg/store"
)

func testRubric() rubric.JudgeRubric {
	return rubric.JudgeRubric{
		TaskType: "summarization",
		Version:  1,
		Criteria: []rubric.Criterion{
			{Name: "factual_grounding", Weight: 0.6, Description: "grounded"},
			{Name: "conciseness", Weight: 0.4, Description: "short"},
		},
	}
}

type stubJudge struct {
	result judge.Result
	err    error
}

func (j *stubJudge) Score(ctx context.Context, r rubric.JudgeRubric, s judge.Sample) (judge.Result, error) {
	return j.result, j.err
}

type recordingWriter struct {
	rows []store.ScoredSample
	err  error
}

func (w *recordingWriter) WriteBatch(ctx context.Context, rows []store.ScoredSample) error {
	if w.err != nil {
		return w.err
	}
	w.rows = append(w.rows, rows...)
	return nil
}

type recordingDLQ struct {
	entries []dlq.Entry
	err     error
}

func (d *recordingDLQ) Publish(ctx context.Context, entry dlq.Entry) error {
	if d.err != nil {
		return d.err
	}
	d.entries = append(d.entries, entry)
	return nil
}

func validMessage() SampleMessage {
	return SampleMessage{
		TenantID:      "tenant-a",
		TaskType:      "summarization",
		ModelID:       "gpt-4o-mini",
		RubricVersion: 1,
		Prompt:        "summarize",
		Response:      "a summary",
	}
}

func marshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestProcessBatch_scoresAndWrites(t *testing.T) {
	j := &stubJudge{result: judge.Result{
		Scores:    map[string]float64{"factual_grounding": 10, "conciseness": 5},
		Rationale: "solid",
	}}
	w := &recordingWriter{}
	d := &recordingDLQ{}
	rubrics := NewMapRubricStore([]rubric.JudgeRubric{testRubric()})
	p := NewProcessor(rubrics, j, w, d)
	p.now = func() time.Time { return time.Unix(1000, 0) }

	err := p.ProcessBatch(context.Background(), [][]byte{marshal(t, validMessage())})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(w.rows) != 1 {
		t.Fatalf("expected 1 row written, got %d", len(w.rows))
	}
	if len(d.entries) != 0 {
		t.Fatalf("expected no DLQ entries, got %d", len(d.entries))
	}
	row := w.rows[0]
	if row.TenantID != "tenant-a" || row.Score != 80 { // 0.6*100 + 0.4*50 = 80
		t.Fatalf("unexpected row: %+v", row)
	}
	if row.NormalizedScore != 0.8 { // Score/100
		t.Fatalf("unexpected normalized score: %+v", row)
	}
}

func TestProcessBatch_malformedMessageGoesToDLQ(t *testing.T) {
	j := &stubJudge{}
	w := &recordingWriter{}
	d := &recordingDLQ{}
	rubrics := NewMapRubricStore([]rubric.JudgeRubric{testRubric()})
	p := NewProcessor(rubrics, j, w, d)

	err := p.ProcessBatch(context.Background(), [][]byte{[]byte(`not json`)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(w.rows) != 0 {
		t.Fatalf("expected no rows written, got %d", len(w.rows))
	}
	if len(d.entries) != 1 || d.entries[0].Reason != dlq.ReasonMalformedMessage {
		t.Fatalf("expected 1 malformed_message DLQ entry, got %+v", d.entries)
	}
}

func TestProcessBatch_missingRequiredFieldGoesToDLQ(t *testing.T) {
	j := &stubJudge{}
	w := &recordingWriter{}
	d := &recordingDLQ{}
	rubrics := NewMapRubricStore([]rubric.JudgeRubric{testRubric()})
	p := NewProcessor(rubrics, j, w, d)

	msg := validMessage()
	msg.TenantID = ""
	err := p.ProcessBatch(context.Background(), [][]byte{marshal(t, msg)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.entries) != 1 || d.entries[0].Reason != dlq.ReasonMalformedMessage {
		t.Fatalf("expected malformed_message DLQ entry, got %+v", d.entries)
	}
}

func TestProcessBatch_unknownRubricGoesToDLQAsMalformedRubric(t *testing.T) {
	j := &stubJudge{}
	w := &recordingWriter{}
	d := &recordingDLQ{}
	rubrics := NewMapRubricStore(nil) // empty store: every lookup fails
	p := NewProcessor(rubrics, j, w, d)

	err := p.ProcessBatch(context.Background(), [][]byte{marshal(t, validMessage())})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.entries) != 1 || d.entries[0].Reason != dlq.ReasonMalformedRubric {
		t.Fatalf("expected malformed_rubric DLQ entry, got %+v", d.entries)
	}
}

func TestProcessBatch_judgeUnavailableGoesToDLQ(t *testing.T) {
	j := &stubJudge{err: judge.ErrJudgeUnavailable}
	w := &recordingWriter{}
	d := &recordingDLQ{}
	rubrics := NewMapRubricStore([]rubric.JudgeRubric{testRubric()})
	p := NewProcessor(rubrics, j, w, d)

	err := p.ProcessBatch(context.Background(), [][]byte{marshal(t, validMessage())})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(w.rows) != 0 {
		t.Fatalf("expected no rows written when judge is unavailable, got %d", len(w.rows))
	}
	if len(d.entries) != 1 || d.entries[0].Reason != dlq.ReasonJudgeUnavailable {
		t.Fatalf("expected judge_unavailable DLQ entry, got %+v", d.entries)
	}
}

func TestProcessBatch_circuitOpenGoesToDLQWithDistinctReason(t *testing.T) {
	j := &stubJudge{err: judge.ErrCircuitOpen}
	w := &recordingWriter{}
	d := &recordingDLQ{}
	rubrics := NewMapRubricStore([]rubric.JudgeRubric{testRubric()})
	p := NewProcessor(rubrics, j, w, d)

	err := p.ProcessBatch(context.Background(), [][]byte{marshal(t, validMessage())})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.entries) != 1 || d.entries[0].Reason != dlq.ReasonCircuitOpen {
		t.Fatalf("expected circuit_open DLQ entry, got %+v", d.entries)
	}
}

func TestProcessBatch_mixedBatchWritesGoodRowsAndDLQsBadOnes(t *testing.T) {
	j := &stubJudge{result: judge.Result{
		Scores:    map[string]float64{"factual_grounding": 10, "conciseness": 10},
		Rationale: "great",
	}}
	w := &recordingWriter{}
	d := &recordingDLQ{}
	rubrics := NewMapRubricStore([]rubric.JudgeRubric{testRubric()})
	p := NewProcessor(rubrics, j, w, d)
	p.now = func() time.Time { return time.Unix(1000, 0) }

	good := marshal(t, validMessage())
	bad := []byte(`{malformed`)
	err := p.ProcessBatch(context.Background(), [][]byte{good, bad})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(w.rows) != 1 {
		t.Fatalf("expected 1 good row written despite bad sibling, got %d", len(w.rows))
	}
	if len(d.entries) != 1 {
		t.Fatalf("expected 1 DLQ entry for the bad sibling, got %d", len(d.entries))
	}
}

func TestProcessBatch_writerErrorPropagates(t *testing.T) {
	j := &stubJudge{result: judge.Result{
		Scores:    map[string]float64{"factual_grounding": 10, "conciseness": 10},
		Rationale: "great",
	}}
	w := &recordingWriter{err: errors.New("clickhouse down")}
	d := &recordingDLQ{}
	rubrics := NewMapRubricStore([]rubric.JudgeRubric{testRubric()})
	p := NewProcessor(rubrics, j, w, d)

	err := p.ProcessBatch(context.Background(), [][]byte{marshal(t, validMessage())})
	if err == nil {
		t.Fatal("expected writer error to propagate")
	}
}

func TestProcessBatch_emptyBatchIsANoop(t *testing.T) {
	j := &stubJudge{}
	w := &recordingWriter{}
	d := &recordingDLQ{}
	rubrics := NewMapRubricStore([]rubric.JudgeRubric{testRubric()})
	p := NewProcessor(rubrics, j, w, d)

	if err := p.ProcessBatch(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error on empty batch: %v", err)
	}
	if len(w.rows) != 0 || len(d.entries) != 0 {
		t.Fatal("expected no writes and no DLQ entries for an empty batch")
	}
}
