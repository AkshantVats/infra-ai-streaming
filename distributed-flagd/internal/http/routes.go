// SPDX-License-Identifier: MIT
package httpapi

import "net/http"

// RegisterRoutes wires all HTTP handlers onto mux.
func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("/flags", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			h.CreateFlag(w, r)
		case http.MethodGet:
			h.ListFlags(w, r)
		default:
			writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/flags/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.GetFlag(w, r)
		case http.MethodPut:
			h.UpdateFlag(w, r)
		case http.MethodDelete:
			h.DeleteFlag(w, r)
		default:
			writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/healthz", h.Healthz)
	mux.HandleFunc("/evaluate", h.Evaluate)
}
