package metrics

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/buildinfo"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// StartServer exposes /health and /metrics on port.
func StartServer(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.Handle("/metrics", promhttp.Handler())
	addr := fmt.Sprintf(":%d", port)
	go func() {
		log.Printf("level=info msg=metrics_server_started addr=%s", addr)
		if err := http.ListenAndServe(addr, mux); err != nil && err != http.ErrServerClosed {
			log.Printf("level=error msg=metrics_server_failed err=%v", err)
		}
	}()
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":      "ok",
		"version":     buildinfo.Version,
		"git_sha":     buildinfo.GitSHA,
		"build_time":  buildinfo.BuildTime,
	})
}
