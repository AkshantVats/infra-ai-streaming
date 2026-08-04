// SPDX-License-Identifier: MIT

package feedback

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeEmitter struct {
	calledTenant, calledModel, calledHash string
	err                                   error
}

func (f *fakeEmitter) EmitCacheFeedback(ctx context.Context, tenantID, modelID, matchedPromptHash string) error {
	f.calledTenant, f.calledModel, f.calledHash = tenantID, modelID, matchedPromptHash
	return f.err
}

func TestServeHTTPAcceptsValidFeedback(t *testing.T) {
	emitter := &fakeEmitter{}
	h := NewHandler(emitter)

	req := httptest.NewRequest(http.MethodPost, "/feedback/thumbsdown", strings.NewReader(`{"tenant_id":"tenant-a","prompt_hash":"abc123"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusAccepted)
	}
	if emitter.calledTenant != "tenant-a" || emitter.calledHash != "abc123" {
		t.Errorf("emitter called with tenant=%q hash=%q, want tenant-a/abc123", emitter.calledTenant, emitter.calledHash)
	}
	if emitter.calledModel != ModelID {
		t.Errorf("emitter called with model=%q, want %q", emitter.calledModel, ModelID)
	}
}

func TestServeHTTPRejectsMalformedJSON(t *testing.T) {
	h := NewHandler(&fakeEmitter{})

	req := httptest.NewRequest(http.MethodPost, "/feedback/thumbsdown", strings.NewReader(`not json`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestServeHTTPRejectsMissingFields(t *testing.T) {
	h := NewHandler(&fakeEmitter{})

	for _, body := range []string{
		`{"prompt_hash":"abc123"}`,
		`{"tenant_id":"tenant-a"}`,
		`{}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/feedback/thumbsdown", strings.NewReader(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("body %q: status = %d, want %d", body, w.Code, http.StatusBadRequest)
		}
	}
}

func TestServeHTTPRejectsNonPost(t *testing.T) {
	h := NewHandler(&fakeEmitter{})

	req := httptest.NewRequest(http.MethodGet, "/feedback/thumbsdown", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestServeHTTPMapsEmitterFailureTo502(t *testing.T) {
	h := NewHandler(&fakeEmitter{err: errors.New("lensai unreachable")})

	req := httptest.NewRequest(http.MethodPost, "/feedback/thumbsdown", strings.NewReader(`{"tenant_id":"tenant-a","prompt_hash":"abc123"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadGateway)
	}
}
