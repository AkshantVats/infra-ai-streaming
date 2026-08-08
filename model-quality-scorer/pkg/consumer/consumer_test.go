// SPDX-License-Identifier: MIT

package consumer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/akshantvats/model-quality-scorer/pkg/dlq"
	"github.com/akshantvats/model-quality-scorer/pkg/judge"
	"github.com/akshantvats/model-quality-scorer/pkg/rubric"
)

// fakeReader feeds a fixed queue of messages to FetchMessage, blocking
// (respecting ctx) once the queue is drained — the same shape a real
// kafka.Reader has on an idle topic.
type fakeReader struct {
	mu        sync.Mutex
	queue     []kafka.Message
	committed []kafka.Message
}

func (r *fakeReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	r.mu.Lock()
	if len(r.queue) > 0 {
		msg := r.queue[0]
		r.queue = r.queue[1:]
		r.mu.Unlock()
		return msg, nil
	}
	r.mu.Unlock()
	<-ctx.Done()
	return kafka.Message{}, ctx.Err()
}

func (r *fakeReader) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.committed = append(r.committed, msgs...)
	return nil
}

func (r *fakeReader) committedCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.committed)
}

func TestRun_flushesOnBatchSize(t *testing.T) {
	origBatch, origFlush := BatchSize, FlushInterval
	BatchSize = 3
	FlushInterval = time.Hour // effectively disabled for this test
	defer func() { BatchSize, FlushInterval = origBatch, origFlush }()

	msg := marshal(t, validMessage())
	r := &fakeReader{queue: []kafka.Message{{Value: msg}, {Value: msg}, {Value: msg}}}

	j := &stubJudge{result: judge.Result{Scores: map[string]float64{"factual_grounding": 10, "conciseness": 10}, Rationale: "ok"}}
	w := &recordingWriter{}
	d := &recordingDLQ{}
	rubrics := NewMapRubricStore([]rubric.JudgeRubric{testRubric()})
	p := NewProcessor(rubrics, j, w, d)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, r, p) }()

	deadline := time.After(2 * time.Second)
	for r.committedCount() < 3 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for size-triggered flush")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	<-done

	if len(w.rows) != 3 {
		t.Fatalf("expected 3 rows written after size-triggered flush, got %d", len(w.rows))
	}
}

func TestRun_flushesOnInterval(t *testing.T) {
	origBatch, origFlush := BatchSize, FlushInterval
	BatchSize = 100
	FlushInterval = 30 * time.Millisecond
	defer func() { BatchSize, FlushInterval = origBatch, origFlush }()

	msg := marshal(t, validMessage())
	r := &fakeReader{queue: []kafka.Message{{Value: msg}}}

	j := &stubJudge{result: judge.Result{Scores: map[string]float64{"factual_grounding": 10, "conciseness": 10}, Rationale: "ok"}}
	w := &recordingWriter{}
	d := &recordingDLQ{}
	rubrics := NewMapRubricStore([]rubric.JudgeRubric{testRubric()})
	p := NewProcessor(rubrics, j, w, d)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, r, p) }()

	deadline := time.After(2 * time.Second)
	for r.committedCount() < 1 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for interval-triggered flush")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	<-done

	if len(w.rows) != 1 {
		t.Fatalf("expected 1 row written after interval-triggered flush, got %d", len(w.rows))
	}
}

func TestRun_flushErrorLeavesMessagesUncommitted(t *testing.T) {
	origBatch, origFlush := BatchSize, FlushInterval
	BatchSize = 100
	FlushInterval = 20 * time.Millisecond
	defer func() { BatchSize, FlushInterval = origBatch, origFlush }()

	msg := marshal(t, validMessage())
	r := &fakeReader{queue: []kafka.Message{{Value: msg}}}

	j := &stubJudge{result: judge.Result{Scores: map[string]float64{"factual_grounding": 10, "conciseness": 10}, Rationale: "ok"}}
	w := &recordingWriter{err: errors.New("clickhouse down")}
	d := &recordingDLQ{}
	rubrics := NewMapRubricStore([]rubric.JudgeRubric{testRubric()})
	p := NewProcessor(rubrics, j, w, d)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err := Run(ctx, r, p)
	if err == nil {
		t.Fatal("expected Run to return the flush error")
	}
	if r.committedCount() != 0 {
		t.Fatalf("expected 0 committed messages after a failed flush, got %d", r.committedCount())
	}
}

var _ dlq.Publisher = (*recordingDLQ)(nil)
