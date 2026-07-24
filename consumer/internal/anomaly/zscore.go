// SPDX-License-Identifier: MIT
// Package anomaly detects latency outliers for the ai_anomalies topic.
package anomaly

import (
	"math"
	"sync"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/model"
)

// LatencyKey scopes anomaly detection to a particular tenant + model.
type LatencyKey struct {
	TenantID string
	ModelID  string
}

// DetectedAnomaly captures the z-score computation context for an outlier latency.
type DetectedAnomaly struct {
	TenantID        string
	ModelID         string
	EventID         *string
	TimestampUnixMs uint64
	LatencyMs       uint32

	ZScore        float64
	MeanLatencyMs float64
	StdLatencyMs  float64
}

type windowStats struct {
	values []float64
	idx    int // next index to overwrite
	count  int // number of items observed (<= len(values))
	sum    float64
	sumsq  float64
}

// ZScoreLatencyDetector detects high-latency anomalies using a rolling window.
//
// The z-score is computed against the *previous* window (before adding the current sample).
type ZScoreLatencyDetector struct {
	mu sync.Mutex

	threshold  float64
	windowSize int
	minSamples int
	epsilon    float64

	stats map[LatencyKey]*windowStats
}

func NewZScoreLatencyDetector(threshold float64, windowSize int, minSamples int) *ZScoreLatencyDetector {
	if windowSize < 2 {
		windowSize = 2
	}
	if minSamples < 2 {
		minSamples = 2
	}
	if minSamples > windowSize {
		minSamples = windowSize
	}
	if threshold <= 0 {
		threshold = 3.0
	}

	return &ZScoreLatencyDetector{
		threshold:  threshold,
		windowSize: windowSize,
		minSamples: minSamples,
		epsilon:    1e-9,
		stats:      make(map[LatencyKey]*windowStats),
	}
}

func (d *ZScoreLatencyDetector) ObserveEvent(e model.InferenceEvent) *DetectedAnomaly {
	key := LatencyKey{TenantID: e.TenantID, ModelID: e.ModelID}
	x := float64(e.LatencyMs)

	d.mu.Lock()
	defer d.mu.Unlock()

	s, ok := d.stats[key]
	if !ok {
		s = &windowStats{values: make([]float64, d.windowSize)}
		d.stats[key] = s
	}

	// Check anomaly against existing window.
	var anomaly *DetectedAnomaly
	if s.count >= d.minSamples {
		mean := s.sum / float64(s.count)
		// population variance: E[x^2] - (E[x])^2
		variance := (s.sumsq/float64(s.count) - mean*mean)
		if variance < 0 {
			variance = 0
		}
		std := math.Sqrt(variance)
		if std > d.epsilon {
			z := (x - mean) / std
			if z >= d.threshold {
				anomaly = &DetectedAnomaly{
					TenantID:        e.TenantID,
					ModelID:         e.ModelID,
					EventID:         e.EventID,
					TimestampUnixMs: e.TimestampUnixMs,
					LatencyMs:       e.LatencyMs,
					ZScore:          z,
					MeanLatencyMs:   mean,
					StdLatencyMs:    std,
				}
			}
		}
	}

	// Update rolling window with the new sample.
	if s.count < d.windowSize {
		s.values[s.idx] = x
		s.sum += x
		s.sumsq += x * x
		s.count++
		s.idx = (s.idx + 1) % d.windowSize
		return anomaly
	}

	old := s.values[s.idx]
	s.sum -= old
	s.sumsq -= old * old
	s.values[s.idx] = x
	s.sum += x
	s.sumsq += x * x
	s.idx = (s.idx + 1) % d.windowSize
	return anomaly
}
