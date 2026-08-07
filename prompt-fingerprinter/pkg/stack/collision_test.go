// SPDX-License-Identifier: MIT

package stack

import (
	"context"
	"testing"
	"time"

	"github.com/akshantvats/prompt-fingerprinter/pkg/fingerprint"
)

// This file is Day 73's collision drill: an intentional exercise of
// what happens when two distinct prompts land at the same L1 Redis
// key. A real SHA-256 collision cannot be manufactured in a test (that
// is the entire point of using SHA-256, per DESIGN.md §2) — instead,
// each drill below directly seeds MemRedis at the key a collision
// would have produced, which exercises exactly the same code path
// Stack.Get would take if fingerprint.Fingerprint ever did collide.
// The drill is not testing whether a collision can happen; it is
// testing what this stack does to contain one when — cryptographically
// implausible as it is — it does.

// collisionKey returns the L1 key two distinct requests would share if
// their fingerprints collided under tenantID.
func collisionKey(tenantID string, req fingerprint.PromptRequest) string {
	return fingerprint.RedisKey(tenantID, fingerprint.Fingerprint(req))
}

// TestCollisionDrill_SharedKeyServesWrongResponse documents the raw
// failure mode a collision produces: L1 has no way to detect that the
// value at a key was written for a different request than the one
// asking for it now. This is not a bug to fix — an exact-match cache
// keyed by hash alone cannot distinguish "same prompt" from "different
// prompt, same hash" without storing the original prompt for
// comparison, which would defeat the point of a cheap Redis GET. The
// two tests after this one are what actually contain the blast radius
// this test demonstrates.
func TestCollisionDrill_SharedKeyServesWrongResponse(t *testing.T) {
	ctx := context.Background()
	reqA := fingerprint.PromptRequest{
		Messages: []fingerprint.Message{{Role: "user", Content: "summarize this doc"}},
		Model:    "gpt-4o",
	}
	reqB := fingerprint.PromptRequest{
		Messages: []fingerprint.Message{{Role: "user", Content: "translate this doc to French"}},
		Model:    "gpt-4o",
	}

	redis := NewMemRedis()
	// Simulate the collision directly: write reqB's response under the
	// key reqA's own (real, non-colliding) fingerprint maps to. This is
	// the state the system would be in if fingerprint.Fingerprint(reqA)
	// had, against 2^-256 odds, equalled fingerprint.Fingerprint(reqB).
	reqBResponse := "translated response for: " + reqB.Messages[0].Content
	key := collisionKey("tenant-a", reqA)
	if err := redis.Set(ctx, key, reqBResponse, HardTTL); err != nil {
		t.Fatalf("seed collision: %v", err)
	}

	s := &Stack{Redis: redis, L2: &MemL2{}, Metrics: NewMemMetrics()}
	result, err := s.Get(ctx, "tenant-a", reqA)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if result.Response != reqBResponse {
		t.Fatalf("want the collision's actual failure mode (reqA served reqB's response), got %q", result.Response)
	}
}

// TestCollisionDrill_TenantScopingContainsBlastRadius confirms a
// collision under one tenant's key cannot leak into a different
// tenant's, even for the identical prompt content. RedisKey folds
// tenantID into the key itself (fingerprint.RedisKey), so a collision
// is contained to the single tenant whose request produced it.
func TestCollisionDrill_TenantScopingContainsBlastRadius(t *testing.T) {
	ctx := context.Background()
	req := fingerprint.PromptRequest{
		Messages: []fingerprint.Message{{Role: "user", Content: "summarize this doc"}},
		Model:    "gpt-4o",
	}

	redis := NewMemRedis()
	corruptedKey := collisionKey("tenant-a", req)
	if err := redis.Set(ctx, corruptedKey, "corrupted response", HardTTL); err != nil {
		t.Fatalf("seed collision: %v", err)
	}

	l2 := &MemL2{Responses: map[string]string{"tenant-b": "tenant-b's real response"}}
	s := &Stack{Redis: redis, L2: l2, Metrics: NewMemMetrics()}

	// Same prompt content, different tenant: must not read tenant-a's
	// corrupted key.
	result, err := s.Get(ctx, "tenant-b", req)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if result.Tier != TierL2 || result.Response != "tenant-b's real response" {
		t.Fatalf("want tenant-b unaffected by tenant-a's collision (L2 hit with its own response), got %+v", result)
	}
}

// TestCollisionDrill_TTLBoundsBlastRadius confirms a corrupted entry
// does not persist indefinitely: once HardTTL elapses, MemRedis treats
// it as expired and Stack.Get falls through to L2, self-healing
// without any operator intervention. This is the mitigation DESIGN.md
// §3's "verify TTL isolation" calls for — a collision's damage is
// bounded in time even though it cannot be detected in advance.
func TestCollisionDrill_TTLBoundsBlastRadius(t *testing.T) {
	ctx := context.Background()
	req := fingerprint.PromptRequest{
		Messages: []fingerprint.Message{{Role: "user", Content: "summarize this doc"}},
		Model:    "gpt-4o",
	}

	clock := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	redis := NewMemRedisWithClock(clock)
	corruptedKey := collisionKey("tenant-a", req)
	if err := redis.Set(ctx, corruptedKey, "corrupted response", HardTTL); err != nil {
		t.Fatalf("seed collision: %v", err)
	}

	l2 := &MemL2{Responses: map[string]string{"tenant-a": "fresh, correct response"}}
	s := &Stack{Redis: redis, L2: l2, Metrics: NewMemMetrics()}

	// One second before the ceiling: still corrupted. This pins the
	// exact bound rather than a loose "eventually expires" assertion —
	// the collision's damage is bounded at exactly HardTTL, not
	// approximately.
	clock.Advance(HardTTL - time.Second)
	before, err := s.Get(ctx, "tenant-a", req)
	if err != nil {
		t.Fatalf("Get (before TTL): %v", err)
	}
	if before.Tier != TierL1 || before.Response != "corrupted response" {
		t.Fatalf("want the corrupted entry still served just before HardTTL, got %+v", before)
	}

	// Past the ceiling: MemRedis reports a miss, Stack falls through to
	// L2 and self-heals — the corrupted entry is gone, replaced by a
	// correct backfill.
	clock.Advance(2 * time.Second)
	after, err := s.Get(ctx, "tenant-a", req)
	if err != nil {
		t.Fatalf("Get (after TTL): %v", err)
	}
	if after.Tier != TierL2 || after.Response != "fresh, correct response" {
		t.Fatalf("want the corrupted entry expired and L2 consulted fresh, got %+v", after)
	}

	// The self-heal's backfill must itself carry a fresh TTL, not
	// immediately read back as expired.
	if !redis.Contains(corruptedKey) {
		t.Fatalf("want self-heal's backfill to still be present immediately after write")
	}
}
