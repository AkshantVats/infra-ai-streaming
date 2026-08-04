// SPDX-License-Identifier: MIT

// Package feedback is the thumbs-down webhook DESIGN.md §4 called "a
// required consumer of the cache_hit event stream" and Day 63's plan
// item names outright: a user who got a wrong cached answer can flag the
// specific hit that produced it, keyed by the same prompt_hash a cache_hit
// event's trace_id already carries. This is the minimal real signal
// available without §4's full sampled human/LLM-judge review pipeline --
// it undercounts (only users who noticed and bothered to flag), which is
// why pkg/analytics treats it as a lower-bound proxy, not the true
// false-positive rate.
package feedback

import (
	"context"
	"encoding/json"
	"net/http"
)

// Emitter is the subset of *lensai.Writer Handler depends on, mirroring
// pkg/lookup.EventEmitter's shape so tests can inject a fake instead of an
// httptest.Server.
type Emitter interface {
	EmitCacheFeedback(ctx context.Context, tenantID, modelID, matchedPromptHash string) error
}

// ModelID identifies the cache lookup path in emitted cache_feedback
// events. It matches pkg/lookup.ModelID's value so a cache_feedback row
// and the cache_hit row it flags share the same model_id -- duplicating
// the string rather than importing pkg/lookup keeps this package's only
// dependency the Emitter interface it already needs, the same
// dependency-minimizing choice pkg/lookup itself makes by not importing
// pkg/lensai directly.
const ModelID = "semantic-cache-lookup"

// request is the thumbs-down webhook's POST body: which tenant, and which
// cache entry (by the prompt_hash a cache_hit event's trace_id already
// carries) the user is flagging as wrong.
type request struct {
	TenantID   string `json:"tenant_id"`
	PromptHash string `json:"prompt_hash"`
}

// Handler serves the thumbs-down webhook.
type Handler struct {
	emitter Emitter
}

// NewHandler creates a Handler that emits flagged feedback through emitter.
func NewHandler(emitter Emitter) *Handler {
	return &Handler{emitter: emitter}
}

// ServeHTTP accepts POST {"tenant_id", "prompt_hash"} and emits a
// cache_feedback event for it. Malformed or incomplete input is a 400 (the
// caller's mistake); a failure to reach LensAI is a 502 (this service's
// dependency failed) -- the same "which side owns the failure" split
// pkg/lookup.Result.EmitErr draws between a bad lookup and a failed
// observability emission, applied here to HTTP status codes since this
// package's only job is the emission itself.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "feedback: method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "feedback: malformed JSON body", http.StatusBadRequest)
		return
	}
	if req.TenantID == "" || req.PromptHash == "" {
		http.Error(w, "feedback: tenant_id and prompt_hash are required", http.StatusBadRequest)
		return
	}

	if err := h.emitter.EmitCacheFeedback(r.Context(), req.TenantID, ModelID, req.PromptHash); err != nil {
		http.Error(w, "feedback: failed to record feedback", http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
