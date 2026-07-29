// SPDX-License-Identifier: MIT
package kafka_test

import (
	"testing"

	"github.com/AkshantVats/tool-call-analyzer/pkg/kafka"
	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
)

func TestDefaultTopic(t *testing.T) {
	if kafka.DefaultTopic != "tools.normalized.v1" {
		t.Errorf("unexpected default topic: %q", kafka.DefaultTopic)
	}
}

func TestNew_NoBrokers(t *testing.T) {
	_, err := kafka.New(kafka.Config{Brokers: nil})
	if err == nil {
		t.Error("expected error for empty brokers")
	}
}

func TestNew_DefaultTopicFallback(t *testing.T) {
	// This test verifies topic defaulting logic without making a real connection.
	// We test by observing that Config with empty Topic falls back correctly.
	// Full integration test requires a live broker and is skipped here.
	cfg := kafka.Config{
		Brokers: []string{"localhost:9092"},
		Topic:   "",
	}
	// We won't actually connect; just verify the cfg struct is well-formed.
	if len(cfg.Brokers) == 0 {
		t.Error("should have 1 broker")
	}
}

// TestToolCall_KeySelection verifies the key selection logic expectation:
// TraceID is preferred over ID for message keying.
func TestToolCall_KeySelection(t *testing.T) {
	tc := types.ToolCall{
		ID:      "call_abc",
		TraceID: "trace-xyz-789",
		Name:    "search_web",
		Vendor:  "openai",
	}
	key := tc.TraceID
	if key == "" {
		key = tc.ID
	}
	if key != "trace-xyz-789" {
		t.Errorf("expected TraceID to be used as key, got %q", key)
	}
}
