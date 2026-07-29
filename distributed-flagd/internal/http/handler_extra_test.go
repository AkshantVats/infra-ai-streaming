// SPDX-License-Identifier: MIT
package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/akshantvats/distributed-flagd/internal/audit"
	"github.com/akshantvats/distributed-flagd/internal/etcdstore"
	"github.com/akshantvats/distributed-flagd/internal/eval"
	httpapi "github.com/akshantvats/distributed-flagd/internal/http"
)

// ── fake dependencies ─────────────────────────────────────────────────────────

type fakeStore struct {
	flags   map[string]*etcdstore.FlagData
	setErr  error
	listErr error
	delErr  error
}

func newFakeStore() *fakeStore {
	return &fakeStore{flags: make(map[string]*etcdstore.FlagData)}
}

func (f *fakeStore) GetFlag(_ context.Context, name string) (*etcdstore.FlagData, error) {
	fd, ok := f.flags[name]
	if !ok {
		return nil, errors.New("flag not found: " + name)
	}
	return fd, nil
}

func (f *fakeStore) SetFlag(_ context.Context, fd *etcdstore.FlagData) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.flags[fd.Name] = fd
	return nil
}

func (f *fakeStore) ListFlags(_ context.Context) ([]*etcdstore.FlagData, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]*etcdstore.FlagData, 0, len(f.flags))
	for _, fd := range f.flags {
		out = append(out, fd)
	}
	return out, nil
}

func (f *fakeStore) DeleteFlag(_ context.Context, name string) error {
	if f.delErr != nil {
		return f.delErr
	}
	if _, ok := f.flags[name]; !ok {
		return errors.New("flag not found: " + name)
	}
	delete(f.flags, name)
	return nil
}

type fakeLogger struct{ err error }

func (l *fakeLogger) Log(_ context.Context, _ audit.Entry) error { return l.err }

type fakeEvaluator struct {
	result eval.EvalResult
	err    error
}

func (e *fakeEvaluator) ResolveModelVersion(_ context.Context, _, _ string) (eval.EvalResult, error) {
	return e.result, e.err
}

type fakeStoreIface interface {
	GetFlag(ctx context.Context, name string) (*etcdstore.FlagData, error)
	SetFlag(ctx context.Context, fd *etcdstore.FlagData) error
	ListFlags(ctx context.Context) ([]*etcdstore.FlagData, error)
	DeleteFlag(ctx context.Context, name string) error
}

func newMuxWith(store fakeStoreIface, logger *fakeLogger, evaluator *fakeEvaluator) *http.ServeMux {
	h := httpapi.NewWithDeps(store, logger, evaluator)
	mux := http.NewServeMux()
	httpapi.RegisterRoutes(mux, h)
	return mux
}

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

// ── Evaluate success + evaluator error ───────────────────────────────────────

func TestEvaluateSuccess(t *testing.T) {
	store := newFakeStore()
	logger := &fakeLogger{}
	evaluator := &fakeEvaluator{result: eval.EvalResult{
		ModelVersion: "gpt-4o-mini",
		Variant:      "control",
		FlagKey:      "model-rollout:acme",
	}}
	mux := newMuxWith(store, logger, evaluator)

	body := `{"tenant_id":"acme","user_id":"user-1"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/evaluate", strings.NewReader(body))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Evaluate success: want 200, got %d", rec.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["resolved_model_id"] != "gpt-4o-mini" {
		t.Errorf("want gpt-4o-mini, got %s", resp["resolved_model_id"])
	}
}

func TestEvaluateEvaluatorError(t *testing.T) {
	store := newFakeStore()
	logger := &fakeLogger{}
	evaluator := &fakeEvaluator{err: errors.New("etcd timeout")}
	mux := newMuxWith(store, logger, evaluator)

	body := `{"tenant_id":"acme","user_id":"user-1"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/evaluate", strings.NewReader(body))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("evaluator error: want 500, got %d", rec.Code)
	}
}

