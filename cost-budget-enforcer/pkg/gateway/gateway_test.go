// SPDX-License-Identifier: MIT

package gateway

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"github.com/akshantvats/cost-budget-enforcer/pkg/config"
	"github.com/akshantvats/cost-budget-enforcer/pkg/enforcer"
	"github.com/akshantvats/cost-budget-enforcer/pkg/store"
)

func testConfig() config.TenantConfig {
	return config.TenantConfig{
		BudgetTokens:   1000,
		WindowSeconds:  86400,
		FallbackModel:  "gpt-4o-mini",
		AlertThreshold: config.DefaultAlertThreshold,
		SoftThreshold:  config.DefaultSoftThreshold,
		HardThreshold:  config.DefaultHardThreshold,
	}
}

func newTestEnforcer(t *testing.T, now time.Time) *enforcer.Enforcer {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	return &enforcer.Enforcer{
		Store: store.NewRedisStore(rdb),
		Now:   func() time.Time { return now },
	}
}

// newChaosEnforcer returns an Enforcer backed by a miniredis instance the
// caller controls directly, so a test can call mr.Close() mid-run to
// simulate CHAOS.md Scenario 3 ("Redis lost") for the budget store path
// gateway.Handle guards.
func newChaosEnforcer(t *testing.T, now time.Time) (*enforcer.Enforcer, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	return &enforcer.Enforcer{
		Store: store.NewRedisStore(rdb),
		Now:   func() time.Time { return now },
	}, mr
}

// spyCache and spyModel record whether they were ever invoked, so a test
// can assert the DESIGN.md §6 ordering claim directly: a blocked request
// must never touch either one.
type spyCache struct {
	mu     sync.Mutex
	called bool
	result CacheResult
	err    error
}

func (s *spyCache) Lookup(ctx context.Context, tenantID, prompt string) (CacheResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.called = true
	return s.result, s.err
}

func (s *spyCache) wasCalled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.called
}

type spyModel struct {
	mu     sync.Mutex
	called bool
	result ModelResult
	err    error
}

func (s *spyModel) Call(ctx context.Context, tenantID, model, prompt string) (ModelResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.called = true
	return s.result, s.err
}

func (s *spyModel) wasCalled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.called
}

type spyEvents struct {
	mu       sync.Mutex
	spends   int
	hits     int
	blocked  int
	lastCost float64
}

func (s *spyEvents) EmitSpend(ctx context.Context, tenantID, modelID string, costUSD float64, latency time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spends++
	s.lastCost = costUSD
	return nil
}

func (s *spyEvents) EmitCacheHit(ctx context.Context, tenantID, modelID, matchedPromptHash string, latency time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hits++
	return nil
}

func (s *spyEvents) EmitBlocked(ctx context.Context, tenantID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blocked++
	return nil
}

func newGateway(t *testing.T, now time.Time, cache *spyCache, model *spyModel, events *spyEvents) *Gateway {
	t.Helper()
	return &Gateway{
		Enforcer: newTestEnforcer(t, now),
		Config:   func(string) config.TenantConfig { return testConfig() },
		Tokens:   func(string, string) int64 { return 100 },
		Cache:    cache,
		Model:    model,
		Events:   events,
	}
}

func TestHandleBlockedNeverTouchesCacheOrModel(t *testing.T) {
	now := time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC)
	cache := &spyCache{}
	model := &spyModel{}
	events := &spyEvents{}
	g := newGateway(t, now, cache, model, events)

	// Pre-spend 1300 tokens (130% of the 1000 budget) so the next check is
	// already over the hard limit.
	if _, err := g.Enforcer.Check(context.Background(), "tenant-a", 1300, testConfig()); err != nil {
		t.Fatalf("priming Check: %v", err)
	}

	result, err := g.Handle(context.Background(), "tenant-a", "gpt-4o", "hello")
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !result.Blocked {
		t.Fatalf("Blocked = false, want true at 130%% consumed")
	}
	if cache.wasCalled() {
		t.Error("Cache.Lookup was called on a blocked request — order violated, spend could have leaked")
	}
	if model.wasCalled() {
		t.Error("Model.Call was called on a blocked request — order violated, spend could have leaked")
	}
	if result.CostUSD != 0 {
		t.Errorf("CostUSD = %v, want 0 on a blocked request", result.CostUSD)
	}
	if events.blocked != 1 {
		t.Errorf("EmitBlocked called %d times, want 1", events.blocked)
	}
	if events.spends != 0 || events.hits != 0 {
		t.Errorf("spend=%d hit=%d events fired, want 0 both on a blocked request", events.spends, events.hits)
	}
}

