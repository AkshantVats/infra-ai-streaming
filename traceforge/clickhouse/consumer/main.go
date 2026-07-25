// SPDX-License-Identifier: MIT
// TraceForge: Kafka → ClickHouse batch consumer
// Consumes agent-spans topic and batch-inserts spans to ClickHouse.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/IBM/sarama"
)

// Span mirrors the TraceForge span schema for JSON decode.
type Span struct {
	TraceID      string            `json:"trace_id"`
	SpanID       string            `json:"span_id"`
	ParentSpanID string            `json:"parent_span_id"`
	ToolName     string            `json:"tool_name"`
	ToolKind     string            `json:"tool_kind"`
	Model        string            `json:"model"`
	Status       string            `json:"status"`
	StartTime    string            `json:"start_time"`
	LatencyMs    int64             `json:"latency_ms"`
	InputTokens  int               `json:"input_tokens"`
	OutputTokens int               `json:"output_tokens"`
	TotalTokens  int               `json:"total_tokens"`
	CostUSD      float64           `json:"cost_usd"`
	ErrorMessage string            `json:"error_message"`
	Attributes   map[string]string `json:"attributes"`
}

const (
	batchSize     = 500
	flushInterval = 2 * time.Second
)

func main() {
	brokers := envOrDefault("TRACEFORGE_KAFKA_BROKERS", "localhost:9092")
	topic := envOrDefault("TRACEFORGE_KAFKA_TOPIC", "agent-spans")
	chDSN := envOrDefault("TRACEFORGE_CLICKHOUSE_DSN", "clickhouse://localhost:9000/default")

	db, err := sql.Open("clickhouse", chDSN)
	if err != nil {
		log.Fatalf("clickhouse open: %v", err)
	}
	defer db.Close()

	cfg := sarama.NewConfig()
	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	cfg.Consumer.Offsets.Initial = sarama.OffsetNewest

	client, err := sarama.NewConsumerGroup([]string{brokers}, "traceforge-go-consumer", cfg)
	if err != nil {
		log.Fatalf("consumer group: %v", err)
	}
	defer client.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	h := &handler{db: db, buf: make([]Span, 0, batchSize)}
	for {
		if err := client.Consume(ctx, []string{topic}, h); err != nil {
			log.Printf("consume error: %v", err)
		}
		if ctx.Err() != nil {
			return
		}
	}
}

type handler struct {
	db  *sql.DB
	buf []Span
}

func (h *handler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *handler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *handler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				h.flush(sess)
				return nil
			}
			var s Span
			if err := json.Unmarshal(msg.Value, &s); err != nil {
				log.Printf("decode span: %v", err)
				sess.MarkMessage(msg, "")
				continue
			}
			h.buf = append(h.buf, s)
			sess.MarkMessage(msg, "")
			if len(h.buf) >= batchSize {
				h.flush(sess)
			}
		case <-ticker.C:
			h.flush(sess)
		}
	}
}

func (h *handler) flush(sess sarama.ConsumerGroupSession) {
	if len(h.buf) == 0 {
		return
	}
	if err := insertBatch(h.db, h.buf); err != nil {
		log.Printf("insert batch error: %v", err)
	} else {
		log.Printf("flushed %d spans to ClickHouse", len(h.buf))
	}
	h.buf = h.buf[:0]
	sess.Commit()
}

const insertSQL = `INSERT INTO agent_spans
    (trace_id, span_id, parent_span_id, tool_name, tool_kind, model,
     status, error_message, start_time, latency_ms,
     input_tokens, output_tokens, total_tokens, cost_usd, attributes)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

func insertBatch(db *sql.DB, spans []Span) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, s := range spans {
		attrs := "{}"
		if len(s.Attributes) > 0 {
			if b, e := json.Marshal(s.Attributes); e == nil {
				attrs = string(b)
			}
		}
		if _, err := stmt.Exec(
			s.TraceID, s.SpanID, s.ParentSpanID, s.ToolName, s.ToolKind, s.Model,
			s.Status, s.ErrorMessage, s.StartTime, s.LatencyMs,
			s.InputTokens, s.OutputTokens, s.TotalTokens, s.CostUSD, attrs,
		); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
