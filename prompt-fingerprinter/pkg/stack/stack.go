// SPDX-License-Identifier: MIT

// Package stack composes DESIGN.md §3's two cache tiers into a single
// lookup: an L1 exact-match check against Redis (this module's own
// pkg/fingerprint), falling through on a miss to an L2 semantic lookup
// (in production, semantic-cache-engine's pkg/lookup.Lookup — this
// module depends on it only through the L2Store interface below, not a
// direct import, since the two remain separate Go modules in this
// repo with no shared go.work; a future gateway-wiring day is where a
// concrete adapter would implement L2Store against the real package).
// A hit at L2 is written back into Redis so the next byte-identical
// repeat of the same prompt resolves at L1 instead of paying the
// semantic path's cost again — the whole reason a two-tier stack pays
// for itself over time rather than just once.
package stack

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/akshantvats/prompt-fingerprinter/pkg/fingerprint"
)

// tracer emits one span per Stack.Get call, tagged with which tier
// resolved it (DESIGN.md §4's "own observability identity" for an
// exact hit versus a semantic hit versus a miss — the same
// distinction Metrics already counts, now visible as a span attribute
// too). Calling otel.Tracer without configuring a TracerProvider is
// safe and cheap: the global default is a no-op, so Get behaves
// identically whether or not a collector is wired up — the same
// "correctness doesn't depend on observability" contract Metrics
// already gives a nil implementation.
var tracer = otel.Tracer("github.com/akshantvats/prompt-fingerprinter/pkg/stack")

// HardTTL is the L1 backfill's expiry. DESIGN.md §3 (Day 70) commits to
// reading semantic-cache-engine's own freshness policy "rather than
// inventing an independent expiry" — that policy's hard ceiling
// (semantic-cache-engine/DESIGN.md §6) is 30 days from write, so this
// constant is that same number, not a new one. semantic-cache-engine's
// decay curve (tightening similarity threshold with age) has no L1
// analog — L1 is an exact-match key, not a similarity search, so there
// is no threshold for a decaying curve to act on. Only the hard
// ceiling applies here.
const HardTTL = 30 * 24 * time.Hour

// Tier identifies which layer resolved a Get call, distinct from the
// bare Hit bool so a caller (and this package's Metrics) can tell an
// l1_hit apart from an l2_hit rather than collapsing both into one
// "cache_hit" number DESIGN.md §4 already warned would hide how much
// traffic is a literal duplicate versus merely similar.
type Tier string

const (
	TierL1   Tier = "l1"
	TierL2   Tier = "l2"
	TierMiss Tier = "miss"
)

// RedisClient is the subset of a Redis client Stack depends on, so
// tests can inject an in-memory fake (see memstore.go) instead of a
// live instance — no Docker daemon in this sandbox, the same
// constraint every prior prompt-fingerprinter/cost-budget-enforcer day
// has logged for their own Redis dependencies.
type RedisClient interface {
	Get(ctx context.Context, key string) (value string, found bool, err error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

// L2Store is the semantic lookup tier. Get returns the cached response
// for tenantID/req if the tenant's similarity threshold is cleared,
// mirroring semantic-cache-engine's pkg/lookup.Result shape (Hit,
// Response) without importing that package directly.
type L2Store interface {
	Get(ctx context.Context, tenantID string, req fingerprint.PromptRequest) (response string, hit bool, err error)
}

// Metrics is Stack's three-way outcome counter. Unlike RedisClient and
// L2Store, a nil Metrics is valid — Get still runs correctly, it just
// reports nothing, the same "observability is optional, correctness
// isn't" contract semantic-cache-engine's pkg/lookup.EventEmitter uses.
type Metrics interface {
	IncL1Hit(ctx context.Context, tenantID string)
	IncL2Hit(ctx context.Context, tenantID string)
	IncMiss(ctx context.Context, tenantID string)
}

// Result is the outcome of a single Stack.Get call.
type Result struct {
	// Hit is true if either tier resolved the request.
	Hit bool
	// Tier is which layer produced the response: TierL1, TierL2, or
	// TierMiss when Hit is false.
	Tier Tier
	// Response is the cached completion to serve back. Only
	// meaningful when Hit is true.
	Response string
}

// Stack composes the two cache tiers behind a single Get call.
type Stack struct {
	Redis   RedisClient
	L2      L2Store
	Metrics Metrics
}

// Get runs DESIGN.md §3's lookup order: fingerprint the request,
// check Redis (L1) first, and only fall through to L2 on an L1 miss —
// or on a Redis error, which fails open to L2 rather than failing the
// request, the same fail-open choice cost-budget-enforcer/pkg/
// middleware makes for its own Redis dependency. An L2 hit is
// backfilled into Redis before returning so the next identical prompt
// becomes an L1 hit.
func (s *Stack) Get(ctx context.Context, tenantID string, req fingerprint.PromptRequest) (Result, error) {
	ctx, span := tracer.Start(ctx, "prompt_fingerprinter.stack.get",
		trace.WithAttributes(attribute.String("tenant.id", tenantID)))
	defer span.End()

	if tenantID == "" {
		err := fmt.Errorf("stack: tenant_id is required")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return Result{}, err
	}

	key := fingerprint.RedisKey(tenantID, fingerprint.Fingerprint(req))

	if value, found, err := s.Redis.Get(ctx, key); err == nil && found {
		span.SetAttributes(attribute.String("cache.tier", string(TierL1)))
		span.SetStatus(codes.Ok, "")
		s.incL1Hit(ctx, tenantID)
		return Result{Hit: true, Tier: TierL1, Response: value}, nil
	}
	// Either an L1 miss, or a Redis error — both fail open to L2. A
	// Redis outage degrades this stack to semantic-cache-engine's own
	// latency, not to a hard failure of the request.

	response, hit, err := s.L2.Get(ctx, tenantID, req)
	if err != nil {
		wrapped := fmt.Errorf("stack: L2 lookup: %w", err)
		span.RecordError(wrapped)
		span.SetStatus(codes.Error, wrapped.Error())
		return Result{}, wrapped
	}
	if !hit {
		span.SetAttributes(attribute.String("cache.tier", string(TierMiss)))
		span.SetStatus(codes.Ok, "")
		s.incMiss(ctx, tenantID)
		return Result{Tier: TierMiss}, nil
	}

	span.SetAttributes(attribute.String("cache.tier", string(TierL2)))
	span.SetStatus(codes.Ok, "")
	s.incL2Hit(ctx, tenantID)
	// Backfill is best-effort: the L2 response is already resolved and
	// correct to serve, so a failed write here only costs the next
	// repeat its L1 speedup, not this request's correctness. HardTTL
	// bounds how long this entry can serve a wrong answer if it was
	// ever written under a colliding key (collision_test.go's drill) —
	// the entry self-expires rather than persisting indefinitely.
	_ = s.Redis.Set(ctx, key, response, HardTTL)
	return Result{Hit: true, Tier: TierL2, Response: response}, nil
}

func (s *Stack) incL1Hit(ctx context.Context, tenantID string) {
	if s.Metrics != nil {
		s.Metrics.IncL1Hit(ctx, tenantID)
	}
}

func (s *Stack) incL2Hit(ctx context.Context, tenantID string) {
	if s.Metrics != nil {
		s.Metrics.IncL2Hit(ctx, tenantID)
	}
}

func (s *Stack) incMiss(ctx context.Context, tenantID string) {
	if s.Metrics != nil {
		s.Metrics.IncMiss(ctx, tenantID)
	}
}