func TestHandleCacheHitSkipsModelAndReportsZeroCost(t *testing.T) {
	now := time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC)
	cache := &spyCache{result: CacheResult{Hit: true, Response: "cached answer", MatchedPromptHash: "hash-1"}}
	model := &spyModel{result: ModelResult{Response: "should not be used", CostUSD: 5.00}}
	events := &spyEvents{}
	g := newGateway(t, now, cache, model, events)

	result, err := g.Handle(context.Background(), "tenant-a", "gpt-4o", "hello")
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !result.CacheHit {
		t.Fatal("CacheHit = false, want true")
	}
	if model.wasCalled() {
		t.Error("Model.Call was called despite a cache hit")
	}
	if result.CostUSD != 0 {
		t.Errorf("CostUSD = %v, want 0 on a cache hit", result.CostUSD)
	}
	if result.Response != "cached answer" {
		t.Errorf("Response = %q, want cached answer", result.Response)
	}
	if events.hits != 1 {
		t.Errorf("EmitCacheHit called %d times, want 1", events.hits)
	}
}

func TestHandleModelCallReportsActualCost(t *testing.T) {
	now := time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC)
	cache := &spyCache{result: CacheResult{Hit: false}}
	model := &spyModel{result: ModelResult{Response: "fresh answer", TokensUsed: 120, CostUSD: 0.0042}}
	events := &spyEvents{}
	g := newGateway(t, now, cache, model, events)

	result, err := g.Handle(context.Background(), "tenant-a", "gpt-4o", "hello")
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if result.Blocked || result.CacheHit {
		t.Fatalf("expected a plain model call, got Blocked=%v CacheHit=%v", result.Blocked, result.CacheHit)
	}
	if result.CostUSD != 0.0042 {
		t.Errorf("CostUSD = %v, want 0.0042", result.CostUSD)
	}
	if result.ModelUsed != "gpt-4o" {
		t.Errorf("ModelUsed = %q, want gpt-4o (not degraded)", result.ModelUsed)
	}
	if events.spends != 1 || events.lastCost != 0.0042 {
		t.Errorf("EmitSpend calls=%d lastCost=%v, want 1 and 0.0042", events.spends, events.lastCost)
	}
}

func TestHandleDegradedRewritesModelBeforeCacheAndModelCalls(t *testing.T) {
	now := time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC)
	cache := &spyCache{result: CacheResult{Hit: false}}
	var seenModel string
	model := &spyModel{}
	events := &spyEvents{}
	g := newGateway(t, now, cache, model, events)

	// Prime to exactly 100% consumed -- crosses the soft threshold.
	if _, err := g.Enforcer.Check(context.Background(), "tenant-a", 1000, testConfig()); err != nil {
		t.Fatalf("priming Check: %v", err)
	}

	// Wrap Model.Call in a small closure so we can capture which model name
	// the gateway actually forwarded, without changing the spy's interface.
	g.Model = modelFunc(func(ctx context.Context, tenantID, model, prompt string) (ModelResult, error) {
		seenModel = model
		return ModelResult{Response: "degraded answer", CostUSD: 0.001}, nil
	})

	result, err := g.Handle(context.Background(), "tenant-a", "gpt-4o", "hello")
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !result.Degraded {
		t.Fatal("Degraded = false, want true at 100%% consumed")
	}
	if result.ModelUsed != "gpt-4o-mini" {
		t.Errorf("ModelUsed = %q, want gpt-4o-mini (the configured fallback)", result.ModelUsed)
	}
	if seenModel != "gpt-4o-mini" {
		t.Errorf("model call was made with %q, want the fallback gpt-4o-mini", seenModel)
	}
}

