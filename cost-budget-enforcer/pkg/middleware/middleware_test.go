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

func newTestMiddleware(t *testing.T, now time.Time, budget int64) *Middleware {
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
