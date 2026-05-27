package anomaly

import (
	"math"
	"testing"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/model"
)

func ev(tenant, modelID string, latency uint32, ts uint64) model.InferenceEvent {
	return model.InferenceEvent{
		TenantID:        tenant,
		ModelID:         modelID,
		TimestampUnixMs: ts,
		LatencyMs:       latency,
	}
}

func TestZScoreLatencyDetector_DetectsHighOutlier(t *testing.T) {
	// With windowSize=minSamples=5, the 6th sample is compared against the first 5.
	d := NewZScoreLatencyDetector(2.0, 5, 5)

	latencies := []uint32{100, 102, 98, 101, 99, 130}
	var got []*DetectedAnomaly
	for i, l := range latencies {
		a := d.ObserveEvent(ev("t1", "m1", l, uint64(i)))
		got = append(got, a)
	}

	for i := 0; i < 5; i++ {
		if got[i] != nil {
			t.Fatalf("sample %d unexpectedly flagged: %+v", i, got[i])
		}
	}
	if got[5] == nil {
		t.Fatalf("expected anomaly on sample 5")
	}

	// mean=100, std=sqrt(2) from {100,102,98,101,99}
	wantMean := 100.0
	wantStd := math.Sqrt(2)
	wantZ := (130.0 - wantMean) / wantStd

	if math.Abs(got[5].MeanLatencyMs-wantMean) > 1e-6 {
		t.Fatalf("mean=%f, want %f", got[5].MeanLatencyMs, wantMean)
	}
	if math.Abs(got[5].StdLatencyMs-wantStd) > 1e-6 {
		t.Fatalf("std=%f, want %f", got[5].StdLatencyMs, wantStd)
	}
	if math.Abs(got[5].ZScore-wantZ) > 1e-6 {
		t.Fatalf("z=%f, want %f", got[5].ZScore, wantZ)
	}
}

func TestZScoreLatencyDetector_MinSamplesGate(t *testing.T) {
	// minSamples=3 means sample 3 is still gated (count=2 before adding it).
	d := NewZScoreLatencyDetector(1.0, 5, 3)

	if d.ObserveEvent(ev("t1", "m1", 100, 0)) != nil {
		t.Fatal("unexpected anomaly at sample 0")
	}
	if d.ObserveEvent(ev("t1", "m1", 101, 1)) != nil {
		t.Fatal("unexpected anomaly at sample 1")
	}
	if d.ObserveEvent(ev("t1", "m1", 99, 2)) != nil {
		t.Fatal("unexpected anomaly at sample 2 (minSamples not satisfied yet)")
	}

	// Previous samples {100,101,99}: mean=100, std=sqrt(2/3) ~ 0.816; z ~ 12.25
	if d.ObserveEvent(ev("t1", "m1", 110, 3)) == nil {
		t.Fatal("expected anomaly when minSamples is satisfied")
	}
}

func TestZScoreLatencyDetector_ZeroStdDoesNotFlag(t *testing.T) {
	d := NewZScoreLatencyDetector(0.5, 5, 3)

	// stddev is 0 for {100,100,100} so we should not flag 200 even though z would be huge.
	_ = d.ObserveEvent(ev("t1", "m1", 100, 0))
	_ = d.ObserveEvent(ev("t1", "m1", 100, 1))
	_ = d.ObserveEvent(ev("t1", "m1", 100, 2))

	if d.ObserveEvent(ev("t1", "m1", 200, 3)) != nil {
		t.Fatal("unexpected anomaly when stddev is ~0")
	}
}

func TestZScoreLatencyDetector_IsolatedPerTenantAndModel(t *testing.T) {
	d := NewZScoreLatencyDetector(2.0, 5, 5)

	// Train model m1.
	for i := 0; i < 5; i++ {
		if d.ObserveEvent(ev("t1", "m1", 100+uint32(i%2), uint64(i))) != nil {
			t.Fatal("unexpected anomaly while training")
		}
	}

	// m2 has insufficient samples, so it should not flag yet.
	if d.ObserveEvent(ev("t1", "m2", 130, 6)) != nil {
		t.Fatal("unexpected anomaly for m2 with insufficient samples")
	}

	// Now m1 should be compared against its trained window and flag.
	if d.ObserveEvent(ev("t1", "m1", 130, 7)) == nil {
		t.Fatal("expected anomaly for m1")
	}
}