func TestHandleRequiresTenantAndPrompt(t *testing.T) {
	now := time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC)
	g := newGateway(t, now, &spyCache{}, &spyModel{}, &spyEvents{})

	if _, err := g.Handle(context.Background(), "", "gpt-4o", "hello"); err == nil {
		t.Error("expected error for missing tenant_id, got nil")
	}
	if _, err := g.Handle(context.Background(), "tenant-a", "gpt-4o", ""); err == nil {
		t.Error("expected error for missing prompt, got nil")
	}
}

func TestHandlePropagatesModelCallError(t *testing.T) {
	now := time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC)
	cache := &spyCache{result: CacheResult{Hit: false}}
	model := &spyModel{err: fmt.Errorf("upstream timeout")}
	g := newGateway(t, now, cache, model, &spyEvents{})

	if _, err := g.Handle(context.Background(), "tenant-a", "gpt-4o", "hello"); err == nil {
		t.Fatal("expected error to propagate from Model.Call, got nil")
	}
}

// modelFunc adapts a function literal to ModelClient, used only where a
// test needs to observe an argument the spyModel type doesn't capture.
type modelFunc func(ctx context.Context, tenantID, model, prompt string) (ModelResult, error)

func (f modelFunc) Call(ctx context.Context, tenantID, model, prompt string) (ModelResult, error) {
	return f(ctx, tenantID, model, prompt)
}

// TestHandleChaosRedisDownFailsOpenByDefault reproduces CHAOS.md Scenario 3
// for the gateway path: with FailClosed left false, a request that arrives
// while the budget Store is unreachable still reaches Cache and Model — the
// enforcement check is skipped, not the request.
func TestHandleChaosRedisDownFailsOpenByDefault(t *testing.T) {
	now := time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC)
	enf, mr := newChaosEnforcer(t, now)
	mr.Close()

	cache := &spyCache{result: CacheResult{Hit: false}}
	model := &spyModel{result: ModelResult{Response: "ok", TokensUsed: 10, CostUSD: 0.001}}
	events := &spyEvents{}
	g := &Gateway{
		Enforcer: enf,
		Config:   func(string) config.TenantConfig { return testConfig() },
		Tokens:   func(string, string) int64 { return 100 },
		Cache:    cache,
		Model:    model,
		Events:   events,
	}

	result, err := g.Handle(context.Background(), "tenant-a", "gpt-4o", "hello")
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if result.StoreUnavailable {
		t.Fatal("StoreUnavailable = true, want false (fail-open tenant)")
	}
	if !model.wasCalled() {
		t.Error("Model.Call was not called — fail-open should still reach the model")
	}
	if result.CostUSD != 0.001 {
		t.Errorf("CostUSD = %v, want 0.001 (the request went through)", result.CostUSD)
	}
}

// TestHandleChaosRedisDownFailsClosedWhenConfigured is the same outage for
// a tenant with config.TenantConfig.FailClosed set: Handle must stop before
// Cache or Model, the same way it does for Blocked.
func TestHandleChaosRedisDownFailsClosedWhenConfigured(t *testing.T) {
	now := time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC)
	enf, mr := newChaosEnforcer(t, now)
	mr.Close()

	cache := &spyCache{}
	model := &spyModel{}
	events := &spyEvents{}
	cfg := testConfig()
	cfg.FailClosed = true
	g := &Gateway{
		Enforcer: enf,
		Config:   func(string) config.TenantConfig { return cfg },
		Tokens:   func(string, string) int64 { return 100 },
		Cache:    cache,
		Model:    model,
		Events:   events,
	}

	result, err := g.Handle(context.Background(), "tenant-a", "gpt-4o", "hello")
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !result.StoreUnavailable {
		t.Fatal("StoreUnavailable = false, want true (fail-closed tenant)")
	}
	if cache.wasCalled() {
		t.Error("Cache.Lookup was called on a fail-closed store-unavailable request")
	}
	if model.wasCalled() {
		t.Error("Model.Call was called on a fail-closed store-unavailable request")
	}
	if result.CostUSD != 0 {
		t.Errorf("CostUSD = %v, want 0", result.CostUSD)
	}
	if events.spends != 0 || events.hits != 0 || events.blocked != 0 {
		t.Errorf("spend=%d hit=%d blocked=%d events fired, want 0 all (store-unavailable is not a budget block)",
			events.spends, events.hits, events.blocked)
	}
}
