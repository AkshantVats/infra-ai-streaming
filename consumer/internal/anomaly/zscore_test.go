// SPDX-License-Identifier: MIT
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

// TestZScoreLatencyDetector_NegativeZScoreNotFlagged verifies that a latency
// well below the mean (negative z-score) is never reported as an anomaly.
func TestZScoreLatencyDetector_NegativeZScoreNotFlagged(t *testing.T) {
	d := NewZScoreLatencyDetector(2.0, 5, 5)
	for i := 0; i < 5; i++ {
		d.ObserveEvent(ev("t1", "m1", 200, uint64(i)))
	}
	// Latency of 1ms is far below the mean of 200ms; z-score is large and negative.
	if d.ObserveEvent(ev("t1", "m1", 1, 5)) != nil {
		t.Fatal("unexpectedly flagged a latency far below the mean")
	}
}

// TestZScoreLatencyDetector_RollingWindowEviction verifies that after filling
// a window of size 3, old samples are evicted and anomaly detection uses only
// the current window.
func TestZScoreLatencyDetector_RollingWindowEviction(t *testing.T) {
	// Window size 3, min samples 3.
	d := NewZScoreLatencyDetector(2.0, 3, 3)

	// Fill: {1000, 1001, 999} → mean≈1000, very small std.
	d.ObserveEvent(ev("t1", "m1", 1000, 0))
	d.ObserveEvent(ev("t1", "m1", 1001, 1))
	d.ObserveEvent(ev("t1", "m1", 999, 2))

	// Evict 1000, add 1000 → window {1001, 999, 1000}, still near 1000.
	// Then add another near-1000 — no anomaly expected.
	if d.ObserveEvent(ev("t1", "m1", 1000, 3)) != nil {
		t.Fatal("unexpected anomaly while rolling near-baseline samples")
	}
}

// TestZScoreLatencyDetector_AnomalyFieldsPopulated verifies that all fields
// of DetectedAnomaly are correctly populated when an anomaly is detected.
func TestZScoreLatencyDetector_AnomalyFieldsPopulated(t *testing.T) {
	// Use a varied window so std > 0, enabling z-score computation.
	// {100, 102, 98, 101, 99} → mean=100, std=sqrt(2) ≈ 1.414
	d := NewZScoreLatencyDetector(2.0, 5, 5)
	latencies := []uint32{100, 102, 98, 101, 99}
	for i, l := range latencies {
		d.ObserveEvent(ev("t1", "m1", l, uint64(i)))
	}
	// z-score for 130 is (130-100)/sqrt(2) ≈ 21.2, well above threshold 2.0.
	evID := "my-event-id"
	a := d.ObserveEvent(model.InferenceEvent{
		TenantID:        "t1",
		ModelID:         "m1",
		EventID:         &evID,
		TimestampUnixMs: 99,
		LatencyMs:       130,
	})
	if a == nil {
		t.Fatal("expected anomaly")
	}
	if a.TenantID != "t1" {
		t.Errorf("TenantID = %q", a.TenantID)
	}
	if a.ModelID != "m1" {
		t.Errorf("ModelID = %q", a.ModelID)
	}
	if a.EventID == nil || *a.EventID != evID {
		t.Errorf("EventID = %v", a.EventID)
	}
	if a.TimestampUnixMs != 99 {
		t.Errorf("TimestampUnixMs = %d", a.TimestampUnixMs)
	}
	if a.LatencyMs != 130 {
		t.Errorf("LatencyMs = %d", a.LatencyMs)
	}
	if a.ZScore <= 0 {
		t.Errorf("ZScore = %f, want > 0", a.ZScore)
	}
}

// TestZScoreLatencyDetector_ConstructorClamping verifies that invalid
// constructor arguments are clamped to safe minimums.
func TestZScoreLatencyDetector_ConstructorClamping(t *testing.T) {
	// windowSize < 2 → clamped to 2
	// minSamples < 2 → clamped to 2
	// threshold <= 0 → clamped to 3.0
	d := NewZScoreLatencyDetector(0, 1, 1)
	if d.windowSize != 2 {
		t.Errorf("windowSize = %d, want 2 after clamping", d.windowSize)
	}
	if d.minSamples != 2 {
		t.Errorf("minSamples = %d, want 2 after clamping", d.minSamples)
	}
	if d.threshold != 3.0 {
		t.Errorf("threshold = %f, want 3.0 after clamping", d.threshold)
	}
}

// TestZScoreLatencyDetector_MinSamplesClampedToWindowSize verifies that if
// minSamples > windowSize, minSamples is clamped down to windowSize.
func TestZScoreLatencyDetector_MinSamplesClampedToWindowSize(t *testing.T) {
	d := NewZScoreLatencyDetector(2.0, 5, 10)
	if d.minSamples != d.windowSize {
		t.Errorf("minSamples = %d, want %d (== windowSize)", d.minSamples, d.windowSize)
	}
}

// TestZScoreLatencyDetector_ThresholdInclusive verifies z >= threshold triggers an anomaly.
func TestZScoreLatencyDetector_ThresholdInclusive(t *testing.T) {
	d := NewZScoreLatencyDetector(2.0, 5, 5)
	latencies := []uint32{100, 102, 98, 101, 99}
	for i, l := range latencies {
		d.ObserveEvent(ev("t1", "m1", l, uint64(i)))
	}
	// z for 102 vs mean=100, std=sqrt(2) is sqrt(2) < 2.0 — below threshold.
	if a := d.ObserveEvent(ev("t1", "m1", 102, 5)); a != nil {
		t.Fatalf("z=%.4f unexpectedly flagged (threshold 2.0)", a.ZScore)
	}
	// 130 is far above mean — z >> 2.0 (same window as AnomalyFieldsPopulated).
	a := d.ObserveEvent(ev("t1", "m1", 130, 6))
	if a == nil {
		t.Fatal("expected anomaly at z >= threshold")
	}
	if a.ZScore < 2.0 {
		t.Fatalf("ZScore = %.4f, want >= 2.0", a.ZScore)
	}
}
