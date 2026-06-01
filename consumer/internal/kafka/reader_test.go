package kafka

import (
	"context"
	"strings"
	"testing"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/model"
)

// ── DeserializeBatch original tests ──────────────────────────────────────────

func TestDeserializeBatch(t *testing.T) {
	payload := []byte(`{
		"events": [{
			"tenant_id": "demo",
			"model_id": "gpt-4o",
			"timestamp_unix_ms": 1715000000000,
			"latency_ms": 342,
			"prompt_tokens": 512,
			"completion_tokens": 128,
			"cost_usd": 0.00423,
			"status": "success"
		}]
	}`)

	batch, err := DeserializeBatch(payload)
	if err != nil {
		t.Fatalf("DeserializeBatch: %v", err)
	}
	if len(batch.Events) != 1 {
		t.Fatalf("events len = %d, want 1", len(batch.Events))
	}

	ev := batch.Events[0]
	if ev.TenantID != "demo" || ev.ModelID != "gpt-4o" {
		t.Errorf("tenant/model = %q / %q", ev.TenantID, ev.ModelID)
	}
	if ev.PromptTokens != 512 || ev.CompletionTokens != 128 {
		t.Errorf("tokens = %d / %d", ev.PromptTokens, ev.CompletionTokens)
	}
	if ev.CostUSD != 0.00423 {
		t.Errorf("cost_usd = %g", ev.CostUSD)
	}
	if ev.LatencyMs != 342 {
		t.Errorf("latency_ms = %d", ev.LatencyMs)
	}
}

