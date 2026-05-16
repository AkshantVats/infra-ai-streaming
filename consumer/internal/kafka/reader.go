package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/config"
	"github.com/akshantvats/infra-ai-streaming/consumer/internal/model"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Reader consumes batched inference events from Kafka and logs each event to stdout.
type Reader struct {
	client *kgo.Client
	topic  string
}

// NewReader builds a franz-go consumer for the configured topic and group.
func NewReader(cfg config.Config) (*Reader, error) {
	brokers := strings.Split(cfg.KafkaBrokers, ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(cfg.KafkaGroupID),
		kgo.ConsumeTopics(cfg.KafkaTopic),
		kgo.DisableAutoCommit(),
		// New groups join at the log end; change KAFKA_GROUP_ID to replay from earliest.
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka client: %w", err)
	}

	return &Reader{client: cl, topic: cfg.KafkaTopic}, nil
}

// Run polls records until ctx is cancelled.
func (r *Reader) Run(ctx context.Context) error {
	log.Printf("level=info msg=consumer_started topic=%s", r.topic)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fetches := r.client.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, ferr := range errs {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				log.Printf("level=error msg=fetch_partition_failed topic=%s partition=%d err=%v",
					ferr.Topic, ferr.Partition, ferr.Err)
			}
			continue
		}

		var toCommit []*kgo.Record
		fetches.EachPartition(func(p kgo.FetchTopicPartition) {
			for _, rec := range p.Records {
				if err := r.handleRecord(ctx, rec); err != nil {
					log.Printf("level=error msg=record_failed topic=%s partition=%d offset=%d err=%v",
						rec.Topic, rec.Partition, rec.Offset, err)
					continue
				}
				toCommit = append(toCommit, rec)
			}
		})
		if len(toCommit) > 0 {
			if err := r.client.CommitRecords(ctx, toCommit...); err != nil {
				log.Printf("level=error msg=commit_failed count=%d err=%v", len(toCommit), err)
			}
		}
	}
}

func (r *Reader) handleRecord(ctx context.Context, rec *kgo.Record) error {
	_ = ctx
	batch, err := DeserializeBatch(rec.Value)
	if err != nil {
		return err
	}
	for _, event := range batch.Events {
		LogEvent(event)
	}
	return nil
}

// DeserializeBatch parses the Rust producer JSON envelope {"events":[...]}.
func DeserializeBatch(payload []byte) (model.IngestBatch, error) {
	var batch model.IngestBatch
	if err := json.Unmarshal(payload, &batch); err != nil {
		return model.IngestBatch{}, fmt.Errorf("unmarshal ingest batch: %w", err)
	}
	return batch, nil
}

// LogEvent prints the blog-stable stdout format for each consumed event.
func LogEvent(e model.InferenceEvent) {
	log.Printf(
		"level=info msg=event_consumed tenant_id=%s model_id=%s prompt_tokens=%d completion_tokens=%d cost_usd=%g latency_ms=%d",
		e.TenantID,
		e.ModelID,
		e.PromptTokens,
		e.CompletionTokens,
		e.CostUSD,
		e.LatencyMs,
	)
}

// Close releases the underlying Kafka client.
func (r *Reader) Close() {
	r.client.Close()
}
