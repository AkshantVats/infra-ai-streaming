// SPDX-License-Identifier: MIT

// Command feedbackwebhook is the CLI/server entry point for semantic-cache-engine's
// Day 63 thumbs-down webhook (pkg/feedback): it listens for POST
// {"tenant_id","prompt_hash"} and dual-writes a cache_feedback event to
// LensAI (pkg/lensai) for each one. This is the minimal real consumer
// DESIGN.md §4 called for -- a user can flag a specific cache hit as
// wrong -- feeding pkg/analytics's false-positive-rate proxy panel.
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/akshantvats/semantic-cache-engine/pkg/feedback"
	"github.com/akshantvats/semantic-cache-engine/pkg/lensai"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// defaultAddr is used when the PORT environment variable is unset, kept as
// a documented default rather than a required flag since a webhook
// listener's port is usually fixed by its deployment, not chosen per
// invocation.
const defaultAddr = ":8090"

// run implements the CLI and returns the process exit code, kept separate
// from main for testability without exec'ing a built binary or binding a
// real socket in the fail-fast paths -- the same shape as
// cmd/cachelookup/main.go::run and cmd/embedworker/main.go::run.
func run(args []string, stdout, stderr io.Writer) int {
	ingestURL := os.Getenv("LENSAI_INGEST_URL")
	if ingestURL == "" {
		_, _ = fmt.Fprintln(stderr, "feedbackwebhook: LENSAI_INGEST_URL is required")
		return 2
	}

	addr := os.Getenv("PORT")
	if addr == "" {
		addr = defaultAddr
	} else {
		addr = ":" + addr
	}

	mux := newMux(lensai.New(ingestURL))

	_, _ = fmt.Fprintf(stdout, "feedbackwebhook: listening on %s, forwarding to %s\n", addr, ingestURL)
	if err := http.ListenAndServe(addr, mux); err != nil {
		_, _ = fmt.Fprintf(stderr, "feedbackwebhook: %v\n", err)
		return 1
	}
	return 0
}

// newMux wires pkg/feedback.Handler to the webhook's one route, split out
// from run so tests can exercise routing without binding a real socket.
func newMux(emitter feedback.Emitter) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/feedback/thumbsdown", feedback.NewHandler(emitter))
	return mux
}
