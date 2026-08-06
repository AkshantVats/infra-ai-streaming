// SPDX-License-Identifier: MIT

// Package middleware wraps an outbound LLM call with enforcer.Enforcer:
// before forwarding a request, it checks the caller's tenant budget and
// either forwards unmodified (pass/alert), rewrites the JSON body's
// "model" field to the tenant's fallback (degrade), or short-circuits
// with 429 + Retry-After (block) — the day's plan item ("Middleware:
// before LLM call, check tenant budget; decrement estimated tokens;
// block or downgrade if exceeded") implemented as standard net/http
// middleware so it composes with ingestion's existing handler chain.
package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/akshantvats/cost-budget-enforcer/pkg/config"
	"github.com/akshantvats/cost-budget-enforcer/pkg/enforcer"
)

// TenantResolver extracts the tenant ID a request should be budgeted
// against.
type TenantResolver func(r *http.Request) string

// TokenEstimator estimates how many tokens the outbound call will
// consume, given the tenant and the request. Implementations
// typically parse the request body's prompt/messages and apply a
// tokenizer or a length heuristic; the estimate need not be exact —
// DESIGN.md's counter is itself an approximation, and the real spend
// (from the provider's usage report) reconciles on a later call the
// same way this module's DESIGN.md describes for tokens_used.
type TokenEstimator func(tenantID string, r *http.Request) int64

// ConfigLookup returns tenantID's budget configuration.
type ConfigLookup func(tenantID string) config.TenantConfig

// Middleware wraps next with a budget check. It never blocks on a slow
// downstream: the enforcer.Enforcer decides synchronously from Redis
// state alone before next is (or isn't) invoked.
type Middleware struct {
	Enforcer *enforcer.Enforcer
	Tenant   TenantResolver
	Tokens   TokenEstimator
	Config   ConfigLookup
}

// Wrap returns an http.Handler that enforces the tenant's budget
// before delegating to next.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := m.Tenant(r)
		if tenantID == "" {
			next.ServeHTTP(w, r)
			return
		}
		cfg := m.Config(tenantID)
		tokens := m.Tokens(tenantID, r)

		d, err := m.Enforcer.Check(r.Context(), tenantID, tokens, cfg)
		if err != nil {
			if cfg.FailClosed {
				// This tenant opted into fail-closed: an unreachable
				// Store means we can no longer prove the request is
				// under budget, so it's rejected rather than forwarded
				// unmetered. See config.TenantConfig.FailClosed.
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"error":"budget store unavailable","policy":"fail_closed"}`))
				return
			}
			// Fail open: an enforcer that can't reach Redis must not
			// take down the request path it's guarding, the same
			// fail-open choice ingestion/src/rate_limit/token_bucket.rs
			// makes when Redis is unavailable.
			next.ServeHTTP(w, r)
			return
		}

		switch d.Action {
		case enforcer.Block:
			w.Header().Set("Retry-After", strconv.Itoa(int(d.RetryAfter.Seconds())))
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"tenant budget exceeded","retry_after_seconds":` +
				strconv.Itoa(int(d.RetryAfter.Seconds())) + `}`))
			return

		case enforcer.Degrade:
			if err := rewriteModel(r, d.FallbackModel); err != nil {
				// Body wasn't valid/rewritable JSON — forward as-is
				// rather than dropping the request for a formatting
				// problem this middleware didn't cause.
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)

		default: // Pass, Alert
			next.ServeHTTP(w, r)
		}
	})
}

// rewriteModel replaces the "model" field in a JSON request body and
// resets Content-Length so downstream readers see a consistent body.
func rewriteModel(r *http.Request, model string) error {
	if r.Body == nil {
		return nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	_ = r.Body.Close()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		r.Body = io.NopCloser(bytes.NewReader(body))
		return err
	}
	payload["model"] = model

	rewritten, err := json.Marshal(payload)
	if err != nil {
		r.Body = io.NopCloser(bytes.NewReader(body))
		return err
	}

	r.Body = io.NopCloser(bytes.NewReader(rewritten))
	r.ContentLength = int64(len(rewritten))
	return nil
}
