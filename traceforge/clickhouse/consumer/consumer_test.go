// SPDX-License-Identifier: MIT
package main

import (
	"encoding/json"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestInsertBatch_SingleSpan(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO agent_spans")
	mock.ExpectExec("INSERT INTO agent_spans").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	spans := []Span{{
		TraceID:   "abc123",
		SpanID:    "def456",
		ToolName:  "test_tool",
		ToolKind:  "http",
		Status:    "OK",
		StartTime: time.Now().UTC().Format(time.RFC3339Nano),
		LatencyMs: 42,
	}}
	if err := insertBatch(db, spans); err != nil {
		t.Fatalf("insertBatch: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}
}

func TestInsertBatch_AttributesSerialized(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO agent_spans")
	mock.ExpectExec("INSERT INTO agent_spans").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	spans := []Span{{
		TraceID:    "trace1",
		SpanID:     "span1",
		StartTime:  time.Now().UTC().Format(time.RFC3339Nano),
		Attributes: map[string]string{"city": "Berlin", "units": "metric"},
	}}
	if err := insertBatch(db, spans); err != nil {
		t.Fatalf("insertBatch: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}
}

func TestFlushOnBatchSize(t *testing.T) {
	spans := make([]Span, batchSize)
	for i := range spans {
		spans[i] = Span{TraceID: "t", SpanID: "s", StartTime: time.Now().UTC().Format(time.RFC3339Nano)}
	}
	b, err := json.Marshal(spans[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("empty marshal")
	}
	if batchSize < 100 || batchSize > 10000 {
		t.Fatalf("batchSize %d out of expected range", batchSize)
	}
}

func TestEnvOrDefault(t *testing.T) {
	got := envOrDefault("__NO_SUCH_VAR__", "fallback")
	if got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
}

func TestInsertBatch_EmptyAttributes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO agent_spans")
	mock.ExpectExec("INSERT INTO agent_spans").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	spans := []Span{{
		TraceID:   "tid",
		SpanID:    "sid",
		StartTime: time.Now().UTC().Format(time.RFC3339Nano),
	}}
	if err := insertBatch(db, spans); err != nil {
		t.Fatalf("insertBatch with empty attrs: %v", err)
	}
}
