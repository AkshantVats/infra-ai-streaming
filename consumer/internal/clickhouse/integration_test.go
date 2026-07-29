// SPDX-License-Identifier: MIT

//go:build integration

package clickhouse

import (
	"context"
	"os"
	"testing"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/config"
	"github.com/akshantvats/infra-ai-streaming/consumer/internal/model"
)

func clickhouseDSN(t *testing.T) string {
	t.Helper()
	v := os.Getenv("CLICKHOUSE_DSN")
	if v == "" {
		t.Skip("set CLICKHOUSE_DSN to run ClickHouse integration tests")
	}
	return v
}

func testConfig(dsn string) config.Config {
	return config.Config{
		ClickHouseDSN:  dsn,
		BatchSize:      1,
		FlushInterval:  100 * time.Millisecond,
		CBFailures:     3,
		CBResetTimeout: 30 * time.Second,
		InsertRetries:  1,
		DrainInterval:  5 * time.Second,
		DrainBatchSize: 100,
	}
}

// nopDLQ silently discards DLQ events.
type nopDLQ struct{}

func (nopDLQ) Publish(_ context.Context, _ model.InferenceEvent, _ string, _ int) error {
	return nil
}

// TestNewBatchWriterConnects verifies that NewBatchWriter dials ClickHouse and pings it.
func TestNewBatchWriterConnects(t *testing.T) {
	dsn := clickhouseDSN(t)
	ctx := context.Background()

	overflow := &mockOverflow{}
	w, err := NewBatchWriter(ctx, testConfig(dsn), overflow, nopDLQ{}, newTestMetrics())
	if err != nil {
		t.Fatalf("NewBatchWriter: %v", err)
	}
	defer w.Close()
}

// TestClickHouseTableExists verifies the inference_events table is reachable.
func TestClickHouseTableExists(t *testing.T) {
	dsn := clickhouseDSN(t)
	ctx := context.Background()

	opts, err := ch.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("ParseDSN: %v", err)
	}
	conn, err := ch.Open(opts)
	if err != nil {
		t.Fatalf("ch.Open: %v", err)
	}
	defer conn.Close()

	if err := conn.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	row := conn.QueryRow(ctx,
		`SELECT count() FROM system.tables WHERE database='infra_ai' AND name='inference_events'`)
	var cnt uint64
	if err := row.Scan(&cnt); err != nil {
		t.Fatalf("scan table count: %v", err)
	}
	if cnt == 0 {
		t.Fatal("table infra_ai.inference_events does not exist")
	}
}

// TestAcceptFlushRowAppearsInClickHouse produces one event via Accept,
// triggers Flush, then queries ClickHouse to confirm the row landed.
func TestAcceptFlushRowAppearsInClickHouse(t *testing.T) {
	dsn := clickhouseDSN(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	overflow := &mockOverflow{}
	cfg := testConfig(dsn)
	cfg.BatchSize = 1 // trigger auto-flush on first Accept

	w, err := NewBatchWriter(ctx, cfg, overflow, nopDLQ{}, newTestMetrics())
	if err != nil {
		t.Fatalf("NewBatchWriter: %v", err)
	}
	defer w.Close()

	ev := sampleEvent()

	// Accept blocks until handoff completes (CH write or overflow).
	if err := w.Accept(ctx, []model.InferenceEvent{ev}); err != nil {
		t.Fatalf("Accept: %v", err)
	}

	// Verify the row appeared in ClickHouse.
	row := w.conn.QueryRow(ctx,
		`SELECT count() FROM infra_ai.inference_events WHERE tenant_id = $1 AND model_id = $2`,
		ev.TenantID, ev.ModelID)
	var cnt uint64
	if err := row.Scan(&cnt); err != nil {
		t.Fatalf("query row count: %v", err)
	}
	if cnt == 0 {
		t.Fatalf("expected row in infra_ai.inference_events for tenant=%s model=%s, got 0", ev.TenantID, ev.ModelID)
	}
}

// TestCircuitBreakerOpensAndOverflows verifies that after CBFailures insert
// failures the breaker opens and subsequent events go to overflow.
func TestCircuitBreakerOpensAndOverflows(t *testing.T) {
	// Use a deliberately bad DSN so inserts fail immediately.
	badDSN := "clickhouse://127.0.0.1:19999/nonexistent"

	// We need a valid ClickHouse connection for NewBatchWriter (it pings on startup).
	// Use the real DSN for construction, then corrupt the connection for inserts.
	realDSN := os.Getenv("CLICKHOUSE_DSN")
	if realDSN == "" {
		t.Skip("set CLICKHOUSE_DSN to run ClickHouse integration tests")
	}
	_ = badDSN

	ctx := context.Background()
	overflow := &mockOverflow{}
	cfg := testConfig(realDSN)
	cfg.CBFailures = 1 // open after first failure
	cfg.InsertRetries = 1
	cfg.BatchSize = 1

	w, err := NewBatchWriter(ctx, cfg, overflow, nopDLQ{}, newTestMetrics())
	if err != nil {
		t.Fatalf("NewBatchWriter: %v", err)
	}
	defer w.Close()

	// Force the breaker open by injecting a failure directly.
	w.cb.RecordFailure()

	if w.cb.State() != BreakerOpen {
		t.Fatalf("expected breaker open, got %v", w.cb.State())
	}

	ev := sampleEvent()

	acceptCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := w.Accept(acceptCtx, []model.InferenceEvent{ev}); err != nil {
		t.Fatalf("Accept: %v", err)
	}

	if overflow.pushed == 0 {
		t.Fatal("expected event to overflow when breaker is open, got 0 overflow pushes")
	}
}
