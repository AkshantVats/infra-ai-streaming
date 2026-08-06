// SPDX-License-Identifier: MIT

// Package gateway composes cost-budget-enforcer's three moving pieces into
// RouteIQ's stub gateway: enforcer.Check (budget) runs before CacheClient
// (semantic cache) runs before ModelClient (the model call itself), per
// DESIGN.md §6. It is a stub in the sense DESIGN.md §6 means it — CacheClient
// and ModelClient are interfaces with no production implementation wired in
// yet, the same way Day 60's semantic-cache-engine shipped a DESIGN.md before
// pkg/cachestore had a real Postgres behind it — but the ordering this
// package enforces, and the LensAI events it emits for each outcome, are the
// real, permanent contract downstream callers get to depend on.
package gateway

import (
	"context"
	"fmt"
	"time"

	"github.com/akshantvats/cost-budget-enforcer/pkg/config"
	"github.com/akshantvats/cost-budget-enforcer/pkg/enforcer"
)

// CacheResult is the outcome of a CacheClient.Lookup call.
type CacheResult struct {
	// Hit is true if the semantic cache had a usable response for this
	// prompt.
	Hit bool
	// Response is the cached completion. Only meaningful when Hit is true.
	Response string
	// MatchedPromptHash identifies the cache entry that matched, carried
	// through to the gateway_cache_hit event's trace_id the same way
	// semantic-cache-engine/pkg/lookup.Result.MatchedPromptHash does.
	MatchedPromptHash string
	// Latency is the cache lookup's own duration, reported to LensAI
	// instead of the gateway's total request time so a cache_hit row and a
	// real inference row stay comparable on "how long did the thing that
	// actually happened take."
	Latency time.Duration
}

// CacheClient is the subset of a semantic cache client Gateway depends on.
// This package takes it as an interface rather than importing
// semantic-cache-engine/pkg/lookup directly: cost-budget-enforcer and
// semantic-cache-engine are separate Go modules in this repo, and a stub
// gateway proving out composition order is not yet the place to collapse
// that boundary — DESIGN.md §6 calls that out explicitly as future work.
type CacheClient interface {
	Lookup(ctx context.Context, tenantID, prompt string) (CacheResult, error)
}

// ModelResult is the outcome of a ModelClient.Call call.
type ModelResult struct {
	Response   string
	TokensUsed int64
	// CostUSD is the model call's actual price, as reported by the model
	// client itself — not an estimate. This is the number DESIGN.md §6's
	// wiring puts on LensAI's cost_usd stream.
	CostUSD float64
	Latency time.Duration
}

// ModelClient is the subset of a model-calling client Gateway depends on.
type ModelClient interface {
	Call(ctx context.Context, tenantID, model, prompt string) (ModelResult, error)
}

// EventEmitter is the subset of *lensai.Writer Gateway depends on, so tests
// can inject a fake instead of an httptest.Server — the same pattern
// semantic-cache-engine/pkg/lookup.EventEmitter already uses for its own
// LensAI dependency.
type EventEmitter interface {
	EmitSpend(ctx context.Context, tenantID, modelID string, costUSD float64, latency time.Duration) error
	EmitCacheHit(ctx context.Context, tenantID, modelID, matchedPromptHash string, latency time.Duration) error
	EmitBlocked(ctx context.Context, tenantID string) error
}

// ConfigLookup returns tenantID's budget configuration.
type ConfigLookup func(tenantID string) config.TenantConfig

// TokenEstimator estimates how many tokens a call for prompt will consume,
// the same role pkg/middleware.TokenEstimator plays for the HTTP path.
type TokenEstimator func(tenantID, prompt string) int64

// Gateway is RouteIQ's stub composition of budget enforcement, semantic
// cache, and model call, in that fixed order.
type Gateway struct {
	Enforcer *enforcer.Enforcer
	Config   ConfigLookup
	Tokens   TokenEstimator
	Cache    CacheClient
	Model    ModelClient
	// Events receives one LensAI event per Handle call, when set. A nil
	// Events skips emission entirely — useful for callers without a LensAI
	// endpoint configured yet, the same escape hatch
	// semantic-cache-engine/pkg/lookup.Lookup gives a nil emitter.
	Events EventEmitter
}

