// SPDX-License-Identifier: MIT
package traceforge

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

var (
	httpClient = &http.Client{Timeout: 2 * time.Second}
	kafkaOnce  sync.Once
	kafkaClient *kgo.Client
)

func collectorURL() string {
	u := os.Getenv("TRACEFORGE_COLLECTOR_URL")
	if u == "" {
		u = "http://localhost:8080/v1/spans"
	}
	return u
}

func kafkaBrokers() []string {
	raw := os.Getenv("TRACEFORGE_KAFKA_BROKERS")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

func kafkaTopic() string {
	t := os.Getenv("TRACEFORGE_KAFKA_TOPIC")
	if t == "" {
		t = "agent-spans"
	}
	return t
}

// emit sends span over HTTP to the collector and, if Kafka is configured, also
// produces to the agent-spans topic. Both sends are fire-and-forget: failures
// are silently dropped to avoid blocking the caller.
func emit(ctx context.Context, s *Span) {
	payload, err := json.Marshal(s)
	if err != nil {
		return
	}
	go sendHTTP(payload)
	if brokers := kafkaBrokers(); len(brokers) > 0 {
		go sendKafka(ctx, brokers, payload)
	}
}

func sendHTTP(payload []byte) {
	req, err := http.NewRequest(http.MethodPost, collectorURL(), bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func sendKafka(ctx context.Context, brokers []string, payload []byte) {
	kafkaOnce.Do(func() {
		cl, err := kgo.NewClient(
			kgo.SeedBrokers(brokers...),
			kgo.DefaultProduceTopic(kafkaTopic()),
		)
		if err == nil {
			kafkaClient = cl
		}
	})
	if kafkaClient == nil {
		return
	}
	kafkaClient.ProduceSync(ctx, &kgo.Record{Value: payload})
}
