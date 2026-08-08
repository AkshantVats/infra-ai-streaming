// SPDX-License-Identifier: MIT

// Package dlq publishes samples that could not be scored to
// judge-requests-dlq, tagged with why. DESIGN.md §5: a failed grading
// attempt is not a 0 and not silently dropped — it is a distinct outcome
// the aggregation layer has to know how to exclude explicitly.
package dlq

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
)

// Reason tags why a sample landed in the DLQ instead of quality_scores.
type Reason string

const (
	// ReasonMalformedMessage means the raw Kafka message wasn't valid
	// JSON or was missing required sample fields.
	ReasonMalformedMessage Reason = "malformed_message"
	// ReasonMalformedRubric means the sample named a task_type/version
	// whose rubric failed rubric.JudgeRubric.Validate.
	ReasonMalformedRubric Reason = "malformed_rubric"
	// ReasonJudgeUnavailable means the judge call failed its initial
	// attempt and its one bounded retry (DESIGN.md §5.2).
	ReasonJudgeUnavailable Reason = "judge_unavailable"
	// ReasonCircuitOpen means the breaker had already tripped for this
	// sample's task_type, so no judge call was attempted at all
	// (DESIGN.md §5.3).
	ReasonCircuitOpen Reason = "circuit_open"
)

// Topic is the DLQ topic name DESIGN.md §3 commits to.
const Topic = "judge-requests-dlq"

// Entry is one dead-lettered sample.
type Entry struct {
	Reason  Reason `json:"reason"`
	Detail  string `json:"detail"`
	Payload []byte `json:"payload"` // the original raw Kafka message value, unmodified
}

// Publisher writes Entries to the DLQ.
type Publisher interface {
	Publish(ctx context.Context, entry Entry) error
}

// KafkaPublisher is the production Publisher, keyed by tenant ID so a
// tenant's dead-lettered samples stay ordered relative to each other —
// same convention as cost-budget-enforcer's audit.KafkaPublisher.
type KafkaPublisher struct {
	writer *kafka.Writer
}

// NewKafkaPublisher constructs a Publisher writing to Topic on brokers.
func NewKafkaPublisher(brokers []string) *KafkaPublisher {
	return &KafkaPublisher{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        Topic,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll,
		},
	}
}

// Publish implements Publisher.
func (p *KafkaPublisher) Publish(ctx context.Context, entry Entry) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("dlq: marshal entry (reason=%s): %w", entry.Reason, err)
	}
	if err := p.writer.WriteMessages(ctx, kafka.Message{Value: payload}); err != nil {
		return fmt.Errorf("dlq: publish entry (reason=%s): %w", entry.Reason, err)
	}
	return nil
}

// Close flushes and releases the underlying Kafka connection.
func (p *KafkaPublisher) Close() error {
	return p.writer.Close()
}
