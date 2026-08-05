// SPDX-License-Identifier: MIT

// Package admin implements the live tenant-budget Admin API: a single
// endpoint, PATCH /tenants/{id}/budget, that lets an operator change a
// tenant's budget_tokens, window, fallback model, webhook URL, or
// thresholds without a config-file edit and a process restart. Every
// applied change is published to pkg/audit before the response is
// sent, and — unlike enforcer's Redis fail-open — a change whose audit
// record can't be published is rolled back rather than allowed to
// stand silently unaudited.
package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/akshantvats/cost-budget-enforcer/pkg/audit"
	"github.com/akshantvats/cost-budget-enforcer/pkg/config"
)

// Handler serves PATCH /tenants/{id}/budget.
type Handler struct {
	Store *config.LiveStore
	Audit audit.Publisher
	// Now is the injectable clock; defaults to time.Now if left nil.
	Now func() time.Time
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeError(w, http.StatusMethodNotAllowed, "only PATCH is supported")
		return
	}

	tenantID, ok := parseTenantID(r.URL.Path)
	if !ok {
		writeError(w, http.StatusBadRequest, "path must be /tenants/{id}/budget")
		return
	}

	var patch config.TenantConfigPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	before, after, err := h.Store.Patch(tenantID, patch)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	actor := r.Header.Get("X-Admin-Actor")
	if actor == "" {
		actor = "unknown"
	}
	event := audit.BudgetChangeEvent{
		TenantID:  tenantID,
		Actor:     actor,
		Before:    toMap(before),
		After:     toMap(after),
		Timestamp: h.now().Unix(),
	}

	if err := h.Audit.Publish(r.Context(), event); err != nil {
		// Fail closed: an admin change that can't be durably audited
		// must not take effect. Roll the in-memory store back to the
		// pre-patch value so a retried Publish and a retried PATCH
		// converge to the same state rather than compounding.
		h.Store.Set(tenantID, before)
		writeError(w, http.StatusServiceUnavailable, "audit log unavailable, change rejected: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(after)
}

// parseTenantID extracts {id} from a path of the exact shape
// /tenants/{id}/budget. It rejects an empty id and an id containing a
// slash, since either would mean the path doesn't actually name a
// single tenant.
func parseTenantID(path string) (string, bool) {
	const prefix = "/tenants/"
	const suffix = "/budget"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if id == "" || strings.Contains(id, "/") {
		return "", false
	}
	return id, true
}

// toMap converts a TenantConfig to the map[string]any shape
// audit.BudgetChangeEvent stores, via its JSON tags, so the audit
// record's field names match the Admin API's request/response body
// rather than needing a second, hand-maintained mapping.
func toMap(tc config.TenantConfig) map[string]any {
	data, err := json.Marshal(tc)
	if err != nil {
		// TenantConfig is a plain struct of JSON-marshalable scalar
		// fields; Marshal cannot fail for it.
		panic(fmt.Sprintf("admin: marshal TenantConfig: %v", err))
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		panic(fmt.Sprintf("admin: unmarshal TenantConfig into map: %v", err))
	}
	return m
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
