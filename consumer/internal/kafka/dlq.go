package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/metrics"
	"github.com/akshantvats/infra-ai-streaming/consumer/internal/model"
)

// DLQMessage is published to ai_inference_dlq after insert retries are exhausted.
type DLQMessage struct {
	Event     model.InferenceEvent `json:"event"`
	Error     string               `json:"error"`
	Retries   int                  `json:"retries"`
	Timestamp int64                `json:"failed_at_unix_ms"`
}

// DLQPublisher sends failed events to the DLQ topic.
type DLQPublisher struct {
	client *kgo.Client
	topic  string
	m      *metrics.M
}

// NewDLQPublisher creates a franz-go producer for the DLQ topic.
func NewDLQPublisher(brokers, topic string, m *metrics.M) (*DLQPublisher, error) {
	seed := strings.Split(brokers, ",")
	for i := range seed {
		seed[i] = strings.TrimSpace(seed[i])
	}
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(seed...),
		kgo.DefaultProduceTopic(topic),
	)
	if err != nil {
		return nil, fmt.Errorf("create dlq producer: %w", err)
	}
	return &DLQPublisher{client: cl, topic: topic, m: m}, nil
}

// Publish emits one event to the DLQ topic.
func (p *DLQPublisher) Publish(ctx context.Context, event model.InferenceEvent, errMsg string, retries int) error {
	msg := DLQMessage{
		Event:     event,
		Error:     errMsg,
		Retries:   retries,
		Timestamp: time.Now().UnixMilli(),
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	rec := &kgo.Record{Topic: p.topic, Value: body}
	res := p.client.ProduceSync(ctx, rec)
	if err := res.FirstErr(); err != nil {
		return fmt.Errorf("dlq produce: %w", err)
	}
	p.m.DLQEvents.Inc()
	return nil
}

// Close shuts down the DLQ producer.
func (p *DLQPublisher) Close() {
	p.client.Close()
}
