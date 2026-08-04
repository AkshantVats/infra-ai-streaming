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

func TestEmitCacheHitPostsExpectedEnvelope(t *testing.T) {
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
	err := writer.EmitCacheHit(context.Background(), "tenant-a", "semantic-cache-lookup", "abc123hash", 12*time.Millisecond)
	if err != nil {
		t.Fatalf("EmitCacheHit: %v", err)
	}

	if len(got.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got.Events))
	}
	evt := got.Events[0]
	if evt.Source != SourceCacheHit {
		t.Errorf("Source = %q, want %q", evt.Source, SourceCacheHit)
	}
	if evt.TraceID != "abc123hash" {
		t.Errorf("TraceID = %q, want abc123hash", evt.TraceID)
	}
	if evt.CostUSD != 0 {
		t.Errorf("CostUSD = %v, want 0", evt.CostUSD)
	}
	if evt.LatencyMs != 12 {
		t.Errorf("LatencyMs = %d, want 12", evt.LatencyMs)
	}
}

func TestEmitCacheHitRequiresTenantAndHash(t *testing.T) {
	writer := New("http://unused.invalid/ingest")

	if err := writer.EmitCacheHit(context.Background(), "", "model", "hash", time.Millisecond); err == nil {
		t.Error("expected error for missing tenant_id, got nil")
	}
	if err := writer.EmitCacheHit(context.Background(), "tenant-a", "model", "", time.Millisecond); err == nil {
		t.Error("expected error for missing matched prompt hash, got nil")
	}
}

func TestEmitCacheHitPropagatesNon2xxStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	writer := NewWithClient(srv.URL, srv.Client())
	err := writer.EmitCacheHit(context.Background(), "tenant-a", "model", "hash", time.Millisecond)
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestEmitCacheMissPostsExpectedEnvelope(t *testing.T) {
	var got ingestRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	writer := NewWithClient(srv.URL, srv.Client())
	err := writer.EmitCacheMiss(context.Background(), "tenant-a", "semantic-cache-lookup", 3*time.Millisecond)
	if err != nil {
		t.Fatalf("EmitCacheMiss: %v", err)
	}

	if len(got.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got.Events))
	}
	evt := got.Events[0]
	if evt.Source != SourceCacheMiss {
		t.Errorf("Source = %q, want %q", evt.Source, SourceCacheMiss)
	}
	if evt.TraceID != "" {
		t.Errorf("TraceID = %q, want empty (no matched entry on a miss)", evt.TraceID)
	}
	if evt.LatencyMs != 3 {
		t.Errorf("LatencyMs = %d, want 3", evt.LatencyMs)
	}
}

func TestEmitCacheMissRequiresTenant(t *testing.T) {
	writer := New("http://unused.invalid/ingest")

	if err := writer.EmitCacheMiss(context.Background(), "", "model", time.Millisecond); err == nil {
		t.Error("expected error for missing tenant_id, got nil")
	}
}

func TestEmitCacheMissPropagatesNon2xxStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	writer := NewWithClient(srv.URL, srv.Client())
	err := writer.EmitCacheMiss(context.Background(), "tenant-a", "model", time.Millisecond)
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestEmitCacheFeedbackPostsExpectedEnvelope(t *testing.T) {
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
	err := writer.EmitCacheFeedback(context.Background(), "tenant-a", "semantic-cache-lookup", "abc123hash")
	if err != nil {
		t.Fatalf("EmitCacheFeedback: %v", err)
	}

	if len(got.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got.Events))
	}
	evt := got.Events[0]
	if evt.Source != SourceCacheFeedback {
		t.Errorf("Source = %q, want %q", evt.Source, SourceCacheFeedback)
	}
	if evt.Status != StatusThumbsDown {
		t.Errorf("Status = %q, want %q", evt.Status, StatusThumbsDown)
	}
	if evt.TraceID != "abc123hash" {
		t.Errorf("TraceID = %q, want abc123hash", evt.TraceID)
	}
	if evt.CostUSD != 0 {
		t.Errorf("CostUSD = %v, want 0", evt.CostUSD)
	}
}

func TestEmitCacheFeedbackRequiresTenantAndHash(t *testing.T) {
	writer := New("http://unused.invalid/ingest")

	if err := writer.EmitCacheFeedback(context.Background(), "", "model", "hash"); err == nil {
		t.Error("expected error for missing tenant_id, got nil")
	}
	if err := writer.EmitCacheFeedback(context.Background(), "tenant-a", "model", ""); err == nil {
		t.Error("expected error for missing matched prompt hash, got nil")
	}
}

func TestEmitCacheFeedbackPropagatesNon2xxStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	writer := NewWithClient(srv.URL, srv.Client())
	err := writer.EmitCacheFeedback(context.Background(), "tenant-a", "model", "hash")
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}