func TestDeserializeBatchInvalidJSON(t *testing.T) {
	_, err := DeserializeBatch([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ── DeserializeBatch table-driven tests ──────────────────────────────────────

// TestDeserializeBatchTable covers valid, empty-array, partial-fields, and
// malformed cases in a single table-driven test.
func TestDeserializeBatchTable(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		wantErr    bool
		wantCount  int
		checkFirst func(t *testing.T, ev model.InferenceEvent)
	}{
		{
			name: "single event all fields",
			payload: `{"events":[{
				"tenant_id":"acme","model_id":"claude-3","timestamp_unix_ms":1000,
				"latency_ms":50,"prompt_tokens":10,"completion_tokens":5,"cost_usd":0.001
			}]}`,
			wantCount: 1,
			checkFirst: func(t *testing.T, ev model.InferenceEvent) {
				t.Helper()
				if ev.TenantID != "acme" {
					t.Errorf("TenantID = %q, want acme", ev.TenantID)
				}
				if ev.LatencyMs != 50 {
					t.Errorf("LatencyMs = %d, want 50", ev.LatencyMs)
				}
			},
		},
		{
			name: "multiple events",
			payload: `{"events":[
				{"tenant_id":"t1","model_id":"m","timestamp_unix_ms":1,"latency_ms":1,"prompt_tokens":1,"completion_tokens":1,"cost_usd":0},
				{"tenant_id":"t2","model_id":"m","timestamp_unix_ms":2,"latency_ms":2,"prompt_tokens":2,"completion_tokens":2,"cost_usd":0}
			]}`,
			wantCount: 2,
		},
		{
			name:      "empty events array",
			payload:   `{"events":[]}`,
			wantCount: 0,
		},
		{
			name: "optional fields absent (event_id, status, error_code)",
			payload: `{"events":[{
				"tenant_id":"t","model_id":"m","timestamp_unix_ms":1,
				"latency_ms":1,"prompt_tokens":1,"completion_tokens":1,"cost_usd":0
			}]}`,
			wantCount: 1,
			checkFirst: func(t *testing.T, ev model.InferenceEvent) {
				t.Helper()
				if ev.EventID != nil {
					t.Errorf("EventID should be nil, got %v", ev.EventID)
				}
				if ev.Status != nil {
					t.Errorf("Status should be nil, got %v", ev.Status)
				}
			},
		},
		{
			name:    "invalid json",
			payload: `not json`,
			wantErr: true,
		},
		{
			name:    "truncated json",
			payload: `{"events":[{"tenant_id"`,
			wantErr: true,
		},
		{
			name:    "wrong top-level type (array instead of object)",
			payload: `[]`,
			wantErr: true, // Go JSON returns an error when unmarshaling [] into a struct
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			batch, err := DeserializeBatch([]byte(tc.payload))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				// Error message must include the sentinel used by isDeserializeErr.
				if !strings.Contains(err.Error(), "unmarshal ingest batch") {
					t.Errorf("error %q does not contain sentinel", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(batch.Events) != tc.wantCount {
				t.Fatalf("event count = %d, want %d", len(batch.Events), tc.wantCount)
			}
			if tc.checkFirst != nil && len(batch.Events) > 0 {
				tc.checkFirst(t, batch.Events[0])
			}
		})
	}
}

// ── isDeserializeErr ──────────────────────────────────────────────────────────

func TestIsDeserializeErr(t *testing.T) {
	_, err := DeserializeBatch([]byte(`bad`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !isDeserializeErr(err) {
		t.Errorf("isDeserializeErr = false, want true for %v", err)
	}
	if isDeserializeErr(nil) {
		t.Error("isDeserializeErr(nil) should be false")
	}
}

// ── Sink mock for handleRecord unit tests ─────────────────────────────────────

// captureSink records all events passed to Accept.
type captureSink struct {
	events []model.InferenceEvent
	err    error
}

func (s *captureSink) Accept(_ context.Context, events []model.InferenceEvent) error {
	s.events = append(s.events, events...)
	return s.err
}

// TestHandleRecordHappyPath verifies that a well-formed Kafka record is
// deserialized and forwarded to the sink without error.
func TestHandleRecordHappyPath(t *testing.T) {
	sink := &captureSink{}
	r := &Reader{sink: sink, m: newTestMetrics()}

	payload := []byte(`{"events":[{
		"tenant_id":"demo","model_id":"gpt-4o","timestamp_unix_ms":1000,
		"latency_ms":1,"prompt_tokens":1,"completion_tokens":1,"cost_usd":0
	}]}`)

	if err := r.handleRecord(context.Background(), newFakeRecord(payload)); err != nil {
		t.Fatalf("handleRecord: %v", err)
	}
	if len(sink.events) != 1 {
		t.Fatalf("sink received %d events, want 1", len(sink.events))
	}
	if sink.events[0].TenantID != "demo" {
		t.Errorf("TenantID = %q", sink.events[0].TenantID)
	}
}

// TestHandleRecordMalformedPayload verifies deserialization errors are surfaced.
func TestHandleRecordMalformedPayload(t *testing.T) {
	sink := &captureSink{}
	r := &Reader{sink: sink, m: newTestMetrics()}

	err := r.handleRecord(context.Background(), newFakeRecord([]byte("bad json")))
	if err == nil {
		t.Fatal("expected error for malformed payload")
	}
	if !isDeserializeErr(err) {
		t.Errorf("error should be a deserialization error, got: %v", err)
	}
}

// TestHandleRecordMultipleEventsInOneBatch verifies that a single Kafka
// record containing multiple events delivers all events to the sink.
func TestHandleRecordMultipleEventsInOneBatch(t *testing.T) {
	sink := &captureSink{}
	r := &Reader{sink: sink, m: newTestMetrics()}

	payload := []byte(`{"events":[
		{"tenant_id":"t1","model_id":"m","timestamp_unix_ms":1,"latency_ms":1,"prompt_tokens":1,"completion_tokens":1,"cost_usd":0},
		{"tenant_id":"t2","model_id":"m","timestamp_unix_ms":2,"latency_ms":2,"prompt_tokens":2,"completion_tokens":2,"cost_usd":0},
		{"tenant_id":"t3","model_id":"m","timestamp_unix_ms":3,"latency_ms":3,"prompt_tokens":3,"completion_tokens":3,"cost_usd":0}
	]}`)

	if err := r.handleRecord(context.Background(), newFakeRecord(payload)); err != nil {
		t.Fatalf("handleRecord: %v", err)
	}
	if len(sink.events) != 3 {
		t.Fatalf("sink received %d events, want 3", len(sink.events))
	}
}

// TestHandleRecordSinkError verifies that a sink failure is propagated upward.
func TestHandleRecordSinkError(t *testing.T) {
	sink := &captureSink{err: context.DeadlineExceeded}
	r := &Reader{sink: sink, m: newTestMetrics()}

	payload := []byte(`{"events":[{
		"tenant_id":"t","model_id":"m","timestamp_unix_ms":1,
		"latency_ms":1,"prompt_tokens":1,"completion_tokens":1,"cost_usd":0
	}]}`)

	err := r.handleRecord(context.Background(), newFakeRecord(payload))
	if err == nil {
		t.Fatal("expected sink error to be propagated")
	}
}
