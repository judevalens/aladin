package api

import (
	"errors"
	"net/http"

	coreservice "aladin/backend_v2/internal/service"
)

func (s *Server) registerWatchlistRoutes(mux *http.ServeMux) {
	// Lists (universes).
	mux.HandleFunc("GET /api/watchlists", s.handleWatchlistsList)
	mux.HandleFunc("POST /api/watchlists", s.handleWatchlistsCreate)
	mux.HandleFunc("PATCH /api/watchlists/{id}", s.handleWatchlistRename)
	mux.HandleFunc("DELETE /api/watchlists/{id}", s.handleWatchlistDelete)
	// Members of a list.
	mux.HandleFunc("GET /api/watchlists/{id}/items", s.handleWatchlistItems)
	mux.HandleFunc("POST /api/watchlists/{id}/items", s.handleWatchlistItemAdd)
	mux.HandleFunc("DELETE /api/watchlists/{id}/items/{instrumentId}", s.handleWatchlistItemRemove)
}

// GET /api/watchlists — the signed-in user's watchlists (with item counts).
func (s *Server) handleWatchlistsList(w http.ResponseWriter, r *http.Request) {
	lists, err := s.deps.Watchlist().ListWatchlists(r.Context(), principalUserID(r))
	if err != nil {
		s.writeWatchlistError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"watchlists": lists})
}

// POST /api/watchlists {name} — create a named list.
func (s *Server) handleWatchlistsCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	list, err := s.deps.Watchlist().CreateWatchlist(r.Context(), principalUserID(r), payload.Name)
	if err != nil {
		s.writeWatchlistError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, list)
}

// PATCH /api/watchlists/{id} {name} — rename a list.
func (s *Server) handleWatchlistRename(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	if err := s.deps.Watchlist().RenameWatchlist(r.Context(), principalUserID(r), r.PathValue("id"), payload.Name); err != nil {
		s.writeWatchlistError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// DELETE /api/watchlists/{id} — delete a list (items cascade).
func (s *Server) handleWatchlistDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Watchlist().DeleteWatchlist(r.Context(), principalUserID(r), r.PathValue("id")); err != nil {
		s.writeWatchlistError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// GET /api/watchlists/{id}/items — the tickers in a list.
func (s *Server) handleWatchlistItems(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Watchlist().ListItems(r.Context(), principalUserID(r), r.PathValue("id"))
	if err != nil {
		s.writeWatchlistError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// POST /api/watchlists/{id}/items {instrumentId} — add a ticker to a list.
func (s *Server) handleWatchlistItemAdd(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		InstrumentID string `json:"instrumentId"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	if err := s.deps.Watchlist().AddItem(r.Context(), principalUserID(r), r.PathValue("id"), payload.InstrumentID); err != nil {
		s.writeWatchlistError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

// DELETE /api/watchlists/{id}/items/{instrumentId} — remove a ticker from a list.
func (s *Server) handleWatchlistItemRemove(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Watchlist().RemoveItem(r.Context(), principalUserID(r), r.PathValue("id"), r.PathValue("instrumentId")); err != nil {
		s.writeWatchlistError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) writeWatchlistError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, coreservice.ErrInvalidWatchlistInput):
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
	case errors.Is(err, coreservice.ErrWatchlistNotFound):
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Watchlist not found", err)
	default:
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
	}
}
