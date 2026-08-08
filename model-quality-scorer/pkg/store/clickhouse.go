// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"fmt"

	ch "github.com/ClickHouse/clickhouse-go/v2"
)

const insertSQL = `INSERT INTO infra_ai.quality_scores (
	tenant_id, task_type, model_id, rubric_version, score, normalized_score, rationale, scored_at
) VALUES`

// ClickHouseWriter is the production Writer, backed by a live ClickHouse
// connection.
type ClickHouseWriter struct {
	conn ch.Conn
}

// NewClickHouseWriter opens dsn and verifies connectivity before
// returning, so a bad DSN fails at startup rather than on the first
// batch flush.
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

// WriteBatch implements Writer.
func (w *ClickHouseWriter) WriteBatch(ctx context.Context, rows []ScoredSample) error {
	if len(rows) == 0 {
		return nil
	}
	batch, err := w.conn.PrepareBatch(ctx, insertSQL)
	if err != nil {
		return fmt.Errorf("store: prepare batch: %w", err)
	}
	for _, r := range rows {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("store: invalid row for tenant %s: %w", r.TenantID, err)
		}
		if err := batch.Append(r.TenantID, r.TaskType, r.ModelID, r.RubricVersion, r.Score, r.NormalizedScore, r.Rationale, r.ScoredAt); err != nil {
			return fmt.Errorf("store: append row for tenant %s: %w", r.TenantID, err)
		}
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("store: send batch of %d rows: %w", len(rows), err)
	}
	return nil
}

// Close releases the underlying ClickHouse connection.
func (w *ClickHouseWriter) Close() error {
	return w.conn.Close()
}
