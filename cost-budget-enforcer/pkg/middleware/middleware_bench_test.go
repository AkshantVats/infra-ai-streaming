// SPDX-License-Identifier: MIT

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"github.com/akshantvats/cost-budget-enforcer/pkg/config"
	"github.com/akshantvats/cost-budget-enforcer/pkg/enforcer"
	"github.com/akshantvats/cost-budget-enforcer/pkg/store"
)

// BenchmarkWrapPass measures the middleware's per-request overhead on the
// hot path: a tenant comfortably under budget, Action == Pass, no body
// rewrite. This is the cost every request pays, since Pass is the common
// case DESIGN.md §2 describes ("under 80% of budget passes traffic through
// unchanged").
func BenchmarkWrapPass(b *testing.B) {
	now := time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC)
	mw := newTestMiddleware(b, now, 1_000_000_000) // budget high enough that N iterations never cross a threshold
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := mw.Wrap(next)

	req := newRequest(1, "gpt-4o")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

// BenchmarkWrapDegrade measures the same path when the tenant has crossed
// the soft threshold: enforcer.Check still runs, plus rewriteModel's
// JSON decode/re-encode of the request body — the additional cost DESIGN.md
// §3's fallback-model rewrite adds over a plain Pass.
func BenchmarkWrapDegrade(b *testing.B) {
	now := time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC)
	mw := newTestMiddleware(b, now, 1000)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := mw.Wrap(next)

	// Every call requests 0 tokens beyond the initial prime so the tenant
	// stays pinned at the same weighted percentage (~105%, Degrade) across
	// all b.N iterations instead of drifting to Block.
	primeReq := newRequest(1050, "gpt-4o")
	handler.ServeHTTP(httptest.NewRecorder(), primeReq)
	req := newRequest(0, "gpt-4o")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

// BenchmarkWrapFailClosedStoreDown measures the 503 short-circuit path
// added by config.TenantConfig.FailClosed: no Redis round trip completes,
// so this is closer to a floor than the other two benchmarks — the cost of
// detecting the Store is unreachable and writing a fixed response.
//
// The redis client here disables both of go-redis's retry layers (command
// MaxRetries and the connection pool's separate DialerRetries, which
// defaults to 5 attempts 100ms apart) instead of reusing newChaosMiddleware's
// default-configured client: go-redis's default retry policy is the right
// choice for a real outage, but left enabled it would make this specific
// benchmark measure retry-backoff sleep time, not the middleware's own
// overhead — the thing this benchmark exists to isolate.
func BenchmarkWrapFailClosedStoreDown(b *testing.B) {
	now := time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC)
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatalf("miniredis.Run: %v", err)
	}
	rdb := goredis.NewClient(&goredis.Options{
		Addr:          mr.Addr(),
		MaxRetries:    -1, // disable go-redis's command-level retry
		DialerRetries: 1,  // disable the connection pool's own dial-retry loop (default 5, 100ms apart)
		DialTimeout:   20 * time.Millisecond,
		ReadTimeout:   20 * time.Millisecond,
	})
	b.Cleanup(func() { _ = rdb.Close() })
	mr.Close() // simulate Redis down before any traffic arrives

	cfg := config.TenantConfig{
		BudgetTokens:   1000,
		WindowSeconds:  86400,
		FallbackModel:  "gpt-4o-mini",
		AlertThreshold: config.DefaultAlertThreshold,
		SoftThreshold:  config.DefaultSoftThreshold,
		HardThreshold:  config.DefaultHardThreshold,
		FailClosed:     true,
	}
	mw := &Middleware{
		Enforcer: &enforcer.Enforcer{Store: store.NewRedisStore(rdb), Now: func() time.Time { return now }},
		Tenant:   func(r *http.Request) string { return r.Header.Get("X-Tenant-Id") },
		Tokens:   func(tenantID string, r *http.Request) int64 { return 1 },
		Config:   func(tenantID string) config.TenantConfig { return cfg },
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := mw.Wrap(next)
	req := newRequest(1, "gpt-4o")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}
