package clickhouse

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/config"
	"github.com/akshantvats/infra-ai-streaming/consumer/internal/metrics"
	"github.com/akshantvats/infra-ai-streaming/consumer/internal/model"
	redisoverflow "github.com/akshantvats/infra-ai-streaming/consumer/internal/redis"
)

const insertSQL = `INSERT INTO infra_ai.inference_events (
	event_id, tenant_id, model_id, timestamp, latency_ms,
	prefill_latency_ms, decode_latency_ms,
	prompt_tokens, completion_tokens, cost_usd, status, error_code, request_id
) VALUES`

// DLQHandoff sends events to Kafka DLQ after retries are exhausted.
type DLQHandoff interface {
	Publish(ctx context.Context, event model.InferenceEvent, errMsg string, retries int) error
}

type queuedEvent struct {
	event  model.InferenceEvent
	ticket uint64
}

type handoffTicket struct {
	remaining int
	done      chan struct{}
}

// BatchWriter buffers events and flushes to ClickHouse, overflow, or DLQ.
type BatchWriter struct {
	conn          ch.Conn
	cfg           config.Config
	overflow      redisoverflow.OverflowBuffer
	dlq           DLQHandoff
	m             *metrics.M
	cb            *CircuitBreaker
	mu            sync.Mutex
	buf           []queuedEvent
	tickets       map[uint64]*handoffTicket
	nextID        uint64
	flushMu       sync.Mutex
	flushInFlight bool
}

// NewBatchWriter opens ClickHouse and wires dependencies.
func NewBatchWriter(ctx context.Context, cfg config.Config, overflow redisoverflow.OverflowBuffer, dlq DLQHandoff, m *metrics.M) (*BatchWriter, error) {
	opts, err := ch.ParseDSN(cfg.ClickHouseDSN)
	if err != nil {
		return nil, fmt.Errorf("clickhouse parse dsn: %w", err)
	}
	conn, err := ch.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("clickhouse open: %w", err)
	}
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}
	w := &BatchWriter{
		conn:     conn,
		cfg:      cfg,
		overflow: overflow,
		dlq:      dlq,
		m:        m,
		cb:       NewCircuitBreaker(cfg.CBFailures, cfg.CBResetTimeout),
		tickets:  make(map[uint64]*handoffTicket),
	}
	w.m.SetBreakerState(w.cb.State().String())
	return w, nil
}

// Close releases the ClickHouse connection.
func (w *BatchWriter) Close() error {
	return w.conn.Close()
}

// Start runs periodic flush and overflow drain loops until ctx is cancelled.
func (w *BatchWriter) Start(ctx context.Context) {
	flushTicker := time.NewTicker(w.cfg.FlushInterval)
	drainTicker := time.NewTicker(w.cfg.DrainInterval)
	defer flushTicker.Stop()
	defer drainTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			w.Flush(context.Background())
			return
		case <-flushTicker.C:
			w.Flush(ctx)
		case <-drainTicker.C:
			w.drainOverflow(ctx)
		}
	}
}

