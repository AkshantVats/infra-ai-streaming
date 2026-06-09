// SPDX-License-Identifier: MIT
package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	httpapi "github.com/akshantvats/distributed-flagd/internal/http"
)

func TestHealthz(t *testing.T) {
	h := httpapi.New(nil, nil, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.Healthz(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz: want 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("healthz: want application/json, got %s", ct)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	h := httpapi.New(nil, nil, nil)
	mux := http.NewServeMux()
	httpapi.RegisterRoutes(mux, h)

	tests := []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodPatch, "/flags", http.StatusMethodNotAllowed},
		{http.MethodDelete, "/flags", http.StatusMethodNotAllowed},
		{http.MethodPatch, "/flags/x", http.StatusMethodNotAllowed},
		{http.MethodPost, "/flags/x", http.StatusMethodNotAllowed},
	}
	for _, tc := range tests {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s %s: want %d, got %d", tc.method, tc.path, tc.want, rec.Code)
		}
	}
}
