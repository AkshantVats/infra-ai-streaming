// SPDX-License-Identifier: MIT

// Package store persists orchestrator.RunResult batches to ClickHouse's
// benchmark_runs table (pkg/store/schema/001_benchmark_runs.sql), one row
// per repetition. It depends on pkg/orchestrator; pkg/orchestrator has no
// dependency back on pkg/store, so the core run/grade/summarize path stays
// free of any database — see DESIGN.md's "Persistence Is an Injected
// Writer" section.
package store

import (
	"context"
	"fmt"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"

	"github.com/akshantvats/agent-benchmark-runner/pkg/orchestrator"
)

// RunRecord is the flat row shape written to benchmark_runs.
type RunRecord struct {
	TaskID          string
	AgentName       string
	RepetitionIndex uint32
	Seed            int64
	Passed          bool
	StepCount       uint32
	ErrorMessage    string
	Timestamp       time.Time
}

// NewRunRecords maps a batch of orchestrator.RunResult into the flat rows
// benchmark_runs expects, stamping every row with the same ts (the batch's
// completion time) and the taskID/agentName that produced them — neither
// of which orchestrator.RunResult carries itself, since a single Config
// already fixes them for the whole batch.
func NewRunRecords(taskID, agentName string, results []orchestrator.RunResult, ts time.Time) []RunRecord {
	records := make([]RunRecord, len(results))
	for i, r := range results {
		rec := RunRecord{
			TaskID:          taskID,
			AgentName:       agentName,
			RepetitionIndex: uint32(r.RepetitionIndex),
			Seed:            r.Seed,
			Passed:          r.Passed,
			StepCount:       uint32(r.StepCount),
			Timestamp:       ts,
		}
		if r.Err != nil {
			rec.ErrorMessage = r.Err.Error()
		}
		records[i] = rec
	}
	return records
}

// Writer persists benchmark run records. It is an interface, not a
// concrete ClickHouse type, so callers (and tests) that only need to
// exercise orchestration and grading never have to stand up a database.
type Writer interface {
	WriteRuns(ctx context.Context, records []RunRecord) error
}

const insertRunsSQL = `INSERT INTO benchmark_runs (
	task_id, agent_name, repetition_index, seed, passed, step_count, error_message, timestamp
)`

// ClickHouseWriter writes RunRecords to a ClickHouse benchmark_runs table
// over github.com/ClickHouse/clickhouse-go/v2.
type ClickHouseWriter struct {
	conn ch.Conn
}

// NewClickHouseWriter dials dsn and pings it before returning, so
// connection failures surface at construction time rather than on the
// first WriteRuns call.
func NewClickHouseWriter(ctx context.Context, dsn string) (*ClickHouseWriter, error) {
	opts, err := ch.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("store: parse dsn: %w", err)
	}
	conn, err := ch.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &ClickHouseWriter{conn: conn}, nil
}

// Close releases the underlying ClickHouse connection.
func (w *ClickHouseWriter) Close() error {
	return w.conn.Close()
}

// WriteRuns batches and inserts records in a single ClickHouse batch
// insert. An empty records slice is a no-op.
func (w *ClickHouseWriter) WriteRuns(ctx context.Context, records []RunRecord) error {
	if len(records) == 0 {
		return nil
	}

	batch, err := w.conn.PrepareBatch(ctx, insertRunsSQL)
	if err != nil {
		return fmt.Errorf("store: prepare batch: %w", err)
	}
	for _, r := range records {
		if err := batch.Append(
			r.TaskID,
			r.AgentName,
			r.RepetitionIndex,
			r.Seed,
			boolToUInt8(r.Passed),
			r.StepCount,
			r.ErrorMessage,
			r.Timestamp,
		); err != nil {
			return fmt.Errorf("store: append row: %w", err)
		}
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("store: send batch: %w", err)
	}
	return nil
}

func boolToUInt8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
