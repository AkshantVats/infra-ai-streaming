package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/anomaly"
	"github.com/akshantvats/infra-ai-streaming/consumer/internal/clickhouse"
	"github.com/akshantvats/infra-ai-streaming/consumer/internal/config"
	"github.com/akshantvats/infra-ai-streaming/consumer/internal/kafka"
	"github.com/akshantvats/infra-ai-streaming/consumer/internal/metrics"
	redisoverflow "github.com/akshantvats/infra-ai-streaming/consumer/internal/redis"
)

func main() {
	cfg := config.LoadFromEnv()
	m := metrics.New()
	metrics.StartServer(cfg.MetricsPort)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	overflow, err := redisoverflow.NewListOverflow(ctx, cfg.RedisURL, cfg.OverflowKey, m)
	if err != nil {
		log.Fatalf("level=fatal msg=redis_init_failed err=%v", err)
	}
	defer overflow.Close()

	dlq, err := kafka.NewDLQPublisher(cfg.KafkaBrokers, cfg.KafkaDLQTopic, m)
	if err != nil {
		log.Fatalf("level=fatal msg=dlq_init_failed err=%v", err)
	}
	defer dlq.Close()

	detector := anomaly.NewZScoreLatencyDetector(
		cfg.AnomalyZScoreThreshold,
		cfg.AnomalyWindowSize,
		cfg.AnomalyMinSamples,
	)
	anomalyPublisher, err := kafka.NewAnomalyPublisher(cfg.KafkaBrokers, cfg.KafkaAnomaliesTopic, m)
	if err != nil {
		log.Fatalf("level=fatal msg=anomalies_init_failed err=%v", err)
	}
	defer anomalyPublisher.Close()

	writer, err := clickhouse.NewBatchWriter(ctx, cfg, overflow, dlq, m)
	if err != nil {
		log.Fatalf("level=fatal msg=clickhouse_init_failed err=%v", err)
	}
	defer writer.Close()

	go writer.Start(ctx)

	reader, err := kafka.NewReader(cfg, writer, m, detector, anomalyPublisher)
	if err != nil {
		log.Fatalf("level=fatal msg=consumer_init_failed err=%v", err)
	}
	defer reader.Close()

	if err := reader.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("level=fatal msg=consumer_stopped err=%v", err)
	}
	log.Printf("level=info msg=consumer_shutdown")
}
