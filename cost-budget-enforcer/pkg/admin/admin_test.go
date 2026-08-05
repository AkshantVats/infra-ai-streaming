// SPDX-License-Identifier: MIT

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/akshantvats/cost-budget-enforcer/pkg/audit"
	"github.com/akshantvats/cost-budget-enforcer/pkg/config"
)

type fakePublisher struct {
	mu      sync.Mutex
	calls   []audit.BudgetChangeEvent
	failErr error
}

func (f *fakePublisher) Publish(ctx context.Context, event audit.BudgetChangeEvent) error {
	if f.failErr != nil {
		return f.failErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, event)
	return nil
}

func (f *fakePublisher) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func newTestHandler(pub *fakePublisher) (*Handler, *config.LiveStore) {
	store := config.NewLiveStore(config.Config{
		Default: config.TenantConfig{
			BudgetTokens:   1_000_000,
			WindowSeconds:  86400,
			FallbackModel:  "gpt-4o-mini",
			AlertThreshold: 0.8,
			SoftThreshold:  1.0,
			HardThreshold:  1.2,
		},
	})
	h := &Handler{
		Store: store,
		Audit: pub,
		Now:   func() time.Time { return time.Unix(1710000000, 0) },
	}
	return h, store
}

func patchRequest(t *testing.T, path, body string) *http.Request {
	t.Helper()
	return httptest.NewRequest(http.MethodPatch, path, bytes.NewBufferString(body))
}

func TestPatchAppliesPartialUpdate(t *testing.T) {
	pub := &fakePublisher{}
	h, store := newTestHandler(pub)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, patchRequest(t, "/tenants/acme/budget", `{"budget_tokens": 5000000}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var got config.TenantConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.BudgetTokens != 5_000_000 {
		t.Fatalf("BudgetTokens = %d, want 5000000", got.BudgetTokens)
	}
	// Fields not in the patch carry over from the default.
	if got.FallbackModel != "gpt-4o-mini" {
		t.Fatalf("FallbackModel = %q, want carried-over default", got.FallbackModel)
	}

	if store.ForTenant("acme").BudgetTokens != 5_000_000 {
		t.Fatalf("store not updated")
	}
	if pub.count() != 1 {
		t.Fatalf("audit publish count = %d, want 1", pub.count())
	}
	got0 := pub.calls[0]
	if got0.TenantID != "acme" || got0.Actor != "unknown" {
		t.Fatalf("audit event = %+v", got0)
	}
	if got0.Before["budget_tokens"] != float64(1_000_000) || got0.After["budget_tokens"] != float64(5_000_000) {
		t.Fatalf("audit before/after = %v / %v", got0.Before, got0.After)
	}
}

func TestPatchRecordsActorHeader(t *testing.T) {
	pub := &fakePublisher{}
	h, _ := newTestHandler(pub)

	req := patchRequest(t, "/tenants/acme/budget", `{"budget_tokens": 2000000}`)
	req.Header.Set("X-Admin-Actor", "akshant@example.test")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if pub.calls[0].Actor != "akshant@example.test" {
		t.Fatalf("Actor = %q, want header value", pub.calls[0].Actor)
	}
}

func TestPatchRejectsInvalidThresholds(t *testing.T) {
	pub := &fakePublisher{}
	h, store := newTestHandler(pub)

	rec := httptest.NewRecorder()
	// hard (0.5) below soft (1.0) — invalid ordering.
	h.ServeHTTP(rec, patchRequest(t, "/tenants/acme/budget", `{"hard_threshold": 0.5}`))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422, body = %s", rec.Code, rec.Body.String())
	}
	if pub.count() != 0 {
		t.Fatalf("audit should not be called on a rejected patch, got %d calls", pub.count())
	}
	if store.ForTenant("acme").HardThreshold == 0.5 {
		t.Fatalf("store must not be mutated on a rejected patch")
	}
}

func TestPatchRejectsNonPatchMethod(t *testing.T) {
	pub := &fakePublisher{}
	h, _ := newTestHandler(pub)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tenants/acme/budget", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestPatchRejectsMalformedPath(t *testing.T) {
	pub := &fakePublisher{}
	h, _ := newTestHandler(pub)

	for _, path := range []string{"/tenants//budget", "/tenants/acme", "/budget", "/tenants/a/b/budget"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, patchRequest(t, path, `{}`))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("path %q: status = %d, want 400", path, rec.Code)
		}
	}
}

func TestPatchRejectsInvalidJSON(t *testing.T) {
	pub := &fakePublisher{}
	h, _ := newTestHandler(pub)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, patchRequest(t, "/tenants/acme/budget", `{not json`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestPatchRollsBackWhenAuditPublishFails(t *testing.T) {
	pub := &fakePublisher{failErr: errAuditDown}
	h, store := newTestHandler(pub)

	// Seed a known-good tenant config first without going through the
	// failing publisher, so we can assert the rollback restores it.
	store.Set("acme", config.TenantConfig{
		BudgetTokens: 1_000_000, WindowSeconds: 86400, FallbackModel: "gpt-4o-mini",
		AlertThreshold: 0.8, SoftThreshold: 1.0, HardThreshold: 1.2,
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, patchRequest(t, "/tenants/acme/budget", `{"budget_tokens": 9000000}`))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503, body = %s", rec.Code, rec.Body.String())
	}
	if got := store.ForTenant("acme").BudgetTokens; got != 1_000_000 {
		t.Fatalf("BudgetTokens after failed-audit patch = %d, want rolled back to 1000000", got)
	}
}

var errAuditDown = &staticError{"audit: broker unreachable"}

type staticError struct{ msg string }

func (e *staticError) Error() string { return e.msg }
