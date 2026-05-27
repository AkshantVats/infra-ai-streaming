package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/anomaly"
	"github.com/akshantvats/infra-ai-streaming/consumer/internal/config"
	"github.com/akshantvats/infra-ai-streaming/consumer/internal/metrics"
	"github.com/akshantvats/infra-ai-streaming/consumer/internal/model"
)

// EventSink accepts inference events and blocks until handoff (CH, overflow, or DLQ).
type EventSink interface {
	Accept(ctx context.Context, events []model.InferenceEvent) error
}

// Reader consumes batched inference events from Kafka and forwards them to the sink.
type Reader struct {
	client  *kgo.Client
	topic   string
	groupID string
	sink    EventSink
	m       *metrics.M

	detector         *anomaly.ZScoreLatencyDetector
	anomalyPublisher *AnomalyPublisher
}

// NewReader builds a franz-go consumer for the configured topic and group.
func NewReader(cfg config.Config, sink EventSink, m *metrics.M, detector *anomaly.ZScoreLatencyDetector, anomalyPublisher *AnomalyPublisher) (*Reader, error) {
	brokers := strings.Split(cfg.KafkaBrokers, ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(cfg.KafkaGroupID),
		kgo.ConsumeTopics(cfg.KafkaTopic),
		kgo.DisableAutoCommit(),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka client: %w", err)
	}

	return &Reader{
		client:   cl,
		topic:    cfg.KafkaTopic,
		groupID:  cfg.KafkaGroupID,
		sink:     sink,
		m:        m,
		detector: detector,
		// anomalyPublisher may be nil (e.g. tests / disabled feature)
		anomalyPublisher: anomalyPublisher,
	}, nil
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
					if isDeserializeErr(err) {
						r.m.KafkaDeserializationErrors.Inc()
					} else {
						r.m.KafkaRecordHandoffErrors.Inc()
					}
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

		if err := r.reportConsumerLag(ctx); err != nil && ctx.Err() == nil {
			log.Printf("level=warn msg=lag_update_failed err=%v", err)
		}
	}
}

func (r *Reader) reportConsumerLag(ctx context.Context) error {
	adm := kadm.NewClient(r.client)

	ends, err := adm.ListEndOffsets(ctx, r.topic)
	if err != nil {
		return fmt.Errorf("list end offsets: %w", err)
	}

	committed, err := adm.FetchOffsetsForTopics(ctx, r.groupID, r.topic)
	if err != nil {
		return fmt.Errorf("fetch committed offsets: %w", err)
	}

	parts := ends[r.topic]
	for part, endOff := range parts {
		if endOff.Err != nil {
			continue
		}
		commOff, ok := committed.Lookup(r.topic, part)
		if !ok || commOff.Err != nil {
			continue
		}
		lag := metrics.PartitionLag(endOff.Offset, commOff.At)
		r.m.KafkaConsumerLagEvents.WithLabelValues(
			r.topic,
			strconv.Itoa(int(part)),
			r.groupID,
		).Set(lag)
	}
	return nil
}

func isDeserializeErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unmarshal ingest batch")
}

func (r *Reader) handleRecord(ctx context.Context, rec *kgo.Record) error {
	batch, err := DeserializeBatch(rec.Value)
	if err != nil {
		return err
	}

	// Detect and publish inference latency anomalies before handing off the batch.
	if r.detector != nil && r.anomalyPublisher != nil {
		var detected []*anomaly.DetectedAnomaly
		for i := range batch.Events {
			if a := r.detector.ObserveEvent(batch.Events[i]); a != nil {
				detected = append(detected, a)
			}
		}
		if err := r.anomalyPublisher.Publish(ctx, detected); err != nil {
			return err
		}
	}

	if err := r.sink.Accept(ctx, batch.Events); err != nil {
		return err
	}
	r.m.KafkaRecordsProcessed.Add(float64(len(batch.Events)))
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

// Close releases the underlying Kafka client.
func (r *Reader) Close() {
	r.client.Close()
}
