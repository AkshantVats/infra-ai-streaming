// SPDX-License-Identifier: MIT

package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"github.com/akshantvats/cost-budget-enforcer/pkg/config"
	"github.com/akshantvats/cost-budget-enforcer/pkg/enforcer"
	"github.com/akshantvats/cost-budget-enforcer/pkg/store"
)

type noopWebhook struct{}

func (noopWebhook) Send(ctx context.Context, payload enforcer.AlertPayload) error { return nil }

func newTestMiddleware(t testing.TB, now time.Time, budget int64) *Middleware {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	e := &enforcer.Enforcer{
		Store:   store.NewRedisStore(rdb),
		Webhook: noopWebhook{},
		Now:     func() time.Time { return now },
	}
	cfg := config.TenantConfig{
		BudgetTokens:   budget,
		WindowSeconds:  86400,
		FallbackModel:  "gpt-4o-mini",
		AlertThreshold: config.DefaultAlertThreshold,
		SoftThreshold:  config.DefaultSoftThreshold,
		HardThreshold:  config.DefaultHardThreshold,
	}

	return &Middleware{
		Enforcer: e,
		Tenant:   func(r *http.Request) string { return r.Header.Get("X-Tenant-Id") },
		Tokens: func(tenantID string, r *http.Request) int64 {
			n, _ := strconv.ParseInt(r.Header.Get("X-Tokens-Estimate"), 10, 64)
			return n
		},
		Config: func(tenantID string) config.TenantConfig { return cfg },
	}
}

// newChaosMiddleware returns a Middleware backed by a miniredis instance the
// caller controls directly (not torn down via t.Cleanup), so a test can call
// mr.Close() mid-run to simulate Scenario 3 of CHAOS.md — "Redis lost" — for
// the budget store specifically, rather than the rate-limit path CHAOS.md
// already documents.
func newChaosMiddleware(t testing.TB, now time.Time, budget int64, failClosed bool) (*Middleware, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	e := &enforcer.Enforcer{
		Store:   store.NewRedisStore(rdb),
		Webhook: noopWebhook{},
		Now:     func() time.Time { return now },
	}
	cfg := config.TenantConfig{
		BudgetTokens:   budget,
		WindowSeconds:  86400,
		FallbackModel:  "gpt-4o-mini",
		AlertThreshold: config.DefaultAlertThreshold,
		SoftThreshold:  config.DefaultSoftThreshold,
		HardThreshold:  config.DefaultHardThreshold,
		FailClosed:     failClosed,
	}

	mw := &Middleware{
		Enforcer: e,
		Tenant:   func(r *http.Request) string { return r.Header.Get("X-Tenant-Id") },
		Tokens: func(tenantID string, r *http.Request) int64 {
			n, _ := strconv.ParseInt(r.Header.Get("X-Tokens-Estimate"), 10, 64)
			return n
		},
		Config: func(tenantID string) config.TenantConfig { return cfg },
	}
	return mw, mr
}

// TestWrapChaosRedisDownFailsOpenByDefault reproduces CHAOS.md Scenario 3
// against the budget store: with FailClosed left at its zero value, a
// request that arrives after Redis has gone away is still forwarded, the
// same availability-over-fairness bet the existing "Fail open" comment in
// Wrap documents.
func TestWrapChaosRedisDownFailsOpenByDefault(t *testing.T) {
	now := time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC)
	mw, mr := newChaosMiddleware(t, now, 1000, false)
	mr.Close() // simulate Redis down before any traffic arrives

	var reachedNext bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reachedNext = true
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	mw.Wrap(next).ServeHTTP(rec, newRequest(100, "gpt-4o"))

	if !reachedNext {
		t.Fatalf("fail-open tenant: next handler was not called with Redis down")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("fail-open tenant: status = %d, want 200", rec.Code)
	}
}

