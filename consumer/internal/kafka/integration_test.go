// SPDX-License-Identifier: MIT

//go:build integration

package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/config"
	"github.com/akshantvats/infra-ai-streaming/consumer/internal/model"
)

func kafkaBrokers(t *testing.T) string {
	t.Helper()
	v := os.Getenv("KAFKA_BROKERS")
	if v == "" {
		t.Skip("set KAFKA_BROKERS to run Kafka integration tests")
	}
	return v
}

// TestNewReaderConnects verifies that NewReader successfully dials the broker.
func TestNewReaderConnects(t *testing.T) {
	brokers := kafkaBrokers(t)

	cfg := config.Config{
		KafkaBrokers: brokers,
		KafkaTopic:   "ai_inference_events",
		KafkaGroupID: "integration-test-connect",
	}
	sink := &captureSink{}
	r, err := NewReader(cfg, sink, newTestMetrics(), nil, nil)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()
}

// TestReaderRunConsumesMessage produces a message to a dedicated test topic,
// then verifies that a Reader (using AtStart to avoid timing issues)
// delivers the event to the sink via Run.
func TestReaderRunConsumesMessage(t *testing.T) {
	brokers := kafkaBrokers(t)

	topic := fmt.Sprintf("integration-test-%d", time.Now().UnixNano())

	// Build and produce a test record using a raw kgo producer.
	batch := model.IngestBatch{
		Events: []model.InferenceEvent{
			{
				TenantID:         "integ-tenant",
				ModelID:          "gpt-4o",
				TimestampUnixMs:  1715000000000,
				LatencyMs:        42,
				PromptTokens:     10,
				CompletionTokens: 5,
				CostUSD:          0.001,
			},
		},
	}
	payload, err := json.Marshal(batch)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Pre-create topic: auto_create_topics_enabled is async; ProduceSync may race.
	// Use rpk inside the container (internal listener :29092) to create synchronously.
	rpkOut, rpkErr := exec.CommandContext(ctx, "docker", "exec",
		"infra-ai-test-redpanda-1",
		"rpk", "topic", "create", topic,
		"--brokers", "localhost:29092",
		"--partitions", "1", "--replicas", "1",
	).CombinedOutput()
	if rpkErr != nil {
		// Topic may already exist — that's fine; any other error is fatal.
		if !contains(string(rpkOut), "TOPIC_ALREADY_EXISTS") {
			t.Fatalf("create topic %q: %v — %s", topic, rpkErr, rpkOut)
		}
	}

	producer, err := kgo.NewClient(kgo.SeedBrokers(brokers))
	if err != nil {
		t.Fatalf("new producer: %v", err)
	}
	defer producer.Close()

	if err := producer.ProduceSync(ctx, &kgo.Record{Topic: topic, Value: payload}).FirstErr(); err != nil {
		t.Fatalf("produce: %v", err)
	}

	// Build a kgo.Client that reads from start (no consumer group, explicit offset).
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		t.Fatalf("new consumer client: %v", err)
	}

	// Wrap in a Reader directly (same package, so unexported fields are accessible).
	var mu sync.Mutex
	var received []model.InferenceEvent
	done := make(chan struct{})

	sink := &funcSink{fn: func(events []model.InferenceEvent) error {
		mu.Lock()
		received = append(received, events...)
		mu.Unlock()
		close(done)
		return nil
	}}

	r := &Reader{
		client: cl,
		topic:  topic,
		sink:   sink,
		m:      newTestMetrics(),
	}
	defer r.Close()

	runCtx, runCancel := context.WithCancel(context.Background())
	go func() {
		_ = r.Run(runCtx)
	}()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("timed out waiting for message from Kafka")
	}
	runCancel()

	mu.Lock()
	defer mu.Unlock()
	if len(received) == 0 {
		t.Fatal("sink received no events")
	}
	if received[0].TenantID != "integ-tenant" {
		t.Errorf("TenantID = %q, want integ-tenant", received[0].TenantID)
	}
	if received[0].LatencyMs != 42 {
		t.Errorf("LatencyMs = %d, want 42", received[0].LatencyMs)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || containsStr(s, sub))
}
func containsStr(s, sub string) bool {
	for i := range len(s) - len(sub) + 1 {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// funcSink is a one-shot EventSink that calls fn on Accept then becomes a no-op.
type funcSink struct {
	fn   func([]model.InferenceEvent) error
	once sync.Once
}

func (s *funcSink) Accept(_ context.Context, events []model.InferenceEvent) error {
	var retErr error
	s.once.Do(func() {
		retErr = s.fn(events)
	})
	return retErr
}
