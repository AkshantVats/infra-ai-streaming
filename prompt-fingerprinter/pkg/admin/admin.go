// SPDX-License-Identifier: MIT

// Package admin implements prompt-fingerprinter's Admin API: a single
// endpoint, PUT /tenants/{id}/fingerprint-rules, that lets an operator
// configure a tenant's normalization overrides (fingerprint.Rules)
// without a code change or process restart. Shape follows
// cost-budget-enforcer/pkg/admin's parseTenantID/writeError convention,
// but PUT rather than PATCH: a Rules value is three small fields, so
// "send the whole resource" is the simpler contract and doesn't need a
// second pointer-field patch type only for this endpoint.
package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/akshantvats/prompt-fingerprinter/pkg/fingerprint"
	"github.com/akshantvats/prompt-fingerprinter/pkg/rules"
)

// Handler serves PUT /tenants/{id}/fingerprint-rules.
type Handler struct {
	Store *rules.Store
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "only PUT is supported")
		return
	}

	tenantID, ok := parseTenantID(r.URL.Path)
	if !ok {
		writeError(w, http.StatusBadRequest, "path must be /tenants/{id}/fingerprint-rules")
		return
	}

	var body fingerprint.Rules
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	if err := h.Store.Put(tenantID, body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(body)
}

// parseTenantID extracts {id} from a path of the exact shape
// /tenants/{id}/fingerprint-rules. It rejects an empty id and an id
// containing a slash, since either would mean the path doesn't actually
// name a single tenant.
func parseTenantID(path string) (string, bool) {
	const prefix = "/tenants/"
	const suffix = "/fingerprint-rules"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if id == "" || strings.Contains(id, "/") {
		return "", false
	}
	return id, true
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