// Accept enqueues events and blocks until each is handed off (CH, overflow, or DLQ).
func (w *BatchWriter) Accept(ctx context.Context, events []model.InferenceEvent) error {
	if len(events) == 0 {
		return nil
	}
	w.mu.Lock()
	id := w.nextID
	w.nextID++
	ticket := &handoffTicket{remaining: len(events), done: make(chan struct{})}
	w.tickets[id] = ticket
	for _, e := range events {
		w.buf = append(w.buf, queuedEvent{event: e, ticket: id})
	}
	shouldFlush := len(w.buf) >= w.cfg.BatchSize
	w.mu.Unlock()

	if shouldFlush {
		go w.Flush(ctx)
	}

	select {
	case <-ticket.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Flush drains the buffer and performs handoff I/O.
func (w *BatchWriter) Flush(ctx context.Context) {
	w.flushMu.Lock()
	if w.flushInFlight {
		w.flushMu.Unlock()
		return
	}
	w.flushInFlight = true
	w.flushMu.Unlock()

	defer func() {
		w.flushMu.Lock()
		w.flushInFlight = false
		w.flushMu.Unlock()
	}()

	w.mu.Lock()
	if len(w.buf) == 0 {
		w.mu.Unlock()
		return
	}
	queued := append([]queuedEvent(nil), w.buf...)
	w.buf = w.buf[:0]
	w.mu.Unlock()

	w.handoffQueued(ctx, queued)
}

func (w *BatchWriter) drainOverflow(ctx context.Context) {
	if w.cb.State() != BreakerClosed {
		return
	}
	for {
		events, err := w.overflow.PopN(ctx, w.cfg.DrainBatchSize)
		if err != nil {
			log.Printf("level=error msg=overflow_pop_failed err=%v", err)
			return
		}
		if len(events) == 0 {
			return
		}
		// Drained overflow events have no Kafka ticket; handoff is internal only.
		w.handoffEvents(ctx, events, nil)
	}
}

func (w *BatchWriter) handoffQueued(ctx context.Context, queued []queuedEvent) {
	events := make([]model.InferenceEvent, len(queued))
	tickets := make([]uint64, len(queued))
	for i, q := range queued {
		events[i] = q.event
		tickets[i] = q.ticket
	}
	w.handoffEvents(ctx, events, tickets)
}

func (w *BatchWriter) handoffEvents(ctx context.Context, events []model.InferenceEvent, tickets []uint64) {
	if len(events) == 0 {
		return
	}

	if !w.cb.AllowInsert() {
		w.m.SetBreakerState(w.cb.State().String())
		if err := w.overflow.Push(ctx, events); err != nil {
			log.Printf("level=error msg=overflow_push_failed count=%d err=%v", len(events), err)
			return
		}
		w.signalTickets(tickets, len(events))
		return
	}

	start := time.Now()
	handled, err := w.insertWithRetries(ctx, events)
	w.m.ClickHouseFlushDur.Observe(time.Since(start).Seconds())

	if handled {
		w.cb.RecordSuccess()
		w.m.SetBreakerState(w.cb.State().String())
		w.m.ClickHouseBatchSize.Observe(float64(len(events)))
		w.signalTickets(tickets, len(events))
		return
	}

	w.m.ClickHouseWriteErrors.Inc()
	w.cb.RecordFailure()
	w.m.SetBreakerState(w.cb.State().String())
	log.Printf("level=warn msg=clickhouse_batch_failed count=%d err=%v", len(events), err)

	if err := w.overflow.Push(ctx, events); err != nil {
		log.Printf("level=error msg=overflow_push_after_ch_fail count=%d err=%v", len(events), err)
		return
	}
	w.signalTickets(tickets, len(events))
}

// insertWithRetries returns true when events were inserted or sent to DLQ.
func (w *BatchWriter) insertWithRetries(ctx context.Context, events []model.InferenceEvent) (bool, error) {
	var lastErr error
	for attempt := 1; attempt <= w.cfg.InsertRetries; attempt++ {
		lastErr = w.batchInsert(ctx, events)
		if lastErr == nil {
			return true, nil
		}
	}
	for _, e := range events {
		if err := w.dlq.Publish(ctx, e, lastErr.Error(), w.cfg.InsertRetries); err != nil {
			log.Printf("level=error msg=dlq_publish_failed tenant_id=%s err=%v", e.TenantID, err)
			return false, err
		}
	}
	return true, lastErr
}

func (w *BatchWriter) batchInsert(ctx context.Context, events []model.InferenceEvent) error {
	rows, err := RowsFromEvents(events)
	if err != nil {
		return err
	}
	batch, err := w.conn.PrepareBatch(ctx, insertSQL)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if err := batch.Append(
			r.EventID,
			r.TenantID,
			r.ModelID,
			r.Timestamp,
			r.LatencyMs,
			r.PrefillLatencyMs,
			r.DecodeLatencyMs,
			r.PromptTokens,
			r.CompletionTokens,
			r.CostUSD,
			r.Status,
			r.ErrorCode,
			r.RequestID,
		); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (w *BatchWriter) signalTickets(tickets []uint64, count int) {
	if len(tickets) == 0 {
		return
	}
	if len(tickets) != count {
		// Should not happen; fall back to completing all tickets touched.
		seen := make(map[uint64]int)
		for _, id := range tickets {
			seen[id]++
		}
		for id, n := range seen {
			w.finishTicket(id, n)
		}
		return
	}
	for i := 0; i < count; i++ {
		w.finishTicket(tickets[i], 1)
	}
}

func (w *BatchWriter) finishTicket(id uint64, n int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	t, ok := w.tickets[id]
	if !ok {
		return
	}
	t.remaining -= n
	if t.remaining <= 0 {
		close(t.done)
		delete(w.tickets, id)
	}
}
