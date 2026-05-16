package config

import (
	"os"
	"strings"
)

// Config holds consumer runtime settings (mirrors Rust env names where shared).
type Config struct {
	KafkaBrokers string
	KafkaTopic   string
	KafkaGroupID string
	LogLevel     string
}

// LoadFromEnv reads configuration from environment variables with defaults for local dev.
func LoadFromEnv() Config {
	return Config{
		KafkaBrokers: envOr("KAFKA_BROKERS", "127.0.0.1:9092"),
		KafkaTopic:   envOr("KAFKA_TOPIC", "ai_inference_events"),
		KafkaGroupID: envOr("KAFKA_GROUP_ID", "ai-inference-consumer-dev"),
		LogLevel:     envOr("LOG_LEVEL", "info"),
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
