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

func TestEmitExactHitPostsExpectedEnvelope(t *testing.T) {
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
	err := writer.EmitExactHit(context.Background(), "tenant-a", "fingerprint:tenant-a:abc123", 8*time.Microsecond)
	if err != nil {
		t.Fatalf("EmitExactHit: %v", err)
	}

	if len(got.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got.Events))
	}
	evt := got.Events[0]
	if evt.Source != SourceCacheHitExact {
		t.Errorf("Source = %q, want %q", evt.Source, SourceCacheHitExact)
	}
	if evt.CostUSD != 0 {
		t.Errorf("CostUSD = %v, want 0", evt.CostUSD)
	}
	if evt.TraceID != "fingerprint:tenant-a:abc123" {
		t.Errorf("TraceID = %q, want fingerprint key", evt.TraceID)
	}
	if evt.Status != "ok" {
		t.Errorf("Status = %q, want ok", evt.Status)
	}
}

func TestEmitExactHitRequiresTenant(t *testing.T) {
	writer := New("http://unused.invalid/ingest")
	if err := writer.EmitExactHit(context.Background(), "", "fp", time.Microsecond); err == nil {
		t.Error("expected error for missing tenant_id, got nil")
	}
}

func TestEmitExactHitPropagatesNon2xxStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	writer := NewWithClient(srv.URL, srv.Client())
	err := writer.EmitExactHit(context.Background(), "tenant-a", "fp", time.Microsecond)
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}
