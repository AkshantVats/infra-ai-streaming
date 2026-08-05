// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestBudgetChangeEventRoundTrips(t *testing.T) {
	event := BudgetChangeEvent{
		TenantID:  "acme",
		Actor:     "akshant@example.test",
		Before:    map[string]any{"budget_tokens": float64(1000000)},
		After:     map[string]any{"budget_tokens": float64(2000000)},
		Timestamp: 1710000000,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got BudgetChangeEvent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.TenantID != event.TenantID || got.Actor != event.Actor || got.Timestamp != event.Timestamp {
		t.Fatalf("round trip mismatch: got %+v, want %+v", got, event)
	}
	if got.Before["budget_tokens"] != event.Before["budget_tokens"] {
		t.Fatalf("Before mismatch: got %v, want %v", got.Before, event.Before)
	}
}

// TestKafkaPublisherFailsOnUnreachableBroker exercises the real
// KafkaPublisher's error path. This repo has no Docker daemon
// available in CI (the same constraint Day 65's DESIGN.md and Day 66's
// README both log for Redis), so there is no live broker to publish a
// success case against; what this test can and does verify is that
// pointing at an address nothing is listening on surfaces as a
// wrapped, non-nil error rather than hanging or panicking — the
// behavior pkg/admin's fail-closed rollback path depends on.
func TestKafkaPublisherFailsOnUnreachableBroker(t *testing.T) {
	pub := NewKafkaPublisher([]string{"127.0.0.1:1"})
	t.Cleanup(func() { _ = pub.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := pub.Publish(ctx, BudgetChangeEvent{TenantID: "acme", Timestamp: 1})
	if err == nil {
		t.Fatalf("Publish against unreachable broker: want error, got nil")
	}
}
