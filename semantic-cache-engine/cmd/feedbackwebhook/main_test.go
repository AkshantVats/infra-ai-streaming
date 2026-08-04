// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunRequiresLensAIIngestURL(t *testing.T) {
	t.Setenv("LENSAI_INGEST_URL", "")

	var stdout, stderr strings.Builder
	code := run(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "LENSAI_INGEST_URL") {
		t.Errorf("stderr = %q, want mention of LENSAI_INGEST_URL", stderr.String())
	}
}

type fakeEmitter struct{}

func (fakeEmitter) EmitCacheFeedback(ctx context.Context, tenantID, modelID, matchedPromptHash string) error {
	return nil
}

func TestNewMuxRoutesThumbsdownEndpoint(t *testing.T) {
	mux := newMux(fakeEmitter{})

	req := httptest.NewRequest(http.MethodPost, "/feedback/thumbsdown", strings.NewReader(`{"tenant_id":"t1","prompt_hash":"h1"}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusAccepted)
	}
}

func TestNewMux404sUnknownRoute(t *testing.T) {
	mux := newMux(fakeEmitter{})

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
