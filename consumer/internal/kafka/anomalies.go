package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/anomaly"
	"github.com/akshantvats/infra-ai-streaming/consumer/internal/metrics"
)

// AnomalyMessage is the Kafka payload published to ai_anomalies.
type AnomalyMessage struct {
	EventID         *string `json:"event_id,omitempty"`
	TenantID        string  `json:"tenant_id"`
	ModelID         string  `json:"model_id"`
	TimestampUnixMs uint64  `json:"timestamp_unix_ms"`
	LatencyMs       uint32  `json:"latency_ms"`

	// z-score computed against the previous rolling window.
	ZScore        float64 `json:"z_score"`
	MeanLatencyMs float64 `json:"mean_latency_ms"`
	StdLatencyMs  float64 `json:"std_latency_ms"`

	DetectedAtUnixMs int64 `json:"detected_at_unix_ms"`
}

// AnomalyPublisher publishes detected anomalies to a Kafka topic.
type AnomalyPublisher struct {
	client *kgo.Client
	topic  string
	m      *metrics.M
}

func NewAnomalyPublisher(brokers, topic string, m *metrics.M) (*AnomalyPublisher, error) {
	seed := strings.Split(brokers, ",")
	for i := range seed {
		seed[i] = strings.TrimSpace(seed[i])
	}

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(seed...),
		kgo.DefaultProduceTopic(topic),
	)
	if err != nil {
		return nil, fmt.Errorf("create anomalies producer: %w", err)
	}

	return &AnomalyPublisher{client: cl, topic: topic, m: m}, nil
}

func (p *AnomalyPublisher) Publish(ctx context.Context, anomalies []*anomaly.DetectedAnomaly) error {
	if len(anomalies) == 0 {
		return nil
	}

	now := time.Now().UnixMilli()
	recs := make([]*kgo.Record, 0, len(anomalies))
	for _, a := range anomalies {
		msg := AnomalyMessage{
			EventID:          a.EventID,
			TenantID:         a.TenantID,
			ModelID:          a.ModelID,
			TimestampUnixMs:  a.TimestampUnixMs,
			LatencyMs:        a.LatencyMs,
			ZScore:           a.ZScore,
			MeanLatencyMs:    a.MeanLatencyMs,
			StdLatencyMs:     a.StdLatencyMs,
			DetectedAtUnixMs: now,
		}
		body, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("marshal anomaly message: %w", err)
		}
		recs = append(recs, &kgo.Record{Topic: p.topic, Value: body})
	}

	res := p.client.ProduceSync(ctx, recs...)
	if err := res.FirstErr(); err != nil {
		return fmt.Errorf("anomalies produce: %w", err)
	}

	p.m.AnomaliesDetectedTotal.Add(float64(len(anomalies)))
	return nil
}

func (p *AnomalyPublisher) Close() {
	p.client.Close()
}
