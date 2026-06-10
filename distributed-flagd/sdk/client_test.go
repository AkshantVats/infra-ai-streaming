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

func TestEvaluate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/evaluate" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sdk.EvalResponse{
			ResolvedModelID: "gpt-4o-mini",
			Variant:         "control",
			FlagKey:         "model-version",
		})
	}))
	defer srv.Close()

	c := sdk.New(srv.URL)
	resp, err := c.Evaluate(context.Background(), "acme", "user-1")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if resp.ResolvedModelID != "gpt-4o-mini" {
		t.Errorf("want gpt-4o-mini, got %s", resp.ResolvedModelID)
	}
	if resp.Variant != "control" {
		t.Errorf("want control, got %s", resp.Variant)
	}
}

func TestCreateGetDeleteFlag(t *testing.T) {
	flags := map[string]*sdk.FlagData{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/flags":
			var req sdk.FlagRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			fd := &sdk.FlagData{Name: req.Name, Value: req.Value, Enabled: req.Enabled}
			flags[req.Name] = fd
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(fd)
		case r.Method == http.MethodGet && len(r.URL.Path) > 7:
			name := r.URL.Path[7:]
			fd, ok := flags[name]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(fd)
		case r.Method == http.MethodDelete && len(r.URL.Path) > 7:
			name := r.URL.Path[7:]
			delete(flags, name)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := sdk.New(srv.URL)
	ctx := context.Background()

	created, err := c.CreateFlag(ctx, sdk.FlagRequest{
		Name: "test-flag", Value: "v1", Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateFlag: %v", err)
	}
	if created.Name != "test-flag" {
		t.Errorf("want test-flag, got %s", created.Name)
	}

	got, err := c.GetFlag(ctx, "test-flag")
	if err != nil {
		t.Fatalf("GetFlag: %v", err)
	}
	if got.Value != "v1" {
		t.Errorf("want v1, got %s", got.Value)
	}

	if err := c.DeleteFlag(ctx, "test-flag"); err != nil {
		t.Fatalf("DeleteFlag: %v", err)
	}
}

func TestListFlags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/flags" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sdk.FlagList{
			Flags: []*sdk.FlagData{
				{Name: "model-version", Value: "gpt-4o-mini", Enabled: true},
			},
			Count: 1,
		})
	}))
	defer srv.Close()

	c := sdk.New(srv.URL)
	list, err := c.ListFlags(context.Background())
	if err != nil {
		t.Fatalf("ListFlags: %v", err)
	}
	if list.Count != 1 {
		t.Errorf("want 1 flag, got %d", list.Count)
	}
	if list.Flags[0].Name != "model-version" {
		t.Errorf("want model-version, got %s", list.Flags[0].Name)
	}
}

func TestHealthz(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := sdk.New(srv.URL)
	if err := c.Healthz(context.Background()); err != nil {
		t.Fatalf("Healthz: %v", err)
	}
}
