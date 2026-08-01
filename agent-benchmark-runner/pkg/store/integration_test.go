// SPDX-License-Identifier: MIT

//go:build integration

package store

import (
	"context"
	"os"
	"testing"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"

	"github.com/akshantvats/agent-benchmark-runner/pkg/orchestrator"
)

func clickhouseDSN(t *testing.T) string {
	t.Helper()
	v := os.Getenv("CLICKHOUSE_DSN")
	if v == "" {
		t.Skip("set CLICKHOUSE_DSN to run ClickHouse integration tests")
	}
	return v
}

// TestNewClickHouseWriterConnects verifies that NewClickHouseWriter dials
// ClickHouse and pings it.
func TestNewClickHouseWriterConnects(t *testing.T) {
	dsn := clickhouseDSN(t)
	ctx := context.Background()

	w, err := NewClickHouseWriter(ctx, dsn)
	if err != nil {
		t.Fatalf("NewClickHouseWriter: %v", err)
	}
	defer w.Close()
}

// TestWriteRunsRowsAppearInClickHouse writes a small batch of RunRecords
// and queries them back by task_id to confirm they landed.
func TestWriteRunsRowsAppearInClickHouse(t *testing.T) {
	dsn := clickhouseDSN(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	w, err := NewClickHouseWriter(ctx, dsn)
	if err != nil {
		t.Fatalf("NewClickHouseWriter: %v", err)
	}
	defer w.Close()

	taskID := "integration-test-task"
	results := []orchestrator.RunResult{
		{RepetitionIndex: 0, Seed: 1, Passed: true, StepCount: 2},
		{RepetitionIndex: 1, Seed: 2, Passed: false, StepCount: 4},
	}
	records := NewRunRecords(taskID, "integration-test-agent", results, time.Now().UTC())

	if err := w.WriteRuns(ctx, records); err != nil {
		t.Fatalf("WriteRuns: %v", err)
	}

	opts, err := ch.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("ParseDSN: %v", err)
	}
	conn, err := ch.Open(opts)
	if err != nil {
		t.Fatalf("ch.Open: %v", err)
	}
	defer conn.Close()

	row := conn.QueryRow(ctx,
		`SELECT count() FROM benchmark_runs WHERE task_id = $1 AND agent_name = $2`,
		taskID, "integration-test-agent")
	var cnt uint64
	if err := row.Scan(&cnt); err != nil {
		t.Fatalf("scan row count: %v", err)
	}
	if cnt != 2 {
		t.Fatalf("expected 2 rows in benchmark_runs for task_id=%s, got %d", taskID, cnt)
	}
}

// TestWriteRunsEmptyIsNoop verifies WriteRuns tolerates an empty batch
// without erroring.
func TestWriteRunsEmptyIsNoop(t *testing.T) {
	dsn := clickhouseDSN(t)
	ctx := context.Background()

	w, err := NewClickHouseWriter(ctx, dsn)
	if err != nil {
		t.Fatalf("NewClickHouseWriter: %v", err)
	}
	defer w.Close()

	if err := w.WriteRuns(ctx, nil); err != nil {
		t.Fatalf("WriteRuns(nil): %v", err)
	}
}
