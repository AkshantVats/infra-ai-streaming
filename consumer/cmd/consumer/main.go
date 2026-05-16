package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/config"
	"github.com/akshantvats/infra-ai-streaming/consumer/internal/kafka"
)

func main() {
	cfg := config.LoadFromEnv()

	reader, err := kafka.NewReader(cfg)
	if err != nil {
		log.Fatalf("level=fatal msg=consumer_init_failed err=%v", err)
	}
	defer reader.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := reader.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("level=fatal msg=consumer_stopped err=%v", err)
	}
	log.Printf("level=info msg=consumer_shutdown")
}
