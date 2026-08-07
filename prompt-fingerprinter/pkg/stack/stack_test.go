// SPDX-License-Identifier: MIT

package stack

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/akshantvats/prompt-fingerprinter/pkg/fingerprint"
)

// spanAttr returns the string value of attribute key on the first
// recorded span, or "" if either is missing.
func spanAttr(t *testing.T, spans tracetest.SpanStubs, key string) string {
	t.Helper()
	if len(spans) == 0 {
		t.Fatalf("want at least one recorded span, got 0")
	}
	for _, kv := range spans[0].Attributes {
		if string(kv.Key) == key {
			return kv.Value.AsString()
		}
	}
	return ""
}

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
	if err := redis.Set(ctx, key, "cached response", HardTTL); err != nil {
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

// TestGet_Span_TaggedByTier exercises DESIGN.md §4's "own
// observability identity" claim for the span layer: an L1 hit, an L2
// hit, and a miss must each carry a distinguishable cache.tier
// attribute, not collapse into one undifferentiated span.
//
// The global TracerProvider can only be delegated once per process —
// package.tracer was obtained (at stack.go's package-init time) from
// the default no-op provider, and otel's global package permanently
// binds it to whichever provider the *first* call to
// otel.SetTracerProvider installs (see otel's internal/global,
// tracerProvider.setDelegate: "It is guaranteed by the caller that
// this happens only once"). So this test installs one provider for
// the whole subtest tree and resets the shared exporter between runs,
// rather than swapping providers per subtest.
func TestGet_Span_TaggedByTier(t *testing.T) {
	ctx := context.Background()

	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	run := func(t *testing.T, s *Stack, tenantID string) tracetest.SpanStubs {
		t.Helper()
		exporter.Reset()
		if _, err := s.Get(ctx, tenantID, testReq()); err != nil {
			t.Fatalf("Get: %v", err)
		}
		return exporter.GetSpans()
	}

	t.Run("l1_hit", func(t *testing.T) {
		redis := NewMemRedis()
		key := fingerprint.RedisKey("tenant-span-l1", fingerprint.Fingerprint(testReq()))
		if err := redis.Set(ctx, key, "cached", HardTTL); err != nil {
			t.Fatalf("seed redis: %v", err)
		}
		s := &Stack{Redis: redis, L2: &MemL2{}, Metrics: NewMemMetrics()}
		spans := run(t, s, "tenant-span-l1")
		if got := spanAttr(t, spans, "cache.tier"); got != string(TierL1) {
			t.Fatalf("cache.tier = %q, want %q", got, TierL1)
		}
	})

	t.Run("l2_hit", func(t *testing.T) {
		l2 := &MemL2{Responses: map[string]string{"tenant-span-l2": "semantic response"}}
		s := &Stack{Redis: NewMemRedis(), L2: l2, Metrics: NewMemMetrics()}
		spans := run(t, s, "tenant-span-l2")
		if got := spanAttr(t, spans, "cache.tier"); got != string(TierL2) {
			t.Fatalf("cache.tier = %q, want %q", got, TierL2)
		}
	})

	t.Run("miss", func(t *testing.T) {
		s := &Stack{Redis: NewMemRedis(), L2: &MemL2{}, Metrics: NewMemMetrics()}
		spans := run(t, s, "tenant-span-miss")
		if got := spanAttr(t, spans, "cache.tier"); got != string(TierMiss) {
			t.Fatalf("cache.tier = %q, want %q", got, TierMiss)
		}
	})
}

func TestGet_NilMetrics_DoesNotPanic(t *testing.T) {
	ctx := context.Background()
	req := testReq()
	s := &Stack{Redis: NewMemRedis(), L2: &MemL2{Responses: map[string]string{"tenant-f": "resp"}}}
	if _, err := s.Get(ctx, "tenant-f", req); err != nil {
		t.Fatalf("Get with nil Metrics: %v", err)
	}
}
