package metrics

import "testing"

func TestPartitionLag(t *testing.T) {
	tests := []struct {
		end, committed int64
		want           float64
	}{
		{100, 40, 60},
		{100, 100, 0},
		{50, 60, 0},
	}
	for _, tc := range tests {
		got := PartitionLag(tc.end, tc.committed)
		if got != tc.want {
			t.Errorf("PartitionLag(%d, %d) = %v, want %v", tc.end, tc.committed, got, tc.want)
		}
	}
}

func TestNewRegistersLagMetric(t *testing.T) {
	m := New()
	if m.KafkaConsumerLagEvents == nil {
		t.Fatal("KafkaConsumerLagEvents not registered")
	}
}
