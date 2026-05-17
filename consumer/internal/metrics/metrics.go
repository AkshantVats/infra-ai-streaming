package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// M holds consumer Prometheus metrics (minimal Day 5 set).
type M struct {
	KafkaRecordsProcessed prometheus.Counter
	ClickHouseWriteErrors prometheus.Counter
	ClickHouseBatchSize   prometheus.Histogram
	ClickHouseFlushDur    prometheus.Histogram
	CircuitBreakerState   *prometheus.GaugeVec
	RedisOverflowDepth    prometheus.Gauge
	DLQEvents             prometheus.Counter
}

// New registers metrics with the default registry.
func New() *M {
	return &M{
		KafkaRecordsProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "kafka_records_processed_total",
			Help: "Kafka records successfully handed off (CH, overflow, or DLQ).",
		}),
		ClickHouseWriteErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "clickhouse_write_errors_total",
			Help: "ClickHouse batch insert failures (before overflow/DLQ handoff).",
		}),
		ClickHouseBatchSize: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "clickhouse_batch_size",
			Help:    "Events per successful ClickHouse batch insert.",
			Buckets: []float64{10, 50, 100, 250, 500, 1000, 5000},
		}),
		ClickHouseFlushDur: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "clickhouse_flush_duration_seconds",
			Help:    "Wall time for a ClickHouse batch insert attempt.",
			Buckets: prometheus.DefBuckets,
		}),
		CircuitBreakerState: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "circuit_breaker_state",
			Help: "Active circuit breaker state (1=active, 0=inactive).",
		}, []string{"state"}),
		RedisOverflowDepth: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "redis_overflow_depth",
			Help: "Redis LIST length for overflow buffer.",
		}),
		DLQEvents: promauto.NewCounter(prometheus.CounterOpts{
			Name: "dlq_events_total",
			Help: "Events sent to ai_inference_dlq after insert retries exhausted.",
		}),
	}
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
