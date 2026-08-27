package api

import (
	"errors"
	"net/http"

	coreservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerUnfurlRoutes(mux *http.ServeMux) {
	// Server-side link preview for board link objects — the browser can't fetch
	// arbitrary origins (CORS), and the SSRF guard must live where it can't be skipped.
	mux.HandleFunc("POST /api/unfurl", s.handleUnfurl)
}

// POST /api/unfurl {url} — resolve an external URL's preview metadata. Auth'd like every
// non-public route; the result is shaped exactly as the board link shape stores it.
func (s *Server) handleUnfurl(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		URL string `json:"url"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	preview, err := s.deps.Unfurl().Unfurl(r.Context(), payload.URL)
	if err != nil {
		switch {
		case errors.Is(err, coreservice.ErrInvalidUnfurlURL), errors.Is(err, coreservice.ErrUnfurlTargetRefused):
			writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
		default:
			// The upstream site failing is not our failure — 502 keeps client retry
			// logic honest and the shape falls back to a bare domain preview.
			writeAPIError(w, r, http.StatusBadGateway, categoryServiceError, err.Error(), err)
		}
		return
	}
	writeJSON(w, http.StatusOK, preview)
}
