// SPDX-License-Identifier: MIT
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/akshantvats/distributed-flagd/internal/audit"
	"github.com/akshantvats/distributed-flagd/internal/etcdstore"
	"github.com/akshantvats/distributed-flagd/internal/eval"
)

// Handler holds dependencies for all HTTP handlers.
type Handler struct {
	store     *etcdstore.Client
	logger    *audit.Logger
	evaluator *eval.ModelEvaluator
}

// New returns a Handler wired to the etcd store, audit logger, and model evaluator.
func New(store *etcdstore.Client, logger *audit.Logger, evaluator *eval.ModelEvaluator) *Handler {
	return &Handler{store: store, logger: logger, evaluator: evaluator}
}

// evalRequest is the JSON body for POST /evaluate.
type evalRequest struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`
}

// evalResponse is the JSON response for POST /evaluate.
type evalResponse struct {
	ResolvedModelID string `json:"resolved_model_id"`
	Variant         string `json:"variant"`
	FlagKey         string `json:"flag_key"`
}

// Evaluate resolves the active model version for a tenant+user pair.
// POST /evaluate — body: {"tenant_id": "...", "user_id": "..."}
// Response: {"resolved_model_id": "...", "variant": "...", "flag_key": "..."}
func (h *Handler) Evaluate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req evalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.TenantID == "" || req.UserID == "" {
		writeError(w, "tenant_id and user_id are required", http.StatusBadRequest)
		return
	}
	result, err := h.evaluator.ResolveModelVersion(r.Context(), req.TenantID, req.UserID)
	if err != nil {
		writeError(w, "evaluation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(evalResponse{
		ResolvedModelID: result.ModelVersion,
		Variant:         result.Variant,
		FlagKey:         result.FlagKey,
	})
}

// flagRequest is the JSON body for POST /flags and PUT /flags/{name}.
type flagRequest struct {
	Name      string                `json:"name"`
	Value     string                `json:"value"`
	Enabled   bool                  `json:"enabled"`
	Variants  []etcdstore.VariantData `json:"variants,omitempty"`
	ChangedBy string                `json:"changed_by"`
	Reason    string                `json:"reason"`
}

func (h *Handler) CreateFlag(w http.ResponseWriter, r *http.Request) {
	var req flagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateRequest(req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	fd := &etcdstore.FlagData{Name: req.Name, Value: req.Value, Enabled: req.Enabled, Variants: req.Variants}
	if err := h.store.SetFlag(r.Context(), fd); err != nil {
		writeError(w, "etcd write failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	_ = h.logger.Log(r.Context(), audit.Entry{
		FlagName:  req.Name,
		OldValue:  "",
		NewValue:  req.Value,
		ChangedBy: req.ChangedBy,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(fd)
}

func (h *Handler) GetFlag(w http.ResponseWriter, r *http.Request) {
	name := nameFromPath(r.URL.Path)
	if name == "" {
		writeError(w, "flag name required", http.StatusBadRequest)
		return
	}
	fd, err := h.store.GetFlag(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, "flag not found", http.StatusNotFound)
		} else {
			writeError(w, "etcd read failed: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(fd)
}

func (h *Handler) ListFlags(w http.ResponseWriter, r *http.Request) {
	flags, err := h.store.ListFlags(r.Context())
	if err != nil {
		writeError(w, "etcd list failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"flags": flags,
		"count": len(flags),
	})
}

func (h *Handler) UpdateFlag(w http.ResponseWriter, r *http.Request) {
	name := nameFromPath(r.URL.Path)
	if name == "" {
		writeError(w, "flag name required", http.StatusBadRequest)
		return
	}
	old, err := h.store.GetFlag(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, "flag not found", http.StatusNotFound)
		} else {
			writeError(w, "etcd read failed: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	var req flagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	req.Name = name
	if err := validateRequest(req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	fd := &etcdstore.FlagData{Name: name, Value: req.Value, Enabled: req.Enabled, Variants: req.Variants}
	if err := h.store.SetFlag(r.Context(), fd); err != nil {
		writeError(w, "etcd write failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	_ = h.logger.Log(r.Context(), audit.Entry{
		FlagName:  name,
		OldValue:  old.Value,
		NewValue:  req.Value,
		ChangedBy: req.ChangedBy,
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(fd)
}

func (h *Handler) DeleteFlag(w http.ResponseWriter, r *http.Request) {
	name := nameFromPath(r.URL.Path)
	if name == "" {
		writeError(w, "flag name required", http.StatusBadRequest)
		return
	}
	old, err := h.store.GetFlag(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, "flag not found", http.StatusNotFound)
		} else {
			writeError(w, "etcd read failed: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if err := h.store.DeleteFlag(r.Context(), name); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, "flag not found", http.StatusNotFound)
		} else {
			writeError(w, "etcd delete failed: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	_ = h.logger.Log(r.Context(), audit.Entry{
		FlagName:  name,
		OldValue:  old.Value,
		NewValue:  "",
		ChangedBy: "api",
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func nameFromPath(path string) string {
	// /flags/{name} → name
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[1] != "" {
		return parts[1]
	}
	return ""
}

func validateRequest(req flagRequest) error {
	if req.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": msg,
		"code":  fmt.Sprintf("%d", code),
	})
}
