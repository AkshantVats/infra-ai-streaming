package metrics

import (
	"fmt"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// StartServer exposes /health and /metrics on port.
func StartServer(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/metrics", promhttp.Handler())
	addr := fmt.Sprintf(":%d", port)
	go func() {
		log.Printf("level=info msg=metrics_server_started addr=%s", addr)
		if err := http.ListenAndServe(addr, mux); err != nil && err != http.ErrServerClosed {
			log.Printf("level=error msg=metrics_server_failed err=%v", err)
		}
	}()
}
