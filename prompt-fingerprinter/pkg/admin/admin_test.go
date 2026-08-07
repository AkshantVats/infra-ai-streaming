// SPDX-License-Identifier: MIT

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/akshantvats/prompt-fingerprinter/pkg/fingerprint"
	"github.com/akshantvats/prompt-fingerprinter/pkg/rules"
)

func putRequest(t *testing.T, path, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestServeHTTP_PutSetsRulesForTenant(t *testing.T) {
	store := rules.NewStore()
	h := &Handler{Store: store}

	req := putRequest(t, "/tenants/tenant-a/fingerprint-rules", `{"strip_punctuation":true,"lowercase":true,"max_prompt_bytes":4096}`)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}

	got := store.ForTenant(context.Background(), "tenant-a")
	want := fingerprint.Rules{StripPunctuation: true, Lowercase: true, MaxPromptBytes: 4096}
	if got != want {
		t.Errorf("stored Rules = %+v, want %+v", got, want)
	}

	var resp fingerprint.Rules
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp != want {
		t.Errorf("response body = %+v, want %+v", resp, want)
	}
}

func TestServeHTTP_RejectsNonPUT(t *testing.T) {
	h := &Handler{Store: rules.NewStore()}
	req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-a/fingerprint-rules", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestServeHTTP_RejectsMalformedPath(t *testing.T) {
	h := &Handler{Store: rules.NewStore()}
	for _, path := range []string{
		"/tenants//fingerprint-rules",
		"/tenants/tenant-a/budget",
		"/fingerprint-rules",
		"/tenants/a/b/fingerprint-rules",
	} {
		req := putRequest(t, path, `{}`)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("path %q: status = %d, want 400", path, w.Code)
		}
	}
}

func TestServeHTTP_RejectsInvalidJSON(t *testing.T) {
	h := &Handler{Store: rules.NewStore()}
	req := putRequest(t, "/tenants/tenant-a/fingerprint-rules", `not json`)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestServeHTTP_RejectsInvalidMaxPromptBytes(t *testing.T) {
	h := &Handler{Store: rules.NewStore()}
	req := putRequest(t, "/tenants/tenant-a/fingerprint-rules", `{"max_prompt_bytes":-5}`)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestServeHTTP_SecondPutFullyReplacesFirst(t *testing.T) {
	store := rules.NewStore()
	h := &Handler{Store: store}

	first := putRequest(t, "/tenants/tenant-a/fingerprint-rules", `{"strip_punctuation":true,"max_prompt_bytes":100}`)
	h.ServeHTTP(httptest.NewRecorder(), first)

	second := putRequest(t, "/tenants/tenant-a/fingerprint-rules", `{"lowercase":true}`)
	h.ServeHTTP(httptest.NewRecorder(), second)

	got := store.ForTenant(context.Background(), "tenant-a")
	want := fingerprint.Rules{Lowercase: true}
	if got != want {
		t.Errorf("second PUT should fully replace first: got %+v, want %+v", got, want)
	}
}
