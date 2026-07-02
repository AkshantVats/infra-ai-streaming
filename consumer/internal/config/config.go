// SPDX-License-Identifier: MIT
// Package config loads consumer settings from environment variables.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds consumer runtime settings (mirrors Rust env names where shared).
type Config struct {
	KafkaBrokers        string
	KafkaTopic          string
	KafkaGroupID        string
	KafkaDLQTopic       string
	KafkaAnomaliesTopic string
	RedisURL            string
	OverflowKey         string
	ClickHouseDSN       string
	MetricsPort         int
	LogLevel            string
	BatchSize           int
	FlushInterval       time.Duration
	CBFailures          int
	CBResetTimeout      time.Duration
	InsertRetries       int
	DrainInterval       time.Duration
	DrainBatchSize      int

	AnomalyZScoreThreshold float64
	AnomalyWindowSize      int
	AnomalyMinSamples      int
}

// LoadFromEnv reads configuration from environment variables with defaults for local dev.
func LoadFromEnv() Config {
	return Config{
		KafkaBrokers:        envOr("KAFKA_BROKERS", "127.0.0.1:9092"),
		KafkaTopic:          envOr("KAFKA_TOPIC", "ai_inference_events"),
		KafkaGroupID:        envOr("KAFKA_GROUP_ID", "ai-inference-consumer-dev"),
		KafkaDLQTopic:       envOr("KAFKA_DLQ_TOPIC", "ai_inference_dlq"),
		KafkaAnomaliesTopic: envOr("KAFKA_ANOMALIES_TOPIC", "ai_anomalies"),
		RedisURL:            envOr("REDIS_URL", "redis://127.0.0.1:6379"),
		OverflowKey:         envOr("REDIS_OVERFLOW_KEY", "ai_inference:overflow"),
		ClickHouseDSN:       envOr("CLICKHOUSE_DSN", "clickhouse://127.0.0.1:9000/infra_ai"),
		MetricsPort:         envIntOr("METRICS_PORT", 9091),
		LogLevel:            envOr("LOG_LEVEL", "info"),
		BatchSize:           envIntOr("BATCH_SIZE", 1000),
		FlushInterval:       envDurationOr("FLUSH_INTERVAL", 500*time.Millisecond),
		CBFailures:          envIntOr("CB_FAILURE_THRESHOLD", 5),
		CBResetTimeout:      envDurationOr("CB_RESET_TIMEOUT", 30*time.Second),
		InsertRetries:       envIntOr("CLICKHOUSE_INSERT_RETRIES", 3),
		DrainInterval:       envDurationOr("OVERFLOW_DRAIN_INTERVAL", 5*time.Second),
		DrainBatchSize:      envIntOr("OVERFLOW_DRAIN_BATCH", 5000),

		AnomalyZScoreThreshold: envFloatOr("ANOMALY_Z_THRESHOLD", envFloatOr("ANOMALY_ZSCORE_THRESHOLD", 3.0)),
		AnomalyWindowSize:      envIntOr("ANOMALY_WINDOW_SIZE", 100),
		AnomalyMinSamples:      envIntOr("ANOMALY_MIN_SAMPLES", 20),
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envFloatOr(key string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return n
}

func envDurationOr(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
