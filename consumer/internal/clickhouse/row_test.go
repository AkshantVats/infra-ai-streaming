package clickhouse

import (
	"testing"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/model"
)

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
