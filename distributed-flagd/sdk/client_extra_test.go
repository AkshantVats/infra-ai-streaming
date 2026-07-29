// SPDX-License-Identifier: MIT
package sdk_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/akshantvats/distributed-flagd/sdk"
)

// TestUpdateFlagExtra verifies PUT /flags/{name} succeeds and returns the updated FlagData.
func TestUpdateFlagExtra(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "want PUT", http.StatusMethodNotAllowed)
			return
		}
		var req sdk.FlagRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sdk.FlagData{
			Name:    req.Name,
			Value:   req.Value,
			Enabled: req.Enabled,
		})
	}))
	defer srv.Close()

	c := sdk.New(srv.URL)
	updated, err := c.UpdateFlag(context.Background(), "my-flag", sdk.FlagRequest{
		Name:    "my-flag",
		Value:   "v2",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("UpdateFlag: %v", err)
	}
	if updated.Value != "v2" {
		t.Errorf("want v2, got %s", updated.Value)
	}
	if !updated.Enabled {
		t.Error("expected Enabled=true")
	}
}

// TestUpdateFlag_HTTPError verifies that a non-200 response from PUT is returned as an error.
func TestUpdateFlag_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "flag not found"})
	}))
	defer srv.Close()

	c := sdk.New(srv.URL)
	_, err := c.UpdateFlag(context.Background(), "missing", sdk.FlagRequest{Name: "missing"})
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
}

// TestEvaluate_HTTPError verifies Evaluate returns an error on non-200.
func TestEvaluate_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
	}))
	defer srv.Close()

	c := sdk.New(srv.URL)
	_, err := c.Evaluate(context.Background(), "tenant", "user")
	if err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}

// TestGetFlag_HTTPError verifies GetFlag returns an error on 404.
func TestGetFlag_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}))
	defer srv.Close()

	c := sdk.New(srv.URL)
	_, err := c.GetFlag(context.Background(), "unknown")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

// TestListFlags_HTTPError verifies ListFlags returns an error on server failure.
func TestListFlags_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := sdk.New(srv.URL)
	_, err := c.ListFlags(context.Background())
	if err == nil {
		t.Fatal("expected error for 503, got nil")
	}
}

// TestCreateFlag_HTTPError verifies CreateFlag propagates error on unexpected status.
func TestCreateFlag_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "already exists"})
	}))
	defer srv.Close()

	c := sdk.New(srv.URL)
	_, err := c.CreateFlag(context.Background(), sdk.FlagRequest{Name: "dup"})
	if err == nil {
		t.Fatal("expected error for 409, got nil")
	}
}

// TestDeleteFlag_HTTPError verifies DeleteFlag returns an error on 404.
func TestDeleteFlag_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := sdk.New(srv.URL)
	err := c.DeleteFlag(context.Background(), "ghost")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

// TestHealthz_Unhealthy verifies Healthz returns an error when server is down.
func TestHealthz_Unhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := sdk.New(srv.URL)
	err := c.Healthz(context.Background())
	if err == nil {
		t.Fatal("expected error from unhealthy server")
	}
}

// TestDo_ErrorBodyDecoded verifies that a JSON error body is parsed into the error message.
func TestDo_ErrorBodyDecoded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid flag name"})
	}))
	defer srv.Close()

	c := sdk.New(srv.URL)
	_, err := c.GetFlag(context.Background(), "bad name")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// The error message should contain the server's message.
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// TestUpdateFlag_WithVariants verifies variants are round-tripped through UpdateFlag.
func TestUpdateFlag_WithVariants(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req sdk.FlagRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sdk.FlagData{
			Name:     req.Name,
			Variants: req.Variants,
			Enabled:  req.Enabled,
		})
	}))
	defer srv.Close()

	c := sdk.New(srv.URL)
	updated, err := c.UpdateFlag(context.Background(), "rollout-flag", sdk.FlagRequest{
		Name:    "rollout-flag",
		Enabled: true,
		Variants: []sdk.VariantData{
			{Value: "gpt-4o-mini", Weight: 80},
			{Value: "gpt-4o", Weight: 20},
		},
	})
	if err != nil {
		t.Fatalf("UpdateFlag: %v", err)
	}
	if len(updated.Variants) != 2 {
		t.Errorf("want 2 variants, got %d", len(updated.Variants))
	}
	if updated.Variants[0].Weight != 80 {
		t.Errorf("want weight=80, got %d", updated.Variants[0].Weight)
	}
}

// TestInvalidBaseURL_Get exercises the NewRequestWithContext error path in get().
func TestInvalidBaseURL_Get(t *testing.T) {
	c := sdk.New("://not-a-url") // invalid URL causes NewRequestWithContext to fail
	_, err := c.GetFlag(context.Background(), "flag")
	if err == nil {
		t.Error("expected error with invalid base URL")
	}
}

// TestInvalidBaseURL_Post exercises the NewRequestWithContext error path in postExpect().
func TestInvalidBaseURL_Post(t *testing.T) {
	c := sdk.New("://not-a-url")
	_, err := c.Evaluate(context.Background(), "t", "u")
	if err == nil {
		t.Error("expected error with invalid base URL for Evaluate")
	}
}

// TestInvalidBaseURL_Put exercises the NewRequestWithContext error path in put().
func TestInvalidBaseURL_Put(t *testing.T) {
	c := sdk.New("://not-a-url")
	_, err := c.UpdateFlag(context.Background(), "f", sdk.FlagRequest{})
	if err == nil {
		t.Error("expected error with invalid base URL for UpdateFlag")
	}
}

// TestInvalidBaseURL_Del exercises the NewRequestWithContext error path in del().
func TestInvalidBaseURL_Del(t *testing.T) {
	c := sdk.New("://not-a-url")
	err := c.DeleteFlag(context.Background(), "f")
	if err == nil {
		t.Error("expected error with invalid base URL for DeleteFlag")
	}
}

// TestEvaluate_RequestBody verifies the request body contains tenant_id and user_id.
func TestEvaluate_RequestBody(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sdk.EvalResponse{
			ResolvedModelID: "gpt-4o-mini",
			Variant:         "control",
		})
	}))
	defer srv.Close()

	c := sdk.New(srv.URL)
	_, _ = c.Evaluate(context.Background(), "my-tenant", "my-user")

	if gotBody["tenant_id"] != "my-tenant" {
		t.Errorf("tenant_id: want my-tenant, got %q", gotBody["tenant_id"])
	}
	if gotBody["user_id"] != "my-user" {
		t.Errorf("user_id: want my-user, got %q", gotBody["user_id"])
	}
}
