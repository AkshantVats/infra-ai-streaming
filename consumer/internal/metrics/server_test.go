// SPDX-License-Identifier: MIT
package metrics

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status field = %q", body["status"])
	}
	for _, key := range []string{"version", "git_sha", "build_time"} {
		if body[key] == "" {
			t.Fatalf("missing or empty %q", key)
		}
	}
}

// TestStartServer_ErrorPath verifies that StartServer starts a goroutine that
// serves /health without panicking. We find a free port, start the server,
// then verify it responds within a short timeout.
func TestStartServer_ListensAndServes(t *testing.T) {
	// Find a free TCP port.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close() // release so StartServer can bind it

	StartServer(port)

	// Poll until the server is up or timeout.
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:noctx
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return // success
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("StartServer did not respond on port %d within 2s", port)
}
