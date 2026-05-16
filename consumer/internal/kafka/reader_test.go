package kafka

import (
	"testing"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/model"
)

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

func TestLogEventDoesNotPanic(t *testing.T) {
	LogEvent(model.InferenceEvent{
		TenantID:         "demo",
		ModelID:          "gpt-4o",
		PromptTokens:     512,
		CompletionTokens: 128,
		CostUSD:          0.00423,
		LatencyMs:        342,
	})
}
