// SPDX-License-Identifier: MIT
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	topicFlagAudit = "flag_audit"
)

// KafkaEntry is one record published to the flag_audit Kafka topic.
// The topic is append-only; records are never updated or deleted.
type KafkaEntry struct {
	FlagName  string `json:"flag_name"`
	OldValue  string `json:"old_value"`
	NewValue  string `json:"new_value"`
	ChangedBy string `json:"changed_by"`
	ChangedAt int64  `json:"changed_at"` // unix nanoseconds
	Reason    string `json:"reason"`     // e.g. "kill-switch", "canary-advance"
}

// KafkaProducer publishes audit events to the flag_audit Kafka topic.
// The topic must be configured as append-only (cleanup.policy=delete,
// retention.ms=-1 or a long window) by the cluster operator.
type KafkaProducer struct {
	client *kgo.Client
}

// NewKafkaProducer creates a KafkaProducer connected to brokers.
func NewKafkaProducer(brokers []string) (*KafkaProducer, error) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.DefaultProduceTopic(topicFlagAudit),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.RecordDeliveryTimeout(10*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka connect: %w", err)
	}
	return &KafkaProducer{client: cl}, nil
}

// Publish serialises e and produces a record to flag_audit.
// The message key is the flag name so all mutations to one flag land
// on the same partition, preserving order within a flag.
func (p *KafkaProducer) Publish(ctx context.Context, e KafkaEntry) error {
	e.ChangedAt = time.Now().UnixNano()
	val, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal audit entry: %w", err)
	}
	rec := &kgo.Record{
		Topic: topicFlagAudit,
		Key:   []byte(e.FlagName),
		Value: val,
	}
	results := p.client.ProduceSync(ctx, rec)
	return results.FirstErr()
}

// Close flushes in-flight records and closes the Kafka connection.
func (p *KafkaProducer) Close() {
	p.client.Close()
}
