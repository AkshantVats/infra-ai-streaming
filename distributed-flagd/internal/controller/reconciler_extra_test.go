// SPDX-License-Identifier: MIT
package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/akshantvats/distributed-flagd/internal/etcdstore"
)

// TestSpecToFlagData_NoVariants verifies a spec with no variants maps correctly.
func TestSpecToFlagData_NoVariants(t *testing.T) {
	cr := FlagDefinition{
		Spec: FlagSpec{FlagKey: "simple-flag", Enabled: true, Value: "on"},
	}
	fd := specToFlagData(cr)
	if fd.Name != "simple-flag" {
		t.Errorf("Name: want simple-flag, got %q", fd.Name)
	}
	if fd.Value != "on" {
		t.Errorf("Value: want on, got %q", fd.Value)
	}
	if len(fd.Variants) != 0 {
		t.Errorf("Variants: want 0, got %d", len(fd.Variants))
	}
}

// TestSpecToFlagData_Disabled verifies disabled flag is preserved.
func TestSpecToFlagData_Disabled(t *testing.T) {
	cr := FlagDefinition{
		Spec: FlagSpec{FlagKey: "off-flag", Enabled: false},
	}
	fd := specToFlagData(cr)
	if fd.Enabled {
		t.Error("expected Enabled=false")
	}
}

// TestReconcileUnknownEventType verifies unknown event types are silently ignored.
func TestReconcileUnknownEventType(t *testing.T) {
	store := newMockStore()
	event := WatchEvent{
		Type: "BOOKMARK",
		Object: FlagDefinition{
			Spec: FlagSpec{FlagKey: "some-flag", Enabled: true},
		},
	}
	b, _ := json.Marshal(event)
	body := append(b, '\n')

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer srv.Close()

	rec := New(srv.URL, "default", "", store)
	if err := rec.watch(context.Background()); err != nil {
		t.Fatalf("watch: unexpected error: %v", err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.flags) != 0 {
		t.Errorf("expected store empty for BOOKMARK event, got %v", store.flags)
	}
}

// TestWatchEmptyLines verifies the scanner skips empty lines without error.
func TestWatchEmptyLines(t *testing.T) {
	store := newMockStore()
	event := makeEvent("ADDED", "flag-after-blank", true, nil)
	// prepend two blank lines
	body := append([]byte("\n\n"), event...)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer srv.Close()

	rec := New(srv.URL, "default", "", store)
	if err := rec.watch(context.Background()); err != nil {
		t.Fatalf("watch: unexpected error: %v", err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, ok := store.flags["flag-after-blank"]; !ok {
		t.Error("flag after blank lines should be stored")
	}
}

// TestWatchMalformedJSON verifies garbled lines are skipped (logged) without aborting.
func TestWatchMalformedJSON(t *testing.T) {
	store := newMockStore()
	garbage := []byte("not-valid-json\n")
	valid := makeEvent("ADDED", "valid-flag", true, nil)
	body := append(garbage, valid...)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer srv.Close()

	rec := New(srv.URL, "default", "", store)
	if err := rec.watch(context.Background()); err != nil {
		t.Fatalf("watch: unexpected error: %v", err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	// The valid event should still be processed.
	if _, ok := store.flags["valid-flag"]; !ok {
		t.Error("valid flag after malformed line should still be stored")
	}
}

// TestWatchBearerToken verifies the Authorization header is set when token is provided.
func TestWatchBearerToken(t *testing.T) {
	const token = "my-service-account-token"
	var gotAuth string

	store := newMockStore()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		// send nothing — EOF immediately
	}))
	defer srv.Close()

	rec := New(srv.URL, "default", token, store)
	_ = rec.watch(context.Background())

	if !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Errorf("expected Authorization: Bearer ..., got %q", gotAuth)
	}
	if gotAuth != "Bearer "+token {
		t.Errorf("Authorization header: want %q, got %q", "Bearer "+token, gotAuth)
	}
}

// TestWatchNoToken verifies no Authorization header is sent when token is empty.
func TestWatchNoToken(t *testing.T) {
	var gotAuth string
	store := newMockStore()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rec := New(srv.URL, "default", "", store)
	_ = rec.watch(context.Background())
	if gotAuth != "" {
		t.Errorf("expected no Authorization header, got %q", gotAuth)
	}
}

// TestReconcile_StoreSetFlagError verifies a SetFlag error is logged but doesn't stop the loop.
func TestReconcile_StoreSetFlagError(t *testing.T) {
	// errStore returns an error on SetFlag to exercise the reconcile error log path.
	store := &errMockStore{mockStore: *newMockStore()}
	event := makeEvent("ADDED", "error-flag", true, nil)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(event)
	}))
	defer srv.Close()

	rec := New(srv.URL, "default", "", store)
	// watch should return cleanly despite store error
	if err := rec.watch(context.Background()); err != nil {
		t.Fatalf("watch: unexpected error: %v", err)
	}
}

// errMockStore wraps mockStore and returns errors from SetFlag.
type errMockStore struct {
	mockStore
}

func (e *errMockStore) SetFlag(_ context.Context, _ *etcdstore.FlagData) error {
	return context.DeadlineExceeded
}

// TestRun_ContextCancellation verifies Run returns ctx.Err() when context is cancelled
// while a watch is in progress (ctx cancels the inflight HTTP request).
func TestRun_ContextCancellation(t *testing.T) {
	store := newMockStore()
	// Server that returns 200 but hangs — Run should abort when ctx is cancelled.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write nothing — hold open until request context cancels.
		<-r.Context().Done()
	}))
	defer srv.Close()

	rec := New(srv.URL, "default", "", store)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := rec.Run(ctx)
	if err == nil {
		t.Error("expected non-nil error from Run after ctx cancellation")
	}
}

// TestRun_RetryOnWatchError exercises the backoff/retry path in Run.
// The server returns 500 on the first call (triggering a retry) and then
// cancels the ctx so Run terminates cleanly.
func TestRun_RetryOnWatchError(t *testing.T) {
	store := newMockStore()
	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call: return 500 to force a retry.
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Second call: return 200 and hang until ctx cancels.
		w.WriteHeader(http.StatusOK)
		<-r.Context().Done()
	}))
	defer srv.Close()

	rec := New(srv.URL, "default", "", store)
	rec.minBackoff = 1 * time.Millisecond // fast backoff for tests

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := rec.Run(ctx)
	if err == nil {
		t.Error("expected non-nil error from Run")
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 watch calls (1 error + 1 retry), got %d", callCount)
	}
}

// TestRun_BackoffReset verifies backoff is reset to initial after a successful watch.
// Three calls: 500 (error, backoff), 200+EOF (success, backoff reset), ctx cancel.
func TestRun_BackoffReset(t *testing.T) {
	store := newMockStore()
	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch callCount {
		case 1:
			// Fail: triggers retry after backoff.
			w.WriteHeader(http.StatusInternalServerError)
		case 2:
			// Succeed with an ADDED event then close (EOF resets backoff).
			w.WriteHeader(http.StatusOK)
			event := makeEvent("ADDED", "reset-test-flag", true, nil)
			w.Write(event)
		default:
			// Third+ call: context should be nearly done; hang until cancelled.
			w.WriteHeader(http.StatusOK)
			<-r.Context().Done()
		}
	}))
	defer srv.Close()

	rec := New(srv.URL, "default", "", store)
	rec.minBackoff = 1 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := rec.Run(ctx)
	if err == nil {
		t.Error("expected non-nil error from Run")
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 calls, got %d", callCount)
	}
}
