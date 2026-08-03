// SPDX-License-Identifier: MIT
package clickhouse

import (
	"time"

	"github.com/google/uuid"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/model"
)

// Row is a ClickHouse-ready inference event row (infra_ai.inference_events).
type Row struct {
	EventID          uuid.UUID
	TenantID         string
	ModelID          string
	Timestamp        time.Time
	LatencyMs        uint32
	PrefillLatencyMs *uint32
	DecodeLatencyMs  *uint32
	PromptTokens     uint32
	CompletionTokens uint32
	CostUSD          float64
	Status           string
	ErrorCode        *string
	RequestID        *string
	TraceID          *string
	Source           string
}

// RowFromEvent maps the Kafka JSON model to init.sql columns.
func RowFromEvent(e model.InferenceEvent) (Row, error) {
	var eventID uuid.UUID
	if e.EventID != nil && *e.EventID != "" {
		parsed, err := uuid.Parse(*e.EventID)
		if err != nil {
			return Row{}, err
		}
		eventID = parsed
	} else {
		eventID = uuid.New()
	}

	status := "success"
	if e.Status != nil && *e.Status != "" {
		status = *e.Status
	}

	source := "inference"
	if e.Source != nil && *e.Source != "" {
		source = *e.Source
	}

	return Row{
		EventID:          eventID,
		TenantID:         e.TenantID,
		ModelID:          e.ModelID,
		Timestamp:        time.UnixMilli(int64(e.TimestampUnixMs)).UTC(),
		LatencyMs:        e.LatencyMs,
		PrefillLatencyMs: e.PrefillLatencyMs,
		DecodeLatencyMs:  e.DecodeLatencyMs,
		PromptTokens:     e.PromptTokens,
		CompletionTokens: e.CompletionTokens,
		CostUSD:          e.CostUSD,
		Status:           status,
		ErrorCode:        e.ErrorCode,
		RequestID:        e.RequestID,
		TraceID:          e.TraceID,
		Source:           source,
	}, nil
}

// RowsFromEvents converts a slice of inference events.
func RowsFromEvents(events []model.InferenceEvent) ([]Row, error) {
	rows := make([]Row, 0, len(events))
	for _, e := range events {
		r, err := RowFromEvent(e)
		if err != nil {
			return nil, err
		}
		rows = append(rows, r)
	}
	return rows, nil
}
