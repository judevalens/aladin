// Package httptransport owns the public HTTP adapter for URL unfurling.
package httptransport

import (
	"errors"
	"net/http"

	"aladin/backend_v2/internal/httpapi"
	"aladin/backend_v2/internal/unfurl"
)

// Register installs the existing unfurl route without changing its wire contract.
func Register(mux *http.ServeMux, service unfurl.UnfurlService) {
	// Server-side link preview for board link objects — the browser can't fetch
	// arbitrary origins (CORS), and the SSRF guard must live where it can't be skipped.
	mux.HandleFunc("POST /api/unfurl", func(w http.ResponseWriter, r *http.Request) {
		handleUnfurl(w, r, service)
	})
}

// POST /api/unfurl {url} — resolve an external URL's preview metadata. Auth'd like every
// non-public route; the result is shaped exactly as the board link shape stores it.
func handleUnfurl(w http.ResponseWriter, r *http.Request, service unfurl.UnfurlService) {
	var payload struct {
		URL string `json:"url"`
	}
	if err := httpapi.ReadJSON(r, &payload); err != nil {
		httpapi.WriteDecodeError(w, r, err)
		return
	}
	preview, err := service.Unfurl(r.Context(), payload.URL)
	if err != nil {
		switch {
		case errors.Is(err, unfurl.ErrInvalidUnfurlURL), errors.Is(err, unfurl.ErrUnfurlTargetRefused):
			httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", err.Error(), err)
		default:
			// The upstream site failing is not our failure — 502 keeps client retry
			// logic honest and the shape falls back to a bare domain preview.
			httpapi.WriteError(w, r, http.StatusBadGateway, "service_error", err.Error(), err)
		}
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, preview)
}
