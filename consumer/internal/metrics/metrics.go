// Package metrics exposes Prometheus instrumentation for the consumer.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// M holds consumer Prometheus metrics.
type M struct {
	KafkaRecordsProcessed      prometheus.Counter
	ClickHouseWriteErrors      prometheus.Counter
	ClickHouseBatchSize        prometheus.Histogram
	ClickHouseFlushDur         prometheus.Histogram
	CircuitBreakerState        *prometheus.GaugeVec
	RedisOverflowDepth         prometheus.Gauge
	DLQEvents                  prometheus.Counter
	KafkaConsumerLagEvents     *prometheus.GaugeVec
	KafkaDeserializationErrors prometheus.Counter
	KafkaRecordHandoffErrors   prometheus.Counter
	AnomaliesDetectedTotal     *prometheus.CounterVec
}

// New registers metrics with the default Prometheus registry.
// Use NewWithRegistry when the default registry is unavailable (e.g., in tests).
func New() *M {
	return NewWithRegistry(prometheus.DefaultRegisterer)
}

// NewWithRegistry registers metrics with the given Prometheus registerer.
// Pass prometheus.NewRegistry() in tests to avoid duplicate-registration panics
// when multiple test functions each need a fresh *M instance.
func NewWithRegistry(reg prometheus.Registerer) *M {
	factory := promauto.With(reg)
	return &M{
		KafkaRecordsProcessed: factory.NewCounter(prometheus.CounterOpts{
			Name: "kafka_records_processed_total",
			Help: "Kafka records successfully handed off (CH, overflow, or DLQ).",
		}),
		ClickHouseWriteErrors: factory.NewCounter(prometheus.CounterOpts{
			Name: "clickhouse_write_errors_total",
			Help: "ClickHouse batch insert failures (before overflow/DLQ handoff).",
		}),
		ClickHouseBatchSize: factory.NewHistogram(prometheus.HistogramOpts{
			Name:    "clickhouse_batch_size",
			Help:    "Events per successful ClickHouse batch insert.",
			Buckets: []float64{10, 50, 100, 250, 500, 1000, 5000},
		}),
		ClickHouseFlushDur: factory.NewHistogram(prometheus.HistogramOpts{
			Name:    "clickhouse_flush_duration_seconds",
			Help:    "Wall time for a ClickHouse batch insert attempt.",
			Buckets: prometheus.DefBuckets,
		}),
		CircuitBreakerState: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "circuit_breaker_state",
			Help: "Active circuit breaker state (1=active, 0=inactive).",
		}, []string{"state"}),
		RedisOverflowDepth: factory.NewGauge(prometheus.GaugeOpts{
			Name: "redis_overflow_depth",
			Help: "Redis LIST length for overflow buffer.",
		}),
		DLQEvents: factory.NewCounter(prometheus.CounterOpts{
			Name: "dlq_events_total",
			Help: "Events sent to ai_inference_dlq after insert retries exhausted.",
		}),
		KafkaConsumerLagEvents: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kafka_consumer_lag_events",
			Help: "Approximate unconsumed events (high watermark minus committed offset) per partition.",
		}, []string{"topic", "partition", "group"}),
		KafkaDeserializationErrors: factory.NewCounter(prometheus.CounterOpts{
			Name: "kafka_deserialization_errors_total",
			Help: "Kafka records that failed JSON batch deserialization (offset not committed).",
		}),
		KafkaRecordHandoffErrors: factory.NewCounter(prometheus.CounterOpts{
			Name: "kafka_record_handoff_errors_total",
			Help: "Kafka records where sink.Accept failed (offset not committed).",
		}),
		AnomaliesDetectedTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "anomalies_detected_total",
			Help: "Inference latency anomalies detected via z-score and published to ai_anomalies.",
		}, []string{"tenant_id", "model_id"}),
	}
}

// PartitionLag returns endOffset - committedOffset, floored at zero.
func PartitionLag(endOffset, committedOffset int64) float64 {
	lag := endOffset - committedOffset
	if lag < 0 {
		return 0
	}
	return float64(lag)
}

// SetBreakerState sets exactly one of closed|open|halfopen to 1.
func (m *M) SetBreakerState(state string) {
	for _, s := range []string{"closed", "open", "halfopen"} {
		v := 0.0
		if s == state {
			v = 1
		}
		m.CircuitBreakerState.WithLabelValues(s).Set(v)
	}
}
