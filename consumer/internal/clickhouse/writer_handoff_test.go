// SPDX-License-Identifier: MIT
package clickhouse

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/metrics"
	"github.com/akshantvats/infra-ai-streaming/consumer/internal/model"
)

// newTestMetrics returns an M backed by a fresh isolated registry so that
// multiple test functions can each call it without duplicate-registration panics.
func newTestMetrics() *metrics.M {
	return metrics.NewWithRegistry(prometheus.NewRegistry())
}

type mockOverflow struct {
	pushed int
}

func (m *mockOverflow) Push(_ context.Context, events []model.InferenceEvent) error {
	m.pushed += len(events)
	return nil
}

func (m *mockOverflow) PopN(context.Context, int) ([]model.InferenceEvent, error) {
	return nil, nil
}

func (m *mockOverflow) Depth(context.Context) (int64, error) {
	return int64(m.pushed), nil
}

func sampleEvent() model.InferenceEvent {
	return model.InferenceEvent{
		TenantID:         "t1",
		ModelID:          "gpt-4o",
		TimestampUnixMs:  1715000000000,
		LatencyMs:        10,
		PromptTokens:     1,
		CompletionTokens: 1,
		CostUSD:          0.01,
	}
}

func TestFinishHandoffSignalWhenRemainingZero(t *testing.T) {
	w := &BatchWriter{handoffSignals: make(map[uint64]*handoffSignal)}
	done := make(chan struct{})
	w.handoffSignals[1] = &handoffSignal{remaining: 2, done: done}
	w.finishHandoffSignal(1, 1)
	select {
	case <-done:
		t.Fatal("done closed early")
	default:
	}
	w.finishHandoffSignal(1, 1)
	select {
	case <-done:
	default:
		t.Fatal("done not closed")
	}
	if _, ok := w.handoffSignals[1]; ok {
		t.Fatal("handoff signal should be removed")
	}
}

func TestHandoffEventsOverflowWhenBreakerOpen(t *testing.T) {
	overflow := &mockOverflow{}
	w := &BatchWriter{
		cb:             NewCircuitBreaker(1, 30*time.Minute),
		overflow:       overflow,
		m:              newTestMetrics(),
		handoffSignals: make(map[uint64]*handoffSignal),
	}
	w.cb.RecordFailure()
	events := []model.InferenceEvent{sampleEvent(), sampleEvent()}
	w.handoffEvents(context.Background(), events, nil)
	if overflow.pushed != 2 {
		t.Fatalf("overflow pushed %d events, want 2", overflow.pushed)
	}
	if w.cb.State() != BreakerOpen {
		t.Fatalf("breaker state = %v, want open", w.cb.State())
	}
}

func TestAcceptEmptyReturnsNil(t *testing.T) {
	w := &BatchWriter{handoffSignals: make(map[uint64]*handoffSignal)}
	if err := w.Accept(context.Background(), nil); err != nil {
		t.Fatalf("Accept(nil): %v", err)
	}
}
