package clickhouse

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/model"
)

func ptr[T any](v T) *T { return &v }

func TestRowFromEventDefaults(t *testing.T) {
	status := "success"
	row, err := RowFromEvent(model.InferenceEvent{
		TenantID:         "demo",
		ModelID:          "gpt-4o",
		TimestampUnixMs:  1715000000000,
		LatencyMs:        100,
		PromptTokens:     10,
		CompletionTokens: 5,
		CostUSD:          0.01,
		Status:           &status,
	})
	if err != nil {
		t.Fatal(err)
	}
	if row.TenantID != "demo" || row.CostUSD != 0.01 {
		t.Fatalf("row = %+v", row)
	}
	if row.Status != "success" {
		t.Fatalf("status = %q", row.Status)
	}
}

// TestRowFromEventNilEventIDGeneratesUUID verifies that a missing EventID
// causes a random UUID to be assigned rather than returning an error.
func TestRowFromEventNilEventIDGeneratesUUID(t *testing.T) {
	row, err := RowFromEvent(model.InferenceEvent{
		TenantID:        "t1",
		ModelID:         "m1",
		TimestampUnixMs: 1715000000000,
		LatencyMs:       50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row.EventID == (uuid.UUID{}) {
		t.Fatal("expected a non-zero UUID to be generated")
	}
}

// TestRowFromEventEmptyEventIDGeneratesUUID verifies the empty-string branch
// (EventID pointer present but value is "").
func TestRowFromEventEmptyEventIDGeneratesUUID(t *testing.T) {
	row, err := RowFromEvent(model.InferenceEvent{
		EventID:         ptr(""),
		TenantID:        "t1",
		ModelID:         "m1",
		TimestampUnixMs: 1715000000000,
		LatencyMs:       50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row.EventID == (uuid.UUID{}) {
		t.Fatal("expected a non-zero UUID to be generated for empty event_id")
	}
}

// TestRowFromEventExplicitUUID verifies that a valid UUID string is preserved.
func TestRowFromEventExplicitUUID(t *testing.T) {
	id := "550e8400-e29b-41d4-a716-446655440000"
	row, err := RowFromEvent(model.InferenceEvent{
		EventID:         ptr(id),
		TenantID:        "t1",
		ModelID:         "m1",
		TimestampUnixMs: 1715000000000,
		LatencyMs:       50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row.EventID.String() != id {
		t.Fatalf("event_id = %q, want %q", row.EventID, id)
	}
}

// TestRowFromEventInvalidUUIDReturnsError verifies that a malformed UUID
// causes RowFromEvent to return an error.
func TestRowFromEventInvalidUUIDReturnsError(t *testing.T) {
	_, err := RowFromEvent(model.InferenceEvent{
		EventID:         ptr("not-a-uuid"),
		TenantID:        "t1",
		ModelID:         "m1",
		TimestampUnixMs: 1715000000000,
		LatencyMs:       50,
	})
	if err == nil {
		t.Fatal("expected error for invalid UUID, got nil")
	}
}

// TestRowFromEventNilStatusDefaultsToSuccess verifies that a nil Status
// pointer causes the row to carry "success".
func TestRowFromEventNilStatusDefaultsToSuccess(t *testing.T) {
	row, err := RowFromEvent(model.InferenceEvent{
		TenantID:        "t1",
		ModelID:         "m1",
		TimestampUnixMs: 1715000000000,
		LatencyMs:       50,
		// Status intentionally nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "success" {
		t.Fatalf("status = %q, want success", row.Status)
	}
}

// TestRowFromEventTimestampMapping verifies millisecond → time.Time conversion.
func TestRowFromEventTimestampMapping(t *testing.T) {
	tsMs := uint64(1715000000000)
	row, err := RowFromEvent(model.InferenceEvent{
		TenantID:        "t1",
		ModelID:         "m1",
		TimestampUnixMs: tsMs,
		LatencyMs:       50,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := time.UnixMilli(int64(tsMs)).UTC()
	if !row.Timestamp.Equal(want) {
		t.Fatalf("timestamp = %v, want %v", row.Timestamp, want)
	}
}

// TestRowFromEventOptionalFields verifies that optional pointer fields are
// forwarded unchanged from the model.
func TestRowFromEventOptionalFields(t *testing.T) {
	prefill := uint32(10)
	decode := uint32(20)
	errCode := "timeout"
	reqID := "req-123"
	row, err := RowFromEvent(model.InferenceEvent{
		TenantID:         "t1",
		ModelID:          "m1",
		TimestampUnixMs:  1715000000000,
		LatencyMs:        50,
		PrefillLatencyMs: &prefill,
		DecodeLatencyMs:  &decode,
		ErrorCode:        &errCode,
		RequestID:        &reqID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if row.PrefillLatencyMs == nil || *row.PrefillLatencyMs != prefill {
		t.Errorf("PrefillLatencyMs = %v, want %d", row.PrefillLatencyMs, prefill)
	}
	if row.DecodeLatencyMs == nil || *row.DecodeLatencyMs != decode {
		t.Errorf("DecodeLatencyMs = %v, want %d", row.DecodeLatencyMs, decode)
	}
	if row.ErrorCode == nil || *row.ErrorCode != errCode {
		t.Errorf("ErrorCode = %v, want %q", row.ErrorCode, errCode)
	}
	if row.RequestID == nil || *row.RequestID != reqID {
		t.Errorf("RequestID = %v, want %q", row.RequestID, reqID)
	}
}

// TestRowsFromEventsConvertsAll verifies that RowsFromEvents maps a slice of
// events without error and preserves order.
func TestRowsFromEventsConvertsAll(t *testing.T) {
	events := []model.InferenceEvent{
		{TenantID: "t1", ModelID: "m1", TimestampUnixMs: 1, LatencyMs: 10},
		{TenantID: "t2", ModelID: "m2", TimestampUnixMs: 2, LatencyMs: 20},
	}
	rows, err := RowsFromEvents(events)
	if err != nil {
		t.Fatalf("RowsFromEvents: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len = %d, want 2", len(rows))
	}
	if rows[0].TenantID != "t1" || rows[1].TenantID != "t2" {
		t.Fatalf("tenant ordering wrong: %v", rows)
	}
}

// TestRowsFromEventsShortCircuitsOnError verifies that the first invalid event
// causes an early-return error and no partial slice.
func TestRowsFromEventsShortCircuitsOnError(t *testing.T) {
	events := []model.InferenceEvent{
		{EventID: ptr("bad-uuid"), TenantID: "t1", ModelID: "m1", TimestampUnixMs: 1, LatencyMs: 1},
		{TenantID: "t2", ModelID: "m2", TimestampUnixMs: 2, LatencyMs: 2},
	}
	_, err := RowsFromEvents(events)
	if err == nil {
		t.Fatal("expected error for invalid UUID in batch")
	}
}
