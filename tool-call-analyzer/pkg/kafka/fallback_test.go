// SPDX-License-Identifier: MIT
package kafka_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/AkshantVats/tool-call-analyzer/pkg/kafka"
	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
)

func TestFallbackTopicDefault(t *testing.T) {
	if kafka.FallbackTopic != "tool-spans" {
		t.Errorf("unexpected fallback topic: %q", kafka.FallbackTopic)
	}
}

// TestFallbackSend_TableDriven covers the range of SpanEvent shapes Send
// needs to handle without error: a normal span, a zero-cost span (an
// internal tool with no LLM cost attribution), and an errored span.
func TestFallbackSend_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		ev   kafka.SpanEvent
	}{
		{
			name: "normal span",
			ev:   kafka.SpanEvent{ToolID: "tool-1", ToolName: "search_web", CostUSD: 0.002, BufferReason: "clickhouse_timeout"},
		},
		{
			name: "zero cost span",
			ev:   kafka.SpanEvent{ToolID: "tool-2", ToolName: "internal_tool", CostUSD: 0, BufferReason: "clickhouse_error"},
		},
		{
			name: "errored span",
			ev:   kafka.SpanEvent{ToolID: "tool-3", ToolName: "run_sql", HasError: 1, Status: "ERROR", BufferReason: "clickhouse_error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp := mocks.NewSyncProducer(t, nil)
			p := kafka.NewFallbackWithProducer(mp, "")
			mp.ExpectSendMessageAndSucceed()

			if err := p.Send(context.Background(), tt.ev); err != nil {
				t.Fatalf("Send: unexpected error: %v", err)
			}
			if err := mp.Close(); err != nil {
				t.Errorf("mock.Close: %v", err)
			}
		})
	}
}

// TestFallbackSend_ZeroCostSerializesAsZero verifies cost_usd: 0 survives
// JSON marshaling rather than being omitted (a future recovery consumer
// needs to distinguish "no cost" from "field missing").
func TestFallbackSend_ZeroCostSerializesAsZero(t *testing.T) {
	mp := mocks.NewSyncProducer(t, nil)
	p := kafka.NewFallbackWithProducer(mp, "")
	mp.ExpectSendMessageWithCheckerFunctionAndSucceed(func(val []byte) error {
		var got kafka.SpanEvent
		if err := json.Unmarshal(val, &got); err != nil {
			return err
		}
		if got.CostUSD != 0 {
			t.Errorf("expected cost_usd 0, got %v", got.CostUSD)
		}
		return nil
	})

	if err := p.Send(context.Background(), kafka.SpanEvent{ToolID: "t-zero", CostUSD: 0}); err != nil {
		t.Fatalf("Send: unexpected error: %v", err)
	}
	mp.Close()
}

// TestFallbackSend_KeyIsToolID checks the message key stays deterministic
// across two sends of the same span, matching the partition-stability
// requirement recorded in the design doc for the future recovery consumer.
func TestFallbackSend_KeyIsToolID(t *testing.T) {
	mp := mocks.NewSyncProducer(t, nil)
	p := kafka.NewFallbackWithProducer(mp, "")

	var keys []string
	mp.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(func(msg *sarama.ProducerMessage) error {
		k, err := msg.Key.Encode()
		if err != nil {
			return err
		}
		keys = append(keys, string(k))
		return nil
	})
	mp.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(func(msg *sarama.ProducerMessage) error {
		k, err := msg.Key.Encode()
		if err != nil {
			return err
		}
		keys = append(keys, string(k))
		return nil
	})

	ev := kafka.SpanEvent{ToolID: "tool-deterministic", ToolName: "search_web"}
	if err := p.Send(context.Background(), ev); err != nil {
		t.Fatalf("Send #1: %v", err)
	}
	if err := p.Send(context.Background(), ev); err != nil {
		t.Fatalf("Send #2: %v", err)
	}
	mp.Close()

	if len(keys) != 2 || keys[0] != keys[1] {
		t.Errorf("expected identical keys across sends, got %v", keys)
	}
	if len(keys) > 0 && keys[0] != "tool-deterministic" {
		t.Errorf("expected key %q, got %q", "tool-deterministic", keys[0])
	}
}

// TestFallbackNewFallbackFromEnv_NilWhenUnset checks New*FromEnv returns a
// nil producer (not an error) when KAFKA_BROKERS is unset, so callers can
// nil-check and fall back to log-and-drop behaviour.
func TestFallbackNewFallbackFromEnv_NilWhenUnset(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "")
	os.Unsetenv("KAFKA_BROKERS")

	fb, err := kafka.NewFallbackFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fb != nil {
		t.Errorf("expected nil FallbackProducer when KAFKA_BROKERS unset, got %+v", fb)
	}
}

// TestFallbackNew_NoBrokers verifies NewFallback rejects an explicitly
// empty broker list, mirroring Publisher's New(Config{Brokers: nil}) test.
func TestFallbackNew_NoBrokers(t *testing.T) {
	_, err := kafka.NewFallback(kafka.FallbackConfig{Brokers: nil})
	if err == nil {
		t.Error("expected error for empty brokers")
	}
}

// TestFallbackClose verifies Close delegates to the underlying producer
// with no pending expectations.
func TestFallbackClose(t *testing.T) {
	mp := mocks.NewSyncProducer(t, nil)
	p := kafka.NewFallbackWithProducer(mp, "")
	if err := p.Close(); err != nil {
		t.Errorf("Close: unexpected error: %v", err)
	}
}

// TestFallbackSend_BrokerError checks that a send error from the underlying
// producer is propagated to the caller rather than swallowed.
func TestFallbackSend_BrokerError(t *testing.T) {
	mp := mocks.NewSyncProducer(t, nil)
	p := kafka.NewFallbackWithProducer(mp, "")
	mp.ExpectSendMessageAndFail(errors.New("broker unavailable"))

	err := p.Send(context.Background(), kafka.SpanEvent{ToolID: "tool-err"})
	if err == nil {
		t.Error("expected error from failed SendMessage, got nil")
	}
	mp.Close()
}

// TestFallbackSend_ContextCancelled checks Send returns promptly with an
// error when the context is already cancelled, rather than attempting the
// send and hanging.
func TestFallbackSend_ContextCancelled(t *testing.T) {
	mp := mocks.NewSyncProducer(t, nil)
	p := kafka.NewFallbackWithProducer(mp, "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() { done <- p.Send(ctx, kafka.SpanEvent{ToolID: "tool-cancelled"}) }()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected error for cancelled context, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Send did not return promptly for a cancelled context")
	}
}

// TestFallbackSend_CustomTopic verifies a FallbackProducer built with a
// non-default topic actually targets it.
func TestFallbackSend_CustomTopic(t *testing.T) {
	mp := mocks.NewSyncProducer(t, nil)
	p := kafka.NewFallbackWithProducer(mp, "custom-spans-topic")
	mp.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(func(msg *sarama.ProducerMessage) error {
		if msg.Topic != "custom-spans-topic" {
			t.Errorf("expected topic custom-spans-topic, got %q", msg.Topic)
		}
		return nil
	})

	if err := p.Send(context.Background(), kafka.SpanEvent{ToolID: "t"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	mp.Close()
}
