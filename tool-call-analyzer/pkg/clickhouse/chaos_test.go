// SPDX-License-Identifier: MIT
// Chaos-style test: an overloaded ClickHouse (simulated with artificial
// per-request latency) must not cause spans to be dropped -- Insert has to
// buffer them to the Kafka fallback instead. This is the "queued, not
// dropped" lesson a Kafka backpressure buffer teaches, applied to the real
// pkg/clickhouse.Writer.Insert write path.
//
// It uses sarama/mocks.SyncProducer (already a dependency via
// github.com/IBM/sarama) rather than a real broker, so it runs in normal CI
// without needing Kafka up -- no build tag required.
package clickhouse_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/AkshantVats/tool-call-analyzer/pkg/clickhouse"
	"github.com/AkshantVats/tool-call-analyzer/pkg/kafka"
	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
	"github.com/IBM/sarama/mocks"
)

// TestChaosSlowClickHouseAllSpansBuffered fires 100 spans at Insert
// concurrently (10 goroutines x 10 spans each) against a ClickHouse stub
// that sleeps 200ms per request, simulating an overloaded aggregator. The
// Writer's fallback deadline is set well below that (20ms), so every
// request is expected to time out and divert to the Kafka fallback. The
// test asserts none of the 100 spans are dropped: every Insert call returns
// nil, and the mock producer receives and accepts exactly 100 messages.
func TestChaosSlowClickHouseAllSpansBuffered(t *testing.T) {
	const (
		goroutines       = 10
		spansPerRoutine  = 10
		totalSpans       = goroutines * spansPerRoutine
		chLatency        = 200 * time.Millisecond
		fallbackDeadline = 20 * time.Millisecond
	)

	slowClickHouse := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(chLatency)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowClickHouse.Close()

	mp := mocks.NewSyncProducer(t, nil)
	for i := 0; i < totalSpans; i++ {
		mp.ExpectSendMessageAndSucceed()
	}
	fb := kafka.NewFallbackWithProducer(mp, "")

	writer := clickhouse.NewWithClient(slowClickHouse.URL, slowClickHouse.Client())
	writer.SetFallback(fb)
	writer.SetFallbackDeadline(fallbackDeadline)

	var wg sync.WaitGroup
	errs := make(chan error, totalSpans)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(routine int) {
			defer wg.Done()
			for i := 0; i < spansPerRoutine; i++ {
				tc := types.ToolCall{
					ID:       spanID(routine, i),
					TraceID:  "trace-chaos",
					Name:     "search_web",
					Vendor:   "openai",
					Category: types.CategoryHTTP,
				}
				err := writer.Insert(context.Background(), tc, 50, 500, false, "OK")
				errs <- err
			}
		}(g)
	}

	wg.Wait()
	close(errs)

	dropped := 0
	for err := range errs {
		if err != nil {
			dropped++
			t.Errorf("span was dropped instead of buffered: %v", err)
		}
	}

	if dropped != 0 {
		t.Fatalf("expected 0 dropped spans out of %d, got %d dropped", totalSpans, dropped)
	}

	if err := mp.Close(); err != nil {
		t.Errorf("mock producer did not receive all expected messages: %v", err)
	}
}

func spanID(routine, i int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	return string(letters[routine%len(letters)]) + "-" + string(letters[i%len(letters)])
}
