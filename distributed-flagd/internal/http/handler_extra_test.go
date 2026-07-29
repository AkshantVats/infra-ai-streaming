// SPDX-License-Identifier: MIT
package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpapi "github.com/akshantvats/distributed-flagd/internal/http"
)

// helper: build a mux with nil dependencies (tests that hit validation before store calls are safe).
func newMux() *http.ServeMux {
	h := httpapi.New(nil, nil, nil)
	mux := http.NewServeMux()
	httpapi.RegisterRoutes(mux, h)
	return mux
}

// ── POST /evaluate validation ─────────────────────────────────────────────────

func TestEvaluateGetMethodNotAllowed(t *testing.T) {
	mux := newMux()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/evaluate", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /evaluate: want 405, got %d", rec.Code)
	}
}

func TestEvaluateInvalidJSON(t *testing.T) {
	mux := newMux()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/evaluate", strings.NewReader("not-json"))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON: want 400, got %d", rec.Code)
	}
}

func TestEvaluateMissingTenantID(t *testing.T) {
	mux := newMux()
	rec := httptest.NewRecorder()
	body := `{"user_id":"u1"}`
	req := httptest.NewRequest(http.MethodPost, "/evaluate", strings.NewReader(body))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing tenant_id: want 400, got %d", rec.Code)
	}
}

func TestEvaluateMissingUserID(t *testing.T) {
	mux := newMux()
	rec := httptest.NewRecorder()
	body := `{"tenant_id":"t1"}`
	req := httptest.NewRequest(http.MethodPost, "/evaluate", strings.NewReader(body))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing user_id: want 400, got %d", rec.Code)
	}
}

func TestEvaluateBothFieldsMissing(t *testing.T) {
	mux := newMux()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/evaluate", strings.NewReader(`{}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("both fields missing: want 400, got %d", rec.Code)
	}
}

// ── POST /flags validation ────────────────────────────────────────────────────

func TestCreateFlagInvalidJSON(t *testing.T) {
	mux := newMux()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/flags", strings.NewReader("{bad"))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("CreateFlag invalid JSON: want 400, got %d", rec.Code)
	}
}

func TestCreateFlagMissingName(t *testing.T) {
	mux := newMux()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/flags", strings.NewReader(`{"value":"v","changed_by":"ci"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("CreateFlag missing name: want 400, got %d", rec.Code)
	}
}

// ── GET /flags/{name} validation ─────────────────────────────────────────────

func TestGetFlagMissingName(t *testing.T) {
	// /flags/ with no name after the slash hits the /flags/ handler pattern,
	// which calls GetFlag with an empty path segment → 400.
	mux := newMux()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/flags/", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("GetFlag empty name: want 400, got %d", rec.Code)
	}
}

// ── PUT /flags/{name} validation ─────────────────────────────────────────────

func TestUpdateFlagMissingName(t *testing.T) {
	mux := newMux()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/flags/", strings.NewReader(`{}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("UpdateFlag empty name: want 400, got %d", rec.Code)
	}
}

// ── DELETE /flags/{name} validation ──────────────────────────────────────────

func TestDeleteFlagMissingName(t *testing.T) {
	mux := newMux()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/flags/", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("DeleteFlag empty name: want 400, got %d", rec.Code)
	}
}

// ── GET /flags listing ───────────────────────────────────────────────────────

// TestListFlagsMethodNotAllowed verifies that disallowed methods on /flags are rejected.
func TestListFlagsMethodNotAllowed(t *testing.T) {
	mux := newMux()
	for _, method := range []string{http.MethodPut, http.MethodPatch} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/flags", nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /flags: want 405, got %d", method, rec.Code)
		}
	}
}

// ── Content-Type checks ──────────────────────────────────────────────────────

func TestHealthzContentType(t *testing.T) {
	h := httpapi.New(nil, nil, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.Healthz(rec, req)
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Healthz Content-Type = %q, want application/json", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "ok") {
		t.Errorf("Healthz body = %q, want to contain 'ok'", body)
	}
}

func TestEvaluateErrorResponseContentType(t *testing.T) {
	mux := newMux()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/evaluate", strings.NewReader(`{}`))
	mux.ServeHTTP(rec, req)
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("error response Content-Type = %q, want application/json", ct)
	}
}
