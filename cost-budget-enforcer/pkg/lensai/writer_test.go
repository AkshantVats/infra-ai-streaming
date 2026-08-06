// SPDX-License-Identifier: MIT

package lensai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEmitSpendPostsExpectedEnvelope(t *testing.T) {
	var got ingestRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Tenant-ID") != "tenant-a" {
			t.Errorf("X-Tenant-ID header = %q, want tenant-a", r.Header.Get("X-Tenant-ID"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	writer := NewWithClient(srv.URL, srv.Client())
	err := writer.EmitSpend(context.Background(), "tenant-a", "gpt-4o", 0.0234, 480*time.Millisecond)
	if err != nil {
		t.Fatalf("EmitSpend: %v", err)
	}

	if len(got.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got.Events))
	}
	evt := got.Events[0]
	if evt.Source != SourceGatewayInference {
		t.Errorf("Source = %q, want %q", evt.Source, SourceGatewayInference)
	}
	if evt.ModelID != "gpt-4o" {
		t.Errorf("ModelID = %q, want gpt-4o", evt.ModelID)
	}
	if evt.CostUSD != 0.0234 {
		t.Errorf("CostUSD = %v, want 0.0234", evt.CostUSD)
	}
	if evt.LatencyMs != 480 {
		t.Errorf("LatencyMs = %d, want 480", evt.LatencyMs)
	}
}

func TestEmitSpendRequiresTenantAndModel(t *testing.T) {
	writer := New("http://unused.invalid/ingest")

	if err := writer.EmitSpend(context.Background(), "", "gpt-4o", 0.01, time.Millisecond); err == nil {
		t.Error("expected error for missing tenant_id, got nil")
	}
	if err := writer.EmitSpend(context.Background(), "tenant-a", "", 0.01, time.Millisecond); err == nil {
		t.Error("expected error for missing model_id, got nil")
	}
}

func TestEmitSpendPropagatesNon2xxStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	writer := NewWithClient(srv.URL, srv.Client())
	err := writer.EmitSpend(context.Background(), "tenant-a", "gpt-4o", 0.01, time.Millisecond)
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestEmitCacheHitPostsZeroCost(t *testing.T) {
	var got ingestRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	writer := NewWithClient(srv.URL, srv.Client())
	err := writer.EmitCacheHit(context.Background(), "tenant-a", "semantic-cache-lookup", "abc123hash", 5*time.Millisecond)
	if err != nil {
		t.Fatalf("EmitCacheHit: %v", err)
	}

	evt := got.Events[0]
	if evt.Source != SourceGatewayCacheHit {
		t.Errorf("Source = %q, want %q", evt.Source, SourceGatewayCacheHit)
	}
	if evt.CostUSD != 0 {
		t.Errorf("CostUSD = %v, want 0", evt.CostUSD)
	}
	if evt.TraceID != "abc123hash" {
		t.Errorf("TraceID = %q, want abc123hash", evt.TraceID)
	}
}

func TestEmitCacheHitRequiresTenant(t *testing.T) {
	writer := New("http://unused.invalid/ingest")

	if err := writer.EmitCacheHit(context.Background(), "", "model", "hash", time.Millisecond); err == nil {
		t.Error("expected error for missing tenant_id, got nil")
	}
}

func TestEmitBlockedPostsZeroCostAndBlockedStatus(t *testing.T) {
	var got ingestRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	writer := NewWithClient(srv.URL, srv.Client())
	err := writer.EmitBlocked(context.Background(), "tenant-a")
	if err != nil {
		t.Fatalf("EmitBlocked: %v", err)
	}

	evt := got.Events[0]
	if evt.Source != SourceGatewayBlocked {
		t.Errorf("Source = %q, want %q", evt.Source, SourceGatewayBlocked)
	}
	if evt.Status != StatusBlocked {
		t.Errorf("Status = %q, want %q", evt.Status, StatusBlocked)
	}
	if evt.CostUSD != 0 {
		t.Errorf("CostUSD = %v, want 0", evt.CostUSD)
	}
}

func TestEmitBlockedRequiresTenant(t *testing.T) {
	writer := New("http://unused.invalid/ingest")

	if err := writer.EmitBlocked(context.Background(), ""); err == nil {
		t.Error("expected error for missing tenant_id, got nil")
	}
}

func TestEmitPropagatesNon2xxStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	writer := NewWithClient(srv.URL, srv.Client())
	if err := writer.EmitCacheHit(context.Background(), "tenant-a", "model", "hash", time.Millisecond); err == nil {
		t.Fatal("expected error for 500 response on EmitCacheHit, got nil")
	}
	if err := writer.EmitBlocked(context.Background(), "tenant-a"); err == nil {
		t.Fatal("expected error for 500 response on EmitBlocked, got nil")
	}
}
