// SPDX-License-Identifier: MIT
// Package kafka publishes normalized ToolCall structs to the tools.normalized.v1 Kafka topic.
package kafka

import (
	"encoding/json"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
)

const DefaultTopic = "tools.normalized.v1"

// Publisher sends canonical ToolCall messages to Kafka.
type Publisher struct {
	producer sarama.SyncProducer
	topic    string
}

// Config holds Kafka publisher configuration.
type Config struct {
	Brokers []string
	Topic   string
}

// New creates a Publisher connected to the given brokers.
// Topic defaults to DefaultTopic when config.Topic is empty.
func New(cfg Config) (*Publisher, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("no brokers specified")
	}
	topic := cfg.Topic
	if topic == "" {
		topic = DefaultTopic
	}

	sc := sarama.NewConfig()
	sc.Producer.Return.Successes = true
	sc.Producer.RequiredAcks = sarama.WaitForAll
	sc.Producer.Compression = sarama.CompressionSnappy

	p, err := sarama.NewSyncProducer(cfg.Brokers, sc)
	if err != nil {
		return nil, fmt.Errorf("create sarama producer: %w", err)
	}
	return &Publisher{producer: p, topic: topic}, nil
}

// Publish serializes tc as JSON and sends it to the configured topic.
// The Kafka message key is tc.TraceID when set, otherwise tc.ID.
func (p *Publisher) Publish(tc types.ToolCall) (partition int32, offset int64, err error) {
	b, err := json.Marshal(tc)
	if err != nil {
		return 0, 0, fmt.Errorf("marshal ToolCall: %w", err)
	}
	key := tc.TraceID
	if key == "" {
		key = tc.ID
	}
	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(b),
	}
	return p.producer.SendMessage(msg)
}

// Close shuts down the underlying Kafka producer.
func (p *Publisher) Close() error {
	return p.producer.Close()
}