// TestWrapChaosRedisDownFailsClosedWhenConfigured is the same outage against
// a tenant that opted into config.TenantConfig.FailClosed: the request must
// be rejected with 503 rather than forwarded unmetered.
func TestWrapChaosRedisDownFailsClosedWhenConfigured(t *testing.T) {
	now := time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC)
	mw, mr := newChaosMiddleware(t, now, 1000, true)
	mr.Close() // simulate Redis down before any traffic arrives

	var reachedNext bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reachedNext = true
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	mw.Wrap(next).ServeHTTP(rec, newRequest(100, "gpt-4o"))

	if reachedNext {
		t.Fatalf("fail-closed tenant: next handler was called with Redis down")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("fail-closed tenant: status = %d, want 503", rec.Code)
	}
}

// TestWrapChaosRedisRecoversResumesEnforcement completes CHAOS.md's
// four-scene story for the budget store: once Redis comes back, a
// fail-closed tenant's requests are enforced normally again — no restart,
// no stuck-open state — the same automatic recovery Scenario 3 documents
// for the rate-limit path.
func TestWrapChaosRedisRecoversResumesEnforcement(t *testing.T) {
	now := time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC)
	mw, mr := newChaosMiddleware(t, now, 1000, true)
	mr.Close()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	rec := httptest.NewRecorder()
	mw.Wrap(next).ServeHTTP(rec, newRequest(100, "gpt-4o"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("during outage: status = %d, want 503", rec.Code)
	}

	restarted, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run (restart): %v", err)
	}
	t.Cleanup(restarted.Close)
	rdb := goredis.NewClient(&goredis.Options{Addr: restarted.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	mw.Enforcer.Store = store.NewRedisStore(rdb)

	rec2 := httptest.NewRecorder()
	mw.Wrap(next).ServeHTTP(rec2, newRequest(100, "gpt-4o"))
	if rec2.Code != http.StatusOK {
		t.Fatalf("after recovery: status = %d, want 200 (enforcement resumed, under budget)", rec2.Code)
	}
}

func newRequest(tokens int64, model string) *http.Request {
	body, _ := json.Marshal(map[string]any{"model": model, "prompt": "hello"})
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	r.Header.Set("X-Tenant-Id", "tenant-mw")
	r.Header.Set("X-Tokens-Estimate", strconv.FormatInt(tokens, 10))
	return r
}

func TestWrapForwardsUnderBudget(t *testing.T) {
	now := time.Date(2026, 8, 5, 1, 0, 0, 0, time.UTC)
	mw := newTestMiddleware(t, now, 1000)

	var reachedNext bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reachedNext = true
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	mw.Wrap(next).ServeHTTP(rec, newRequest(100, "gpt-4o"))

	if !reachedNext {
		t.Fatalf("next handler was not called for an under-budget request")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestWrapDegradesRewritesModel(t *testing.T) {
	now := time.Date(2026, 8, 5, 1, 0, 0, 0, time.UTC)
	mw := newTestMiddleware(t, now, 1000)

	var gotModel string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(b, &payload)
		gotModel, _ = payload["model"].(string)
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	// 1050/1000 = 105% -> Degrade
	mw.Wrap(next).ServeHTTP(rec, newRequest(1050, "gpt-4o"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (degrade still forwards)", rec.Code)
	}
	if gotModel != "gpt-4o-mini" {
		t.Fatalf("model reaching next = %q, want fallback %q", gotModel, "gpt-4o-mini")
	}
}

func TestWrapBlocksOverHardLimit(t *testing.T) {
	now := time.Date(2026, 8, 5, 1, 0, 0, 0, time.UTC)
	mw := newTestMiddleware(t, now, 1000)

	var reachedNext bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reachedNext = true
	})

	rec := httptest.NewRecorder()
	// 1300/1000 = 130% -> Block
	mw.Wrap(next).ServeHTTP(rec, newRequest(1300, "gpt-4o"))

	if reachedNext {
		t.Fatalf("next handler was called for a blocked request")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatalf("Retry-After header missing on a blocked response")
	}
}

func TestWrapPassesThroughRequestsWithNoTenant(t *testing.T) {
	now := time.Date(2026, 8, 5, 1, 0, 0, 0, time.UTC)
	mw := newTestMiddleware(t, now, 1000)

	var reachedNext bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reachedNext = true
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	rec := httptest.NewRecorder()
	mw.Wrap(next).ServeHTTP(rec, r)

	if !reachedNext {
		t.Fatalf("next handler was not called for a request with no tenant header")
	}
}
