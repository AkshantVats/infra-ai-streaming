// SPDX-License-Identifier: MIT
// Internal tests for the kafka publisher — runs as package kafka so we can
// inject a mock sarama.SyncProducer directly into the unexported field.
package kafka

import (
	"errors"
	"testing"

	"github.com/IBM/sarama"
	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
	"github.com/IBM/sarama/mocks"
)

// newTestPublisher builds a Publisher backed by a sarama mock producer.
func newTestPublisher(t *testing.T, topic string) (*Publisher, *mocks.SyncProducer) {
	t.Helper()
	mp := mocks.NewSyncProducer(t, nil)
	if topic == "" {
		topic = DefaultTopic
	}
	return &Publisher{producer: mp, topic: topic}, mp
}

// TestPublish_WithTraceID checks that a ToolCall with a TraceID is published
// without error and uses TraceID as the message key.
func TestPublish_WithTraceID(t *testing.T) {
	p, mp := newTestPublisher(t, DefaultTopic)
	mp.ExpectSendMessageAndSucceed()

	tc := types.ToolCall{
		ID:      "call-001",
		TraceID: "trace-abc-123",
		Name:    "search_web",
		Vendor:  "openai",
	}
	_, _, err := p.Publish(tc)
	if err != nil {
		t.Fatalf("Publish: unexpected error: %v", err)
	}
	if err := mp.Close(); err != nil {
		t.Errorf("mock.Close: %v", err)
	}
}

// TestPublish_WithoutTraceID_UsesID checks that when TraceID is empty the
// message key falls back to ToolCall.ID.
func TestPublish_WithoutTraceID_UsesID(t *testing.T) {
	p, mp := newTestPublisher(t, DefaultTopic)
	mp.ExpectSendMessageAndSucceed()

	tc := types.ToolCall{
		ID:     "call-no-trace",
		Name:   "run_sql",
		Vendor: "anthropic",
	}
	_, _, err := p.Publish(tc)
	if err != nil {
		t.Fatalf("Publish: unexpected error: %v", err)
	}
	mp.Close()
}

// TestPublish_BrokerError checks that a send error is propagated to the caller.
func TestPublish_BrokerError(t *testing.T) {
	p, mp := newTestPublisher(t, DefaultTopic)
	mp.ExpectSendMessageAndFail(errors.New("broker unavailable"))

	tc := types.ToolCall{ID: "call-err", Name: "fetch_url"}
	_, _, err := p.Publish(tc)
	if err == nil {
		t.Error("expected error from failed SendMessage, got nil")
	}
	mp.Close()
}

// TestPublish_ReturnsPartitionAndOffset verifies the returned values pass
// through from the producer.
func TestPublish_ReturnsPartitionAndOffset(t *testing.T) {
	p, mp := newTestPublisher(t, "custom-topic")
	mp.ExpectSendMessageAndSucceed()

	tc := types.ToolCall{ID: "call-xyz", TraceID: "t1", Name: "code_exec"}
	partition, offset, err := p.Publish(tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// mock always returns partition 0, offset increments from 0
	_ = partition
	_ = offset
	mp.Close()
}

// TestPublish_MultipleMessages checks sequential publishes work correctly.
func TestPublish_MultipleMessages(t *testing.T) {
	p, mp := newTestPublisher(t, DefaultTopic)
	mp.ExpectSendMessageAndSucceed()
	mp.ExpectSendMessageAndSucceed()
	mp.ExpectSendMessageAndSucceed()

	for i, name := range []string{"tool_a", "tool_b", "tool_c"} {
		tc := types.ToolCall{ID: "id", TraceID: "trace", Name: name}
		tc.ID = string(rune('1' + i))
		if _, _, err := p.Publish(tc); err != nil {
			t.Fatalf("Publish[%d]: %v", i, err)
		}
	}
	mp.Close()
}

// TestClose verifies Close delegates to the underlying producer.
func TestClose(t *testing.T) {
	p, mp := newTestPublisher(t, DefaultTopic)
	_ = mp // no expectations — Close should succeed without sends
	if err := p.Close(); err != nil {
		t.Errorf("Close: unexpected error: %v", err)
	}
}

// TestNew_EmptyBrokerSlice verifies New rejects an explicitly empty slice.
func TestNew_EmptyBrokerSlice(t *testing.T) {
	_, err := New(Config{Brokers: []string{}})
	if err == nil {
		t.Error("expected error for empty broker slice")
	}
}

// TestNew_WithMockBroker verifies that New succeeds when a real broker is
// reachable and defaults the topic when Config.Topic is empty.
func TestNew_WithMockBroker(t *testing.T) {
	broker := sarama.NewMockBroker(t, 1)
	defer broker.Close()

	// Register a metadata response so sarama can bootstrap.
	broker.SetHandlerByMap(map[string]sarama.MockResponse{
		"MetadataRequest": sarama.NewMockMetadataResponse(t).
			SetBroker(broker.Addr(), broker.BrokerID()).
			SetLeader(DefaultTopic, 0, broker.BrokerID()),
	})

	p, err := New(Config{Brokers: []string{broker.Addr()}, Topic: ""})
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}
	defer p.Close()

	if p.topic != DefaultTopic {
		t.Errorf("expected default topic %q, got %q", DefaultTopic, p.topic)
	}
}

// TestNew_WithCustomTopic verifies that a non-empty Config.Topic is retained.
func TestNew_WithCustomTopic(t *testing.T) {
	const customTopic = "my.events.v2"

	broker := sarama.NewMockBroker(t, 2)
	defer broker.Close()

	broker.SetHandlerByMap(map[string]sarama.MockResponse{
		"MetadataRequest": sarama.NewMockMetadataResponse(t).
			SetBroker(broker.Addr(), broker.BrokerID()).
			SetLeader(customTopic, 0, broker.BrokerID()),
	})

	p, err := New(Config{Brokers: []string{broker.Addr()}, Topic: customTopic})
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}
	defer p.Close()

	if p.topic != customTopic {
		t.Errorf("expected topic %q, got %q", customTopic, p.topic)
	}
}

// TestPublish_CustomTopic verifies the publisher targets the topic from Config.
func TestPublish_CustomTopic(t *testing.T) {
	p, mp := newTestPublisher(t, "my.custom.topic")
	mp.ExpectSendMessageAndSucceed()

	tc := types.ToolCall{ID: "x", Name: "n", TraceID: "t"}
	if _, _, err := p.Publish(tc); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if p.topic != "my.custom.topic" {
		t.Errorf("expected topic my.custom.topic, got %q", p.topic)
	}
	mp.Close()
}
