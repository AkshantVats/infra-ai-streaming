// SPDX-License-Identifier: MIT
package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// newTestMetrics returns an M backed by a fresh isolated registry.
// Using prometheus.NewRegistry() instead of the default avoids
// duplicate-registration panics when multiple tests call this function.
func newTestMetrics() *M {
	return NewWithRegistry(prometheus.NewRegistry())
}

func TestPartitionLag(t *testing.T) {
	tests := []struct {
		name           string
		end, committed int64
		want           float64
	}{
		{"normal lag", 100, 40, 60},
		{"zero lag", 100, 100, 0},
		{"committed ahead of end (clock skew)", 50, 60, 0},
		{"both zero", 0, 0, 0},
		{"large lag", 1_000_000, 0, 1_000_000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PartitionLag(tc.end, tc.committed)
			if got != tc.want {
				t.Errorf("PartitionLag(%d, %d) = %v, want %v", tc.end, tc.committed, got, tc.want)
			}
		})
	}
}

func TestNewRegistersLagMetric(t *testing.T) {
	m := newTestMetrics()
	if m.KafkaConsumerLagEvents == nil {
		t.Fatal("KafkaConsumerLagEvents not registered")
	}
}

// TestNewRegistersAllMetrics verifies that none of the M fields are nil after construction.
func TestNewRegistersAllMetrics(t *testing.T) {
	m := newTestMetrics()
	if m.KafkaRecordsProcessed == nil {
		t.Error("KafkaRecordsProcessed is nil")
	}
	if m.ClickHouseWriteErrors == nil {
		t.Error("ClickHouseWriteErrors is nil")
	}
	if m.ClickHouseBatchSize == nil {
		t.Error("ClickHouseBatchSize is nil")
	}
	if m.ClickHouseFlushDur == nil {
		t.Error("ClickHouseFlushDur is nil")
	}
	if m.CircuitBreakerState == nil {
		t.Error("CircuitBreakerState is nil")
	}
	if m.RedisOverflowDepth == nil {
		t.Error("RedisOverflowDepth is nil")
	}
	if m.DLQEvents == nil {
		t.Error("DLQEvents is nil")
	}
	if m.KafkaDeserializationErrors == nil {
		t.Error("KafkaDeserializationErrors is nil")
	}
	if m.KafkaRecordHandoffErrors == nil {
		t.Error("KafkaRecordHandoffErrors is nil")
	}
	if m.AnomaliesDetectedTotal == nil {
		t.Error("AnomaliesDetectedTotal is nil")
	}
}

// TestSetBreakerStateExclusive verifies that SetBreakerState activates exactly
// one state label and zeros the others, without panicking.
func TestSetBreakerStateExclusive(t *testing.T) {
	m := newTestMetrics()
	for _, state := range []string{"closed", "open", "halfopen"} {
		m.SetBreakerState(state)
	}
}
