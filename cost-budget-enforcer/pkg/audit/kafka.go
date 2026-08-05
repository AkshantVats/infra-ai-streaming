// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
)

// KafkaPublisher publishes BudgetChangeEvents to Topic, keyed by tenant
// ID so every change for a given tenant lands on the same partition
// and a consumer replaying the log sees that tenant's changes in the
// order they were applied — the same ordering guarantee the root
// `ingestion` producer relies on for per-tenant event streams,
// reproduced here in Go via kafka-go instead of ingestion's rdkafka
// binding, since this module has no other cgo dependency to justify
// bringing one in just for this.
type KafkaPublisher struct {
	writer *kafka.Writer
}

// NewKafkaPublisher constructs a publisher that requires acknowledgment
// from all in-sync replicas before Publish returns — an audit record
// that Kafka hasn't durably accepted yet is indistinguishable from a
// record that was never published, and pkg/admin's fail-closed
// contract (see Publisher's doc comment) depends on that distinction
// being real, not just requested.
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
func (p *KafkaPublisher) Publish(ctx context.Context, event BudgetChangeEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("audit: marshal event for tenant %s: %w", event.TenantID, err)
	}
	err = p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.TenantID),
		Value: payload,
	})
	if err != nil {
		return fmt.Errorf("audit: publish event for tenant %s: %w", event.TenantID, err)
	}
	return nil
}

// Close flushes and releases the underlying Kafka connection. Callers
// should defer Close for the lifetime of the process that owns this
// publisher, the same way ingestion's producer is closed on shutdown.
func (p *KafkaPublisher) Close() error {
	return p.writer.Close()
}
