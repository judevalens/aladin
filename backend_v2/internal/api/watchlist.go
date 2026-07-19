package api

import (
	"errors"
	"net/http"

	coreservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerWatchlistRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/watchlist", s.handleWatchlistList)
	mux.HandleFunc("POST /api/watchlist", s.handleWatchlistAdd)
	mux.HandleFunc("DELETE /api/watchlist/{instrumentId}", s.handleWatchlistRemove)
}

// GET /api/watchlist — the tickers the signed-in user is tracking.
func (s *Server) handleWatchlistList(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Watchlist().List(r.Context(), principalUserID(r))
	if err != nil {
		s.writeWatchlistError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// POST /api/watchlist {instrumentId} — track a ticker.
func (s *Server) handleWatchlistAdd(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		InstrumentID string `json:"instrumentId"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	if err := s.deps.Watchlist().Add(r.Context(), principalUserID(r), payload.InstrumentID); err != nil {
		s.writeWatchlistError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

// DELETE /api/watchlist/{instrumentId} — stop tracking a ticker.
func (s *Server) handleWatchlistRemove(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Watchlist().Remove(r.Context(), principalUserID(r), r.PathValue("instrumentId")); err != nil {
		s.writeWatchlistError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) writeWatchlistError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, coreservice.ErrInvalidWatchlistInput) {
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
		return
	}
	writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
}
