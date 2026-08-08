// SPDX-License-Identifier: MIT

// Package store persists judged samples to ClickHouse's quality_scores
// table, one row per judged sample (DESIGN.md §6): aggregates are
// computed at query time by slicing tenant_id/task_type/model_id, never
// pre-collapsed into a single running average at write time.
package store

import (
	"context"
	"fmt"
	"time"
)

// ScoredSample is one row of quality_scores.
type ScoredSample struct {
	TenantID      string
	TaskType      string
	ModelID       string
	RubricVersion int
	Score         float64 // 0-100, normalized (rubric.WeightedScore's output)
	Rationale     string
	ScoredAt      time.Time
}

// Writer persists a batch of scored samples. Implementations must
// either write every row in the batch or return an error — a partial
// write silently under-reports coverage for that hour against the
// 200/hr target (DESIGN.md §4) without anyone finding out.
type Writer interface {
	WriteBatch(ctx context.Context, rows []ScoredSample) error
}

// Validate reports whether s is a well-formed row worth inserting.
func (s ScoredSample) Validate() error {
	if s.TenantID == "" {
		return fmt.Errorf("store: tenant_id is empty")
	}
	if s.TaskType == "" {
		return fmt.Errorf("store: task_type is empty")
	}
	if s.Score < 0 || s.Score > 100 {
		return fmt.Errorf("store: score %v out of range [0,100]", s.Score)
	}
	if s.ScoredAt.IsZero() {
		return fmt.Errorf("store: scored_at is zero")
	}
	return nil
}
