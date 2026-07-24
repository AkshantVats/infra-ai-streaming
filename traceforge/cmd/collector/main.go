// SPDX-License-Identifier: MIT
// Package main implements the TraceForge ingest server.
// It accepts POST /v1/spans (JSON array of schema.Span) and forwards the spans
// to a downstream OTel Collector via gRPC OTLP.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/AkshantVats/infra-ai-streaming/traceforge/pkg/export"
	"github.com/AkshantVats/infra-ai-streaming/traceforge/pkg/schema"
)

func main() {
	otlpEndpoint := getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	listenAddr := getenv("LISTEN_ADDR", ":8080")

	ctx := context.Background()
	exporter, err := export.New(ctx, otlpEndpoint)
	if err != nil {
		slog.Error("failed to create OTLP exporter", "endpoint", otlpEndpoint, "err", err)
		os.Exit(1)
	}
	defer exporter.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/spans", makeSpanHandler(ctx, exporter))
	mux.HandleFunc("GET /healthz", healthHandler)

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	slog.Info("traceforge collector listening", "addr", listenAddr, "otlp", otlpEndpoint)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func makeSpanHandler(ctx context.Context, exporter *export.SpanExporter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var spans []schema.Span
		if err := json.NewDecoder(r.Body).Decode(&spans); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		for i, s := range spans {
			if err := s.Validate(); err != nil {
				http.Error(w, fmt.Sprintf("span[%d]: %v", i, err), http.StatusBadRequest)
				return
			}
		}

		if err := exporter.Export(ctx, spans); err != nil {
			slog.Error("OTLP export failed", "err", err)
			http.Error(w, "export error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
