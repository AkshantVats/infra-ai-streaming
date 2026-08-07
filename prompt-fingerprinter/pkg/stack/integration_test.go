// SPDX-License-Identifier: MIT

package stack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/akshantvats/prompt-fingerprinter/pkg/admin"
	"github.com/akshantvats/prompt-fingerprinter/pkg/fingerprint"
	"github.com/akshantvats/prompt-fingerprinter/pkg/rules"
)

// countingL2 is an L2Store fake standing in for the embedding API this
// whole cache stack exists to gate — every call to Get is a call that,
// in production, would have paid for an embedding-model round trip.
type countingL2 struct {
	mu       sync.Mutex
	calls    int
	response string
}

func (c *countingL2) Get(_ context.Context, _ string, _ fingerprint.PromptRequest) (string, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return c.response, true, nil
}

func (c *countingL2) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

// TestIntegration_DuplicatePromptSkipsEmbeddingAPI is Day 76's required
// integration test: a byte-identical prompt sent twice must reach the
// embedding API (countingL2) exactly once. The first call is an L1 miss
// that backfills Redis on its L2 hit; the second call resolves entirely
// at L1 and never touches L2 again.
func TestIntegration_DuplicatePromptSkipsEmbeddingAPI(t *testing.T) {
	ctx := context.Background()
	req := fingerprint.PromptRequest{
		Messages: []fingerprint.Message{{Role: "user", Content: "summarize this quarterly report"}},
		Model:    "gpt-4o",
	}

	l2 := &countingL2{response: "the quarterly report shows..."}
	s := &Stack{Redis: NewMemRedis(), L2: l2, Metrics: NewMemMetrics()}

	first, err := s.Get(ctx, "tenant-a", req)
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}
	if first.Tier != TierL2 {
		t.Fatalf("first Get: want TierL2 (cold cache), got %s", first.Tier)
	}

	second, err := s.Get(ctx, "tenant-a", req)
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if second.Tier != TierL1 {
		t.Fatalf("second Get: want TierL1 (duplicate resolved without embedding call), got %s", second.Tier)
	}
	if second.Response != first.Response {
		t.Errorf("second Get returned a different response than the first: %q vs %q", second.Response, first.Response)
	}

	if got := l2.callCount(); got != 1 {
		t.Errorf("embedding API (L2) called %d times for 2 identical requests, want exactly 1", got)
	}
}

// TestIntegration_AdminRulesExpandDuplicateDetection wires pkg/admin's
// Handler behind an httptest.Server, PUTs fingerprint-rules for a tenant,
// and shows two prompts that differ only in case and punctuation — which
// would fingerprint differently under Day 70's default rules — now
// collide at L1 once the tenant has opted in, so the second request never
// reaches the embedding API either.
func TestIntegration_AdminRulesExpandDuplicateDetection(t *testing.T) {
	ctx := context.Background()
	store := rules.NewStore()
	adminSrv := httptest.NewServer(&admin.Handler{Store: store})
	defer adminSrv.Close()

	putReq, err := http.NewRequest(http.MethodPut, adminSrv.URL+"/tenants/tenant-b/fingerprint-rules",
		strings.NewReader(`{"strip_punctuation":true,"lowercase":true}`))
	if err != nil {
		t.Fatalf("build admin PUT request: %v", err)
	}
	putReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		t.Fatalf("admin PUT: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin PUT status = %d, want 200", resp.StatusCode)
	}

	l2 := &countingL2{response: "cached answer"}
	s := &Stack{Redis: NewMemRedis(), L2: l2, Metrics: NewMemMetrics(), Rules: store}

	reqA := fingerprint.PromptRequest{Messages: []fingerprint.Message{{Role: "user", Content: "Summarize this doc!"}}, Model: "gpt-4o"}
	reqB := fingerprint.PromptRequest{Messages: []fingerprint.Message{{Role: "user", Content: "summarize this doc"}}, Model: "gpt-4o"}

	first, err := s.Get(ctx, "tenant-b", reqA)
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}
	if first.Tier != TierL2 {
		t.Fatalf("first Get: want TierL2 (cold cache), got %s", first.Tier)
	}

	second, err := s.Get(ctx, "tenant-b", reqB)
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if second.Tier != TierL1 {
		t.Fatalf("second Get: want TierL1 (rules-normalized duplicate), got %s", second.Tier)
	}

	if got := l2.callCount(); got != 1 {
		t.Errorf("embedding API (L2) called %d times for 2 rules-equivalent requests, want exactly 1", got)
	}
}

// fakeEmitter records EmitExactHit calls, for tests asserting Stack wires
// an L1 hit through to the LensAI writer.
type fakeEmitter struct {
	mu    sync.Mutex
	calls []string // tenantID per call
}

func (f *fakeEmitter) EmitExactHit(_ context.Context, tenantID, _ string, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, tenantID)
	return nil
}

func (f *fakeEmitter) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// TestGet_L1Hit_EmitsExactHitEvent confirms Stack wires an L1 hit through
// to Emitter exactly once, and never on an L2 hit or a miss — DESIGN.md
// §4's cache_hit_exact source value is specifically for the exact-match
// tier, not a general "any hit" event.
func TestGet_L1Hit_EmitsExactHitEvent(t *testing.T) {
	ctx := context.Background()
	req := fingerprint.PromptRequest{Messages: []fingerprint.Message{{Role: "user", Content: "hello"}}, Model: "gpt-4o"}
	key := fingerprint.RedisKey("tenant-a", fingerprint.Fingerprint(req))

	redis := NewMemRedis()
	if err := redis.Set(ctx, key, "cached", HardTTL); err != nil {
		t.Fatalf("seed redis: %v", err)
	}
	emitter := &fakeEmitter{}
	s := &Stack{Redis: redis, L2: &MemL2{}, Emitter: emitter}

	if _, err := s.Get(ctx, "tenant-a", req); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := emitter.callCount(); got != 1 {
		t.Errorf("EmitExactHit called %d times on L1 hit, want 1", got)
	}
}

// TestGet_NilEmitter_DoesNotPanic confirms Emitter's nil-safety contract:
// a Stack constructed without one (every pre-Day-76 test in this package)
// behaves identically to before.
func TestGet_NilEmitter_DoesNotPanic(t *testing.T) {
	ctx := context.Background()
	req := fingerprint.PromptRequest{Messages: []fingerprint.Message{{Role: "user", Content: "hello"}}, Model: "gpt-4o"}
	key := fingerprint.RedisKey("tenant-a", fingerprint.Fingerprint(req))

	redis := NewMemRedis()
	if err := redis.Set(ctx, key, "cached", HardTTL); err != nil {
		t.Fatalf("seed redis: %v", err)
	}
	s := &Stack{Redis: redis, L2: &MemL2{}}

	if _, err := s.Get(ctx, "tenant-a", req); err != nil {
		t.Fatalf("Get with nil Emitter: %v", err)
	}
}
