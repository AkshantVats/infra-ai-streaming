// SPDX-License-Identifier: MIT

package stack

import (
	"context"
	"errors"
	"testing"

	"github.com/akshantvats/prompt-fingerprinter/pkg/fingerprint"
)

func testReq() fingerprint.PromptRequest {
	return fingerprint.PromptRequest{
		Messages: []fingerprint.Message{{Role: "user", Content: "summarize this doc"}},
		Model:    "gpt-4o",
	}
}

func TestGet_L1Hit_SkipsL2(t *testing.T) {
	ctx := context.Background()
	req := testReq()
	key := fingerprint.RedisKey("tenant-a", fingerprint.Fingerprint(req))

	redis := NewMemRedis()
	if err := redis.Set(ctx, key, "cached response"); err != nil {
		t.Fatalf("seed redis: %v", err)
	}
	l2 := &MemL2{Responses: map[string]string{"tenant-a": "should never be returned"}}
	metrics := NewMemMetrics()

	s := &Stack{Redis: redis, L2: l2, Metrics: metrics}
	result, err := s.Get(ctx, "tenant-a", req)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !result.Hit || result.Tier != TierL1 {
		t.Fatalf("want L1 hit, got %+v", result)
	}
	if result.Response != "cached response" {
		t.Fatalf("want L1's response, got %q", result.Response)
	}
	if metrics.L1Hits["tenant-a"] != 1 || metrics.L2Hits["tenant-a"] != 0 {
		t.Fatalf("want 1 l1_hit and 0 l2_hit, got %+v / %+v", metrics.L1Hits, metrics.L2Hits)
	}
}

func TestGet_L1Miss_L2Hit_BackfillsL1(t *testing.T) {
	ctx := context.Background()
	req := testReq()
	key := fingerprint.RedisKey("tenant-b", fingerprint.Fingerprint(req))

	redis := NewMemRedis()
	l2 := &MemL2{Responses: map[string]string{"tenant-b": "semantic response"}}
	metrics := NewMemMetrics()

	s := &Stack{Redis: redis, L2: l2, Metrics: metrics}
	result, err := s.Get(ctx, "tenant-b", req)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !result.Hit || result.Tier != TierL2 {
		t.Fatalf("want L2 hit, got %+v", result)
	}
	if result.Response != "semantic response" {
		t.Fatalf("want L2's response, got %q", result.Response)
	}
	if metrics.L1Hits["tenant-b"] != 0 || metrics.L2Hits["tenant-b"] != 1 {
		t.Fatalf("want 0 l1_hit and 1 l2_hit, got %+v / %+v", metrics.L1Hits, metrics.L2Hits)
	}
	if !redis.Contains(key) {
		t.Fatalf("want L2 hit to backfill L1 at key %q", key)
	}

	// The next identical request should now resolve at L1.
	result2, err := s.Get(ctx, "tenant-b", req)
	if err != nil {
		t.Fatalf("Get (repeat): %v", err)
	}
	if result2.Tier != TierL1 {
		t.Fatalf("want repeat request to hit L1 after backfill, got %+v", result2)
	}
	if metrics.L1Hits["tenant-b"] != 1 {
		t.Fatalf("want backfilled repeat to count as l1_hit, got %+v", metrics.L1Hits)
	}
}

func TestGet_L1Miss_L2Miss_CountsMissOnly(t *testing.T) {
	ctx := context.Background()
	req := testReq()

	redis := NewMemRedis()
	l2 := &MemL2{Responses: map[string]string{}}
	metrics := NewMemMetrics()

	s := &Stack{Redis: redis, L2: l2, Metrics: metrics}
	result, err := s.Get(ctx, "tenant-c", req)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if result.Hit || result.Tier != TierMiss {
		t.Fatalf("want a miss, got %+v", result)
	}
	if metrics.L1Hits["tenant-c"] != 0 || metrics.L2Hits["tenant-c"] != 0 || metrics.Misses["tenant-c"] != 1 {
		t.Fatalf("want only 1 miss counted, got %+v / %+v / %+v", metrics.L1Hits, metrics.L2Hits, metrics.Misses)
	}
}

func TestGet_RedisError_FailsOpenToL2(t *testing.T) {
	ctx := context.Background()
	req := testReq()

	redis := NewMemRedis()
	redis.ErrOnGet = errors.New("connection refused")
	l2 := &MemL2{Responses: map[string]string{"tenant-d": "semantic response"}}
	metrics := NewMemMetrics()

	s := &Stack{Redis: redis, L2: l2, Metrics: metrics}
	result, err := s.Get(ctx, "tenant-d", req)
	if err != nil {
		t.Fatalf("Get: want fail-open, not an error: %v", err)
	}
	if !result.Hit || result.Tier != TierL2 {
		t.Fatalf("want fail-open to L2 hit, got %+v", result)
	}
	if metrics.L2Hits["tenant-d"] != 1 {
		t.Fatalf("want 1 l2_hit despite Redis error, got %+v", metrics.L2Hits)
	}
}

func TestGet_L2Error_PropagatesAsError(t *testing.T) {
	ctx := context.Background()
	req := testReq()

	redis := NewMemRedis()
	l2 := &MemL2{ErrTenant: "tenant-e"}
	metrics := NewMemMetrics()

	s := &Stack{Redis: redis, L2: l2, Metrics: metrics}
	_, err := s.Get(ctx, "tenant-e", req)
	if err == nil {
		t.Fatalf("want an error when L2 itself fails, got nil")
	}
}

func TestGet_EmptyTenantID_Errors(t *testing.T) {
	s := &Stack{Redis: NewMemRedis(), L2: &MemL2{}, Metrics: NewMemMetrics()}
	_, err := s.Get(context.Background(), "", testReq())
	if err == nil {
		t.Fatalf("want error for empty tenant_id, got nil")
	}
}

func TestGet_NilMetrics_DoesNotPanic(t *testing.T) {
	ctx := context.Background()
	req := testReq()
	s := &Stack{Redis: NewMemRedis(), L2: &MemL2{Responses: map[string]string{"tenant-f": "resp"}}}
	if _, err := s.Get(ctx, "tenant-f", req); err != nil {
		t.Fatalf("Get with nil Metrics: %v", err)
	}
}
