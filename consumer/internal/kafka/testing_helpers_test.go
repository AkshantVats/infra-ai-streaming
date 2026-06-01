package kafka

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/metrics"
)

// newFakeRecord builds a minimal *kgo.Record for unit tests that do not
// connect to a real Kafka broker. Only the Value field is populated.
func newFakeRecord(value []byte) *kgo.Record {
	return &kgo.Record{
		Topic:     "ai_inference_events",
		Partition: 0,
		Offset:    0,
		Value:     value,
	}
}

// newTestMetrics returns a fresh *metrics.M backed by an isolated Prometheus
// registry. Using a fresh registry per call avoids duplicate-registration
// panics when multiple tests in the same binary call this function.
func newTestMetrics() *metrics.M {
	return metrics.NewWithRegistry(prometheus.NewRegistry())
}
