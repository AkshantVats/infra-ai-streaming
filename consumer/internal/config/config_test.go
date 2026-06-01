package config

import (
	"os"
	"testing"
	"time"
)

// setenv sets an env var for the duration of a test and restores it on cleanup.
func setenv(t *testing.T, key, value string) {
	t.Helper()
	old, hadOld := os.LookupEnv(key)
	os.Setenv(key, value)
	t.Cleanup(func() {
		if hadOld {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})
}

func TestLoadFromEnvDefaults(t *testing.T) {
	// Unset any env vars that might bleed in from the shell.
	for _, k := range []string{
		"KAFKA_BROKERS", "KAFKA_TOPIC", "KAFKA_GROUP_ID", "KAFKA_DLQ_TOPIC",
		"KAFKA_ANOMALIES_TOPIC", "REDIS_URL", "REDIS_OVERFLOW_KEY",
		"CLICKHOUSE_DSN", "METRICS_PORT", "LOG_LEVEL", "BATCH_SIZE",
		"FLUSH_INTERVAL", "CB_FAILURE_THRESHOLD", "CB_RESET_TIMEOUT",
		"CLICKHOUSE_INSERT_RETRIES", "OVERFLOW_DRAIN_INTERVAL",
		"OVERFLOW_DRAIN_BATCH", "ANOMALY_Z_THRESHOLD", "ANOMALY_ZSCORE_THRESHOLD",
		"ANOMALY_WINDOW_SIZE", "ANOMALY_MIN_SAMPLES",
	} {
		old, hadOld := os.LookupEnv(k)
		os.Unsetenv(k)
		k2, old2, hadOld2 := k, old, hadOld // capture loop vars
		t.Cleanup(func() {
			if hadOld2 {
				os.Setenv(k2, old2)
			}
		})
	}

	cfg := LoadFromEnv()

	if cfg.KafkaBrokers != "127.0.0.1:9092" {
		t.Errorf("KafkaBrokers = %q, want default", cfg.KafkaBrokers)
	}
	if cfg.KafkaTopic != "ai_inference_events" {
		t.Errorf("KafkaTopic = %q", cfg.KafkaTopic)
	}
	if cfg.KafkaGroupID != "ai-inference-consumer-dev" {
		t.Errorf("KafkaGroupID = %q", cfg.KafkaGroupID)
	}
	if cfg.KafkaDLQTopic != "ai_inference_dlq" {
		t.Errorf("KafkaDLQTopic = %q", cfg.KafkaDLQTopic)
	}
	if cfg.KafkaAnomaliesTopic != "ai_anomalies" {
		t.Errorf("KafkaAnomaliesTopic = %q", cfg.KafkaAnomaliesTopic)
	}
	if cfg.RedisURL != "redis://127.0.0.1:6379" {
		t.Errorf("RedisURL = %q", cfg.RedisURL)
	}
	if cfg.OverflowKey != "ai_inference:overflow" {
		t.Errorf("OverflowKey = %q", cfg.OverflowKey)
	}
	if cfg.ClickHouseDSN != "clickhouse://127.0.0.1:9000/infra_ai" {
		t.Errorf("ClickHouseDSN = %q", cfg.ClickHouseDSN)
	}
	if cfg.MetricsPort != 9091 {
		t.Errorf("MetricsPort = %d, want 9091", cfg.MetricsPort)
	}
	if cfg.BatchSize != 1000 {
		t.Errorf("BatchSize = %d, want 1000", cfg.BatchSize)
	}
	if cfg.FlushInterval != 500*time.Millisecond {
		t.Errorf("FlushInterval = %v", cfg.FlushInterval)
	}
	if cfg.CBFailures != 5 {
		t.Errorf("CBFailures = %d, want 5", cfg.CBFailures)
	}
	if cfg.CBResetTimeout != 30*time.Second {
		t.Errorf("CBResetTimeout = %v", cfg.CBResetTimeout)
	}
	if cfg.InsertRetries != 3 {
		t.Errorf("InsertRetries = %d, want 3", cfg.InsertRetries)
	}
	if cfg.DrainInterval != 5*time.Second {
		t.Errorf("DrainInterval = %v", cfg.DrainInterval)
	}
	if cfg.DrainBatchSize != 5000 {
		t.Errorf("DrainBatchSize = %d, want 5000", cfg.DrainBatchSize)
	}
	if cfg.AnomalyZScoreThreshold != 3.0 {
		t.Errorf("AnomalyZScoreThreshold = %v, want 3.0", cfg.AnomalyZScoreThreshold)
	}
	if cfg.AnomalyWindowSize != 100 {
		t.Errorf("AnomalyWindowSize = %d, want 100", cfg.AnomalyWindowSize)
	}
	if cfg.AnomalyMinSamples != 20 {
		t.Errorf("AnomalyMinSamples = %d, want 20", cfg.AnomalyMinSamples)
	}
}

func TestLoadFromEnvOverrides(t *testing.T) {
	setenv(t, "KAFKA_BROKERS", "broker1:9092,broker2:9092")
	setenv(t, "KAFKA_TOPIC", "custom_topic")
	setenv(t, "METRICS_PORT", "8080")
	setenv(t, "BATCH_SIZE", "500")
	setenv(t, "FLUSH_INTERVAL", "1s")
	setenv(t, "CB_FAILURE_THRESHOLD", "10")
	setenv(t, "CB_RESET_TIMEOUT", "1m")
	setenv(t, "ANOMALY_Z_THRESHOLD", "2.5")
	setenv(t, "ANOMALY_WINDOW_SIZE", "50")
	setenv(t, "ANOMALY_MIN_SAMPLES", "10")

	cfg := LoadFromEnv()

	if cfg.KafkaBrokers != "broker1:9092,broker2:9092" {
		t.Errorf("KafkaBrokers = %q", cfg.KafkaBrokers)
	}
	if cfg.KafkaTopic != "custom_topic" {
		t.Errorf("KafkaTopic = %q", cfg.KafkaTopic)
	}
	if cfg.MetricsPort != 8080 {
		t.Errorf("MetricsPort = %d", cfg.MetricsPort)
	}
	if cfg.BatchSize != 500 {
		t.Errorf("BatchSize = %d", cfg.BatchSize)
	}
	if cfg.FlushInterval != time.Second {
		t.Errorf("FlushInterval = %v", cfg.FlushInterval)
	}
	if cfg.CBFailures != 10 {
		t.Errorf("CBFailures = %d", cfg.CBFailures)
	}
	if cfg.CBResetTimeout != time.Minute {
		t.Errorf("CBResetTimeout = %v", cfg.CBResetTimeout)
	}
	if cfg.AnomalyZScoreThreshold != 2.5 {
		t.Errorf("AnomalyZScoreThreshold = %v", cfg.AnomalyZScoreThreshold)
	}
	if cfg.AnomalyWindowSize != 50 {
		t.Errorf("AnomalyWindowSize = %d", cfg.AnomalyWindowSize)
	}
	if cfg.AnomalyMinSamples != 10 {
		t.Errorf("AnomalyMinSamples = %d", cfg.AnomalyMinSamples)
	}
}

func TestEnvOrFallsBackOnEmpty(t *testing.T) {
	const key = "TEST_ENV_OR_EMPTY_KEY_XYZABC"
	os.Unsetenv(key)
	defer os.Unsetenv(key)

	if got := envOr(key, "fallback"); got != "fallback" {
		t.Errorf("envOr with unset key = %q, want fallback", got)
	}

	// Whitespace-only value should also fall back.
	os.Setenv(key, "   ")
	if got := envOr(key, "fallback"); got != "fallback" {
		t.Errorf("envOr with whitespace value = %q, want fallback", got)
	}

	os.Setenv(key, "actual")
	if got := envOr(key, "fallback"); got != "actual" {
		t.Errorf("envOr with set key = %q, want actual", got)
	}
}

func TestEnvIntOrInvalidFallsBack(t *testing.T) {
	const key = "TEST_ENV_INT_OR_XYZABC"
	os.Setenv(key, "not_an_int")
	defer os.Unsetenv(key)

	if got := envIntOr(key, 42); got != 42 {
		t.Errorf("envIntOr with invalid int = %d, want 42", got)
	}
}

func TestEnvFloatOrInvalidFallsBack(t *testing.T) {
	const key = "TEST_ENV_FLOAT_OR_XYZABC"
	os.Setenv(key, "nan_not_a_float")
	defer os.Unsetenv(key)

	if got := envFloatOr(key, 1.5); got != 1.5 {
		t.Errorf("envFloatOr with invalid float = %v, want 1.5", got)
	}
}

func TestEnvDurationOrInvalidFallsBack(t *testing.T) {
	const key = "TEST_ENV_DUR_OR_XYZABC"
	os.Setenv(key, "not_a_duration")
	defer os.Unsetenv(key)

	if got := envDurationOr(key, 5*time.Second); got != 5*time.Second {
		t.Errorf("envDurationOr with invalid duration = %v, want 5s", got)
	}
}

// TestAnomalyZScoreThresholdFallbackAlias ensures ANOMALY_ZSCORE_THRESHOLD (legacy)
// is respected when the primary key is absent.
func TestAnomalyZScoreThresholdFallbackAlias(t *testing.T) {
	setenv(t, "ANOMALY_ZSCORE_THRESHOLD", "4.0")
	// Ensure primary key is absent.
	old, hadOld := os.LookupEnv("ANOMALY_Z_THRESHOLD")
	os.Unsetenv("ANOMALY_Z_THRESHOLD")
	t.Cleanup(func() {
		if hadOld {
			os.Setenv("ANOMALY_Z_THRESHOLD", old)
		}
	})

	cfg := LoadFromEnv()
	if cfg.AnomalyZScoreThreshold != 4.0 {
		t.Errorf("AnomalyZScoreThreshold via alias = %v, want 4.0", cfg.AnomalyZScoreThreshold)
	}
}