// ── ListFlags ─────────────────────────────────────────────────────────────────

func TestListFlagsSuccess(t *testing.T) {
	store := newFakeStore()
	store.flags["flag-a"] = &etcdstore.FlagData{Name: "flag-a", Value: "v1", Enabled: true}
	store.flags["flag-b"] = &etcdstore.FlagData{Name: "flag-b", Value: "v2", Enabled: false}
	mux := newMuxWith(store, &fakeLogger{}, &fakeEvaluator{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/flags", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ListFlags: want 200, got %d", rec.Code)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if count, ok := body["count"].(float64); !ok || int(count) != 2 {
		t.Errorf("count: want 2, got %v", body["count"])
	}
}

func TestListFlagsStoreError(t *testing.T) {
	store := newFakeStore()
	store.listErr = errors.New("etcd unavailable")
	mux := newMuxWith(store, &fakeLogger{}, &fakeEvaluator{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/flags", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ListFlags store error: want 500, got %d", rec.Code)
	}
}

// ── GetFlag ───────────────────────────────────────────────────────────────────

func TestGetFlagSuccess(t *testing.T) {
	store := newFakeStore()
	store.flags["my-flag"] = &etcdstore.FlagData{Name: "my-flag", Value: "v1", Enabled: true}
	mux := newMuxWith(store, &fakeLogger{}, &fakeEvaluator{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/flags/my-flag", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GetFlag success: want 200, got %d", rec.Code)
	}
	var fd etcdstore.FlagData
	if err := json.NewDecoder(rec.Body).Decode(&fd); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if fd.Value != "v1" {
		t.Errorf("want v1, got %s", fd.Value)
	}
}

func TestGetFlagNotFound(t *testing.T) {
	store := newFakeStore()
	mux := newMuxWith(store, &fakeLogger{}, &fakeEvaluator{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/flags/missing", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("GetFlag not found: want 404, got %d", rec.Code)
	}
}

func TestGetFlagStoreError(t *testing.T) {
	// Use a store where GetFlag returns a non-not-found error.
	store := &errorStore{}
	mux := newMuxWith(store, &fakeLogger{}, &fakeEvaluator{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/flags/any", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("GetFlag store error: want 500, got %d", rec.Code)
	}
}

// ── CreateFlag ────────────────────────────────────────────────────────────────

func TestCreateFlagSuccess(t *testing.T) {
	store := newFakeStore()
	mux := newMuxWith(store, &fakeLogger{}, &fakeEvaluator{})

	body := `{"name":"new-flag","value":"v1","enabled":true,"changed_by":"ci"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/flags", strings.NewReader(body))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("CreateFlag: want 201, got %d", rec.Code)
	}
	var fd etcdstore.FlagData
	if err := json.NewDecoder(rec.Body).Decode(&fd); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if fd.Name != "new-flag" {
		t.Errorf("want new-flag, got %s", fd.Name)
	}
}

func TestCreateFlagStoreError(t *testing.T) {
	store := newFakeStore()
	store.setErr = errors.New("disk full")
	mux := newMuxWith(store, &fakeLogger{}, &fakeEvaluator{})

	body := `{"name":"new-flag","value":"v1","enabled":true}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/flags", strings.NewReader(body))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("CreateFlag store error: want 500, got %d", rec.Code)
	}
}

// ── UpdateFlag ────────────────────────────────────────────────────────────────

func TestUpdateFlagSuccess(t *testing.T) {
	store := newFakeStore()
	store.flags["my-flag"] = &etcdstore.FlagData{Name: "my-flag", Value: "v1", Enabled: true}
	mux := newMuxWith(store, &fakeLogger{}, &fakeEvaluator{})

	body := `{"value":"v2","enabled":false,"changed_by":"ci"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/flags/my-flag", strings.NewReader(body))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("UpdateFlag success: want 200, got %d", rec.Code)
	}
	var fd etcdstore.FlagData
	if err := json.NewDecoder(rec.Body).Decode(&fd); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if fd.Value != "v2" {
		t.Errorf("want v2, got %s", fd.Value)
	}
}

func TestUpdateFlagNotFound(t *testing.T) {
	store := newFakeStore() // empty store
	mux := newMuxWith(store, &fakeLogger{}, &fakeEvaluator{})

	body := `{"value":"v2"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/flags/missing", strings.NewReader(body))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("UpdateFlag not found: want 404, got %d", rec.Code)
	}
}

func TestUpdateFlagGetStoreError(t *testing.T) {
	store := &errorStore{}
	mux := newMuxWith(store, &fakeLogger{}, &fakeEvaluator{})

	body := `{"value":"v2"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/flags/any", strings.NewReader(body))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("UpdateFlag get error: want 500, got %d", rec.Code)
	}
}

func TestUpdateFlagInvalidJSON(t *testing.T) {
	store := newFakeStore()
	store.flags["my-flag"] = &etcdstore.FlagData{Name: "my-flag", Value: "v1"}
	mux := newMuxWith(store, &fakeLogger{}, &fakeEvaluator{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/flags/my-flag", strings.NewReader("{bad"))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("UpdateFlag invalid JSON: want 400, got %d", rec.Code)
	}
}

func TestUpdateFlagSetStoreError(t *testing.T) {
	store := newFakeStore()
	store.flags["my-flag"] = &etcdstore.FlagData{Name: "my-flag", Value: "v1"}
	store.setErr = errors.New("write failed")
	mux := newMuxWith(store, &fakeLogger{}, &fakeEvaluator{})

	body := `{"value":"v2"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/flags/my-flag", strings.NewReader(body))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("UpdateFlag set error: want 500, got %d", rec.Code)
	}
}

// ── DeleteFlag ────────────────────────────────────────────────────────────────

func TestDeleteFlagSuccess(t *testing.T) {
	store := newFakeStore()
	store.flags["my-flag"] = &etcdstore.FlagData{Name: "my-flag", Value: "v1"}
	mux := newMuxWith(store, &fakeLogger{}, &fakeEvaluator{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/flags/my-flag", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("DeleteFlag success: want 204, got %d", rec.Code)
	}
}

func TestDeleteFlagNotFound(t *testing.T) {
	store := newFakeStore() // empty
	mux := newMuxWith(store, &fakeLogger{}, &fakeEvaluator{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/flags/missing", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("DeleteFlag not found (get): want 404, got %d", rec.Code)
	}
}

func TestDeleteFlagGetStoreError(t *testing.T) {
	store := &errorStore{}
	mux := newMuxWith(store, &fakeLogger{}, &fakeEvaluator{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/flags/any", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("DeleteFlag get error: want 500, got %d", rec.Code)
	}
}

func TestDeleteFlagDeleteStoreError(t *testing.T) {
	store := newFakeStore()
	store.flags["my-flag"] = &etcdstore.FlagData{Name: "my-flag", Value: "v1"}
	store.delErr = errors.New("delete failed")
	mux := newMuxWith(store, &fakeLogger{}, &fakeEvaluator{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/flags/my-flag", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("DeleteFlag del error: want 500, got %d", rec.Code)
	}
}

// ── errorStore: GetFlag always returns a non-not-found error ─────────────────

type errorStore struct{}

func (e *errorStore) GetFlag(_ context.Context, _ string) (*etcdstore.FlagData, error) {
	return nil, errors.New("etcd connection refused")
}
func (e *errorStore) SetFlag(_ context.Context, _ *etcdstore.FlagData) error {
	return errors.New("etcd connection refused")
}
func (e *errorStore) ListFlags(_ context.Context) ([]*etcdstore.FlagData, error) {
	return nil, errors.New("etcd connection refused")
}
func (e *errorStore) DeleteFlag(_ context.Context, _ string) error {
	return errors.New("etcd connection refused")
}
