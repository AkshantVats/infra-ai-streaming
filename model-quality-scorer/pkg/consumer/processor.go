// SPDX-License-Identifier: MIT

// Package consumer wires the judge-requests Kafka topic to the judge and
// store packages: it batches incoming samples, groups them by
// (task_type, rubric_version) so a batch shares one resolved rubric
// template, scores each sample, and flushes the results as one batched
// ClickHouse insert plus zero or more DLQ entries.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/akshantvats/model-quality-scorer/pkg/dlq"
	"github.com/akshantvats/model-quality-scorer/pkg/judge"
	"github.com/akshantvats/model-quality-scorer/pkg/normalize"
	"github.com/akshantvats/model-quality-scorer/pkg/rubric"
	"github.com/akshantvats/model-quality-scorer/pkg/store"
)

// SampleMessage is the judge-requests wire format.
type SampleMessage struct {
	TenantID      string `json:"tenant_id"`
	TaskType      string `json:"task_type"`
	ModelID       string `json:"model_id"`
	RubricVersion int    `json:"rubric_version"`
	Prompt        string `json:"prompt"`
	Response      string `json:"response"`
}

func (m SampleMessage) validate() error {
	if m.TenantID == "" {
		return errors.New("tenant_id is empty")
	}
	if m.TaskType == "" {
		return errors.New("task_type is empty")
	}
	if m.RubricVersion < 1 {
		return errors.New("rubric_version must be >= 1")
	}
	if m.Response == "" {
		return errors.New("response is empty")
	}
	return nil
}

// Judge grades a Sample against a rubric. judge.HaikuJudge implements
// this.
type Judge interface {
	Score(ctx context.Context, r rubric.JudgeRubric, s judge.Sample) (judge.Result, error)
}

// Processor is the per-batch pipeline: parse, resolve rubric, score,
// split into ClickHouse rows and DLQ entries.
type Processor struct {
	rubrics RubricStore
	judge   Judge
	writer  store.Writer
	dlq     dlq.Publisher
	now     func() time.Time
}

// NewProcessor builds a Processor from its four collaborators.
func NewProcessor(rubrics RubricStore, j Judge, writer store.Writer, publisher dlq.Publisher) *Processor {
	return &Processor{rubrics: rubrics, judge: j, writer: writer, dlq: publisher, now: time.Now}
}

// ProcessBatch runs every raw message in msgs through the pipeline and
// flushes the results: one ClickHouse WriteBatch call for every
// successfully scored sample, and one dlq.Publish call per sample that
// couldn't be scored. It returns the first flush error encountered, if
// any — per-message parse/score failures are not returned as errors,
// they are routed to the DLQ, since one malformed sample must never
// block the rest of the batch from being scored.
func (p *Processor) ProcessBatch(ctx context.Context, msgs [][]byte) error {
	var rows []store.ScoredSample
	var deadLetters []dlq.Entry

	for _, raw := range msgs {
		row, entry := p.processOne(ctx, raw)
		if entry != nil {
			deadLetters = append(deadLetters, *entry)
			continue
		}
		rows = append(rows, *row)
	}

	if len(rows) > 0 {
		if err := p.writer.WriteBatch(ctx, rows); err != nil {
			return fmt.Errorf("consumer: write batch of %d rows: %w", len(rows), err)
		}
	}
	for _, entry := range deadLetters {
		if err := p.dlq.Publish(ctx, entry); err != nil {
			return fmt.Errorf("consumer: publish dlq entry (reason=%s): %w", entry.Reason, err)
		}
	}
	return nil
}

func (p *Processor) processOne(ctx context.Context, raw []byte) (*store.ScoredSample, *dlq.Entry) {
	var msg SampleMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, &dlq.Entry{Reason: dlq.ReasonMalformedMessage, Detail: err.Error(), Payload: raw}
	}
	if err := msg.validate(); err != nil {
		return nil, &dlq.Entry{Reason: dlq.ReasonMalformedMessage, Detail: err.Error(), Payload: raw}
	}

	r, err := p.rubrics.Get(msg.TaskType, msg.RubricVersion)
	if err != nil {
		return nil, &dlq.Entry{Reason: dlq.ReasonMalformedRubric, Detail: err.Error(), Payload: raw}
	}

	sample := judge.Sample{
		TenantID:      msg.TenantID,
		TaskType:      msg.TaskType,
		ModelID:       msg.ModelID,
		RubricVersion: msg.RubricVersion,
		Prompt:        msg.Prompt,
		Response:      msg.Response,
	}
	result, err := p.judge.Score(ctx, r, sample)
	if err != nil {
		reason := dlq.ReasonJudgeUnavailable
		if errors.Is(err, judge.ErrCircuitOpen) {
			reason = dlq.ReasonCircuitOpen
		}
		return nil, &dlq.Entry{Reason: reason, Detail: err.Error(), Payload: raw}
	}

	score, err := r.WeightedScore(result.Scores)
	if err != nil {
		// The judge answered but its scores don't fit the rubric it was
		// given (e.g. one criterion out of [0,10]) — this is a judge
		// response bug, not a rubric-template bug, but it still can't
		// become a stored score, so it dead-letters the same way.
		return nil, &dlq.Entry{Reason: dlq.ReasonJudgeUnavailable, Detail: err.Error(), Payload: raw}
	}

	normalizedScore, err := normalize.Score(score)
	if err != nil {
		// Only reachable if WeightedScore's own [0,100] contract broke —
		// the same "judge produced something we can't trust" failure as
		// the WeightedScore error above, so it dead-letters the same way
		// rather than inventing a new DLQ reason for it.
		return nil, &dlq.Entry{Reason: dlq.ReasonJudgeUnavailable, Detail: err.Error(), Payload: raw}
	}

	return &store.ScoredSample{
		TenantID:        msg.TenantID,
		TaskType:        msg.TaskType,
		ModelID:         msg.ModelID,
		RubricVersion:   msg.RubricVersion,
		Score:           score,
		NormalizedScore: normalizedScore,
		Rationale:       result.Rationale,
		ScoredAt:        p.now(),
	}, nil
}
