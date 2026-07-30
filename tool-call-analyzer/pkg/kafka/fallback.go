// SPDX-License-Identifier: MIT
// This file adds a second, independent publish path to package kafka:
// FallbackProducer buffers spans that pkg/clickhouse.Writer could not write
// in time so they are queued, not dropped -- the same lesson a Kafka
// backpressure buffer teaches at Agoda/WhiteFalcon, applied here to
// TraceForge's own write path. It is unrelated to Publisher's job
// (publishing canonical ToolCall events to tools.normalized.v1) but reuses
// the same sarama.SyncProducer pattern so the package doesn't grow a second
// competing Kafka client dependency.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/IBM/sarama"
)

// FallbackTopic is the default Kafka topic spans are buffered to when the
// ClickHouse write path (pkg/clickhouse.Writer.Insert) fails or exceeds its
// write deadline. Deliberately distinct from Publisher's DefaultTopic
// ("tools.normalized.v1") -- the two topics carry different payloads for
// different consumers.
const FallbackTopic = "tool-spans"

// SpanEvent is the JSON envelope written to the fallback topic. Its fields
// mirror the flattened row pkg/clickhouse.Writer would otherwise have
// inserted into ClickHouse, plus buffering metadata, so a future recovery
// consumer can replay it into ClickHouse without needing the original
// types.ToolCall.
type SpanEvent struct {
	TraceID         string  `json:"trace_id"`
	ToolID          string  `json:"tool_id"`
	ToolName        string  `json:"tool_name"`
	Vendor          string  `json:"vendor"`
	Category        string  `json:"category"`
	ModelName       string  `json:"model_name"`
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	CostUSD         float64 `json:"cost_usd"`
	DurationMs      uint64  `json:"duration_ms"`
	TraceDurationMs uint64  `json:"trace_duration_ms"`
	HasError        uint8   `json:"has_error"`
	Status          string  `json:"status"`
	Timestamp       string  `json:"timestamp"`

	// Buffering metadata -- not present on the ClickHouse row, added when
	// the span is diverted to Kafka.
	BufferedAtUnixMs int64  `json:"buffered_at_unix_ms"`
	BufferReason     string `json:"buffer_reason"` // "clickhouse_error" | "clickhouse_timeout"
}

// FallbackConfig configures a FallbackProducer.
type FallbackConfig struct {
	Brokers []string
	Topic   string
}

// FallbackProducer buffers SpanEvents to Kafka using the same
// sarama.SyncProducer pattern as Publisher, so a caller that fails (or is
// too slow) to write to ClickHouse can hand the span off instead of
// dropping it.
type FallbackProducer struct {
	producer sarama.SyncProducer
	topic    string
}

// NewFallback creates a FallbackProducer connected to the given brokers.
// Topic defaults to FallbackTopic when cfg.Topic is empty.
func NewFallback(cfg FallbackConfig) (*FallbackProducer, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("no brokers specified")
	}
	topic := cfg.Topic
	if topic == "" {
		topic = FallbackTopic
	}

	sc := sarama.NewConfig()
	sc.Producer.Return.Successes = true
	sc.Producer.RequiredAcks = sarama.WaitForLocal
	sc.Producer.Compression = sarama.CompressionSnappy

	p, err := sarama.NewSyncProducer(cfg.Brokers, sc)
	if err != nil {
		return nil, fmt.Errorf("create sarama producer: %w", err)
	}
	return &FallbackProducer{producer: p, topic: topic}, nil
}

// NewFallbackFromEnv builds a FallbackProducer from KAFKA_BROKERS
// (comma-separated) and KAFKA_TOPIC (optional, defaults to FallbackTopic).
// It returns (nil, nil) when KAFKA_BROKERS is unset -- callers must
// nil-check the result before wiring it into clickhouse.Writer.SetFallback,
// which is exactly how Kafka buffering stays opt-in per the environment.
func NewFallbackFromEnv() (*FallbackProducer, error) {
	brokersEnv := os.Getenv("KAFKA_BROKERS")
	if strings.TrimSpace(brokersEnv) == "" {
		return nil, nil
	}

	var brokers []string
	for _, b := range strings.Split(brokersEnv, ",") {
		if b = strings.TrimSpace(b); b != "" {
			brokers = append(brokers, b)
		}
	}
	if len(brokers) == 0 {
		return nil, nil
	}

	return NewFallback(FallbackConfig{Brokers: brokers, Topic: os.Getenv("KAFKA_TOPIC")})
}

// NewFallbackWithProducer wraps an already-constructed sarama.SyncProducer
// as a FallbackProducer. This mirrors clickhouse.NewWithClient's dependency
// injection pattern (an *http.Client passed in directly) and lets callers --
// chiefly tests -- inject a sarama/mocks.SyncProducer without dialing a real
// broker. Topic defaults to FallbackTopic when empty.
func NewFallbackWithProducer(producer sarama.SyncProducer, topic string) *FallbackProducer {
	if topic == "" {
		topic = FallbackTopic
	}
	return &FallbackProducer{producer: producer, topic: topic}
}

// Send publishes ev as JSON to the fallback topic. The message key is
// ev.ToolID so retries/replays of the same span land on the same partition
// (dedup-friendly for a future recovery consumer). Returns an error if ctx
// is already cancelled or the underlying send fails; the caller decides
// whether to log the failure or treat the span as truly dropped.
func (f *FallbackProducer) Send(ctx context.Context, ev SpanEvent) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("kafka fallback: %w", err)
	}

	if ev.BufferedAtUnixMs == 0 {
		ev.BufferedAtUnixMs = time.Now().UnixMilli()
	}

	b, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("kafka fallback: marshal span event: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: f.topic,
		Key:   sarama.StringEncoder(ev.ToolID),
		Value: sarama.ByteEncoder(b),
	}
	if _, _, err := f.producer.SendMessage(msg); err != nil {
		return fmt.Errorf("kafka fallback: send failed: %w", err)
	}
	return nil
}

// Close shuts down the underlying Kafka producer.
func (f *FallbackProducer) Close() error {
	return f.producer.Close()
}
