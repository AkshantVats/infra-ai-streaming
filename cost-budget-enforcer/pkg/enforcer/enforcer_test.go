// SPDX-License-Identifier: MIT

package enforcer

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"github.com/akshantvats/cost-budget-enforcer/pkg/config"
	"github.com/akshantvats/cost-budget-enforcer/pkg/store"
)

func newTestEnforcer(t *testing.T, now time.Time) (*Enforcer, *fakeWebhook) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	fw := &fakeWebhook{}
	e := &Enforcer{
		Store:   store.NewRedisStore(rdb),
		Webhook: fw,
		Now:     func() time.Time { return now },
	}
	return e, fw
}

type fakeWebhook struct {
	mu    sync.Mutex
	calls []AlertPayload
}

func (f *fakeWebhook) Send(ctx context.Context, payload AlertPayload) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, payload)
	return nil
}

func (f *fakeWebhook) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func testConfig() config.TenantConfig {
	return config.TenantConfig{
		BudgetTokens:    1000,
		WindowSeconds:   86400,
		FallbackModel:   "gpt-4o-mini",
		AlertWebhookURL: "https://example.test/hooks/budget",
		AlertThreshold:  config.DefaultAlertThreshold,
		SoftThreshold:   config.DefaultSoftThreshold,
		HardThreshold:   config.DefaultHardThreshold,
	}
}

func TestCheckPassesUnderAlertThreshold(t *testing.T) {
	now := time.Date(2026, 8, 5, 1, 0, 0, 0, time.UTC)
	e, fw := newTestEnforcer(t, now)

	d, err := e.Check(context.Background(), "tenant-a", 500, testConfig())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if d.Action != Pass {
		t.Fatalf("Action = %v, want Pass at 50%% consumed", d.Action)
	}
	if fw.count() != 0 {
		t.Fatalf("webhook fired %d times, want 0 below alert threshold", fw.count())
	}
}

func TestCheckAlertsAndDebouncesWebhook(t *testing.T) {
	now := time.Date(2026, 8, 5, 1, 0, 0, 0, time.UTC)
	e, fw := newTestEnforcer(t, now)
	cfg := testConfig()

	d, err := e.Check(context.Background(), "tenant-b", 850, cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if d.Action != Alert {
		t.Fatalf("Action = %v, want Alert at 85%% consumed", d.Action)
	}
	if !d.FireWebhook {
		t.Fatalf("FireWebhook = false on first crossing, want true")
	}
	if fw.count() != 1 {
		t.Fatalf("webhook fired %d times, want 1", fw.count())
	}

	// A second request in the same window, still above alert but below
	// soft, must not fire a second webhook (DESIGN.md §4 debounce).
	d, err = e.Check(context.Background(), "tenant-b", 10, cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if d.Action != Alert {
		t.Fatalf("Action = %v, want Alert on second call", d.Action)
	}
	if d.FireWebhook {
		t.Fatalf("FireWebhook = true on debounced call, want false")
	}
	if fw.count() != 1 {
		t.Fatalf("webhook fired %d times after debounced call, want still 1", fw.count())
	}
}

func TestCheckDegradesAtSoftLimit(t *testing.T) {
	now := time.Date(2026, 8, 5, 1, 0, 0, 0, time.UTC)
	e, _ := newTestEnforcer(t, now)

	d, err := e.Check(context.Background(), "tenant-c", 1050, testConfig())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if d.Action != Degrade {
		t.Fatalf("Action = %v, want Degrade at 105%% consumed", d.Action)
	}
	if d.FallbackModel != "gpt-4o-mini" {
		t.Fatalf("FallbackModel = %q, want configured fallback", d.FallbackModel)
	}
}

func TestCheckBlocksAtHardLimitWithRetryAfter(t *testing.T) {
	now := time.Date(2026, 8, 5, 1, 0, 0, 0, time.UTC)
	e, _ := newTestEnforcer(t, now)

	d, err := e.Check(context.Background(), "tenant-d", 1300, testConfig())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if d.Action != Block {
		t.Fatalf("Action = %v, want Block at 130%% consumed", d.Action)
	}
	if d.RetryAfter <= 0 || d.RetryAfter > 24*time.Hour {
		t.Fatalf("RetryAfter = %v, want a positive duration within the window", d.RetryAfter)
	}
	// now is 01:00 UTC into an 86400s window starting at midnight, so
	// 23 hours remain until reset.
	want := 23 * time.Hour
	if diff := d.RetryAfter - want; diff > time.Second || diff < -time.Second {
		t.Fatalf("RetryAfter = %v, want ~%v", d.RetryAfter, want)
	}
}

func TestCheckPassesWithNoBudgetConfigured(t *testing.T) {
	now := time.Date(2026, 8, 5, 1, 0, 0, 0, time.UTC)
	e, fw := newTestEnforcer(t, now)

	d, err := e.Check(context.Background(), "tenant-e", 999999, config.TenantConfig{})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if d.Action != Pass {
		t.Fatalf("Action = %v, want Pass when no budget is configured", d.Action)
	}
	if fw.count() != 0 {
		t.Fatalf("webhook fired %d times, want 0 when no budget is configured", fw.count())
	}
}