// Result is the outcome of a single Handle call.
type Result struct {
	// Blocked is true when enforcer.Check returned Block. Cache and Model
	// were never called.
	Blocked bool
	// RetryAfter is set when Blocked is true.
	RetryAfter time.Duration
	// Degraded is true when enforcer.Check returned Degrade — ModelUsed
	// differs from the model the caller originally requested.
	Degraded bool
	// ModelUsed is the model actually called: the caller's requested
	// model, or cfg's FallbackModel when Degraded.
	ModelUsed string
	// CacheHit is true when the cache served this request instead of the
	// model.
	CacheHit bool
	Response string
	// TokensUsed and CostUSD are zero when Blocked or CacheHit — no model
	// call means no spend to report.
	TokensUsed int64
	CostUSD    float64
	// EmitErr carries a failure to post this outcome's LensAI event.
	// Handle never changes Blocked, CacheHit, Response, or CostUSD because
	// of an emit failure — the event is an observability side channel, the
	// same non-fatal treatment
	// semantic-cache-engine/pkg/lookup.Result.EmitErr gives its own
	// emission failures.
	EmitErr error
}

// Handle runs DESIGN.md §6's fixed order for one request: check the
// tenant's budget first, and only spend anything — a cache lookup or a
// model call — once that check has passed or degraded the request to a
// cheaper model. A tenant over their hard limit never reaches Cache or
// Model at all, which is what keeps a block's cost_usd at exactly zero
// instead of "however much the cache lookup would have cost."
func (g *Gateway) Handle(ctx context.Context, tenantID, model, prompt string) (Result, error) {
	if tenantID == "" {
		return Result{}, fmt.Errorf("gateway: tenant_id is required")
	}
	if prompt == "" {
		return Result{}, fmt.Errorf("gateway: prompt is required")
	}

	cfg := g.Config(tenantID)
	tokens := g.Tokens(tenantID, prompt)

	decision, err := g.Enforcer.Check(ctx, tenantID, tokens, cfg)
	if err != nil {
		// Fail open, the same choice pkg/middleware.Wrap makes when the
		// enforcer can't reach Redis: a broken budget check must not take
		// down the request path it's guarding.
		decision = enforcer.Decision{Action: enforcer.Pass}
	}

	result := Result{ModelUsed: model}

	switch decision.Action {
	case enforcer.Block:
		result.Blocked = true
		result.RetryAfter = decision.RetryAfter
		if g.Events != nil {
			result.EmitErr = g.Events.EmitBlocked(ctx, tenantID)
		}
		return result, nil

	case enforcer.Degrade:
		result.Degraded = true
		result.ModelUsed = decision.FallbackModel
	}

	cacheResult, err := g.Cache.Lookup(ctx, tenantID, prompt)
	if err != nil {
		// A broken cache fails open to the model, the same "don't let an
		// observability/optimization path take down request availability"
		// principle §1's Lua-script discussion and pkg/middleware's Redis
		// handling both apply elsewhere in this module.
		cacheResult = CacheResult{Hit: false}
	}

	if cacheResult.Hit {
		result.CacheHit = true
		result.Response = cacheResult.Response
		if g.Events != nil {
			result.EmitErr = g.Events.EmitCacheHit(ctx, tenantID, result.ModelUsed, cacheResult.MatchedPromptHash, cacheResult.Latency)
		}
		return result, nil
	}

	modelResult, err := g.Model.Call(ctx, tenantID, result.ModelUsed, prompt)
	if err != nil {
		return Result{}, fmt.Errorf("gateway: model call: %w", err)
	}

	result.Response = modelResult.Response
	result.TokensUsed = modelResult.TokensUsed
	result.CostUSD = modelResult.CostUSD
	if g.Events != nil {
		result.EmitErr = g.Events.EmitSpend(ctx, tenantID, result.ModelUsed, modelResult.CostUSD, modelResult.Latency)
	}
	return result, nil
}
