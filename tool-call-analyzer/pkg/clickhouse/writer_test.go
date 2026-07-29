// SPDX-License-Identifier: MIT
package clickhouse_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AkshantVats/tool-call-analyzer/pkg/clickhouse"
	"github.com/AkshantVats/tool-call-analyzer/pkg/types"
)

func TestWriterInsert(t *testing.T) {
	var receivedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	writer := clickhouse.NewWithClient(srv.URL, srv.Client())

	tc := types.ToolCall{
		ID:       "tcall-001",
		TraceID:  "trace-abc",
		Name:     "search_web",
		Vendor:   "openai",
		Category: types.CategoryHTTP,
		Cost: types.CostEstimate{
			InputTokens:  512,
			OutputTokens: 64,
			ModelName:    "gpt-4o",
			CostUSD:      0.00192,
		},
	}

	if err := writer.Insert(context.Background(), tc, 120, 300, false, "OK"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(receivedBody, `"tool_name":"search_web"`) {
		t.Errorf("expected tool_name in body, got: %s", receivedBody)
	}
	if !strings.Contains(receivedBody, `"vendor":"openai"`) {
		t.Errorf("expected vendor in body, got: %s", receivedBody)
	}
	if !strings.Contains(receivedBody, `"model_name":"gpt-4o"`) {
		t.Errorf("expected model_name in body, got: %s", receivedBody)
	}
	if !strings.Contains(receivedBody, `"duration_ms":120`) {
		t.Errorf("expected duration_ms=120 in body, got: %s", receivedBody)
	}
	if !strings.Contains(receivedBody, `"trace_duration_ms":300`) {
		t.Errorf("expected trace_duration_ms=300 in body, got: %s", receivedBody)
	}
	if !strings.Contains(receivedBody, `"has_error":0`) {
		t.Errorf("expected has_error=0 in body, got: %s", receivedBody)
	}
	if !strings.Contains(receivedBody, `"status":"OK"`) {
		t.Errorf("expected status=OK in body, got: %s", receivedBody)
	}
}

func TestWriterInsertErrorCall(t *testing.T) {
	var receivedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	writer := clickhouse.NewWithClient(srv.URL, srv.Client())
	tc := types.ToolCall{Name: "search_db", Vendor: "anthropic", Category: types.CategoryDB}

	if err := writer.Insert(context.Background(), tc, 50, 400, true, "ERROR"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(receivedBody, `"has_error":1`) {
		t.Errorf("expected has_error=1 in body, got: %s", receivedBody)
	}
	if !strings.Contains(receivedBody, `"status":"ERROR"`) {
		t.Errorf("expected status=ERROR in body, got: %s", receivedBody)
	}
}

func TestWriterHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	writer := clickhouse.NewWithClient(srv.URL, srv.Client())
	tc := types.ToolCall{Name: "test", Vendor: "openai"}

	err := writer.Insert(context.Background(), tc, 100, 200, false, "OK")
	if err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 in error message, got: %v", err)
	}
}

func TestWriterZeroTraceDuration(t *testing.T) {
	// Zero trace_duration_ms is valid to insert -- the alert MV filters it via WHERE trace_duration_ms > 0.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	writer := clickhouse.NewWithClient(srv.URL, srv.Client())
	tc := types.ToolCall{Name: "test", Vendor: "openai"}

	if err := writer.Insert(context.Background(), tc, 100, 0, false, "OK"); err != nil {
		t.Fatalf("unexpected error for zero trace_duration_ms: %v", err)
	}
}
