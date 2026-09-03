package api

import (
	"context"
	"errors"
	"net/http"

	"aladin/backend_v2/internal/watchlist"
)

// watchlistService is owned by the HTTP consumer. It deliberately contains only
// the operations used by these routes, leaving MCP/default-list compatibility
// methods outside the API boundary.
type watchlistService interface {
	ListWatchlists(context.Context, string) ([]watchlist.Watchlist, error)
	CreateWatchlist(context.Context, string, string) (watchlist.Watchlist, error)
	RenameWatchlist(context.Context, string, string, string) error
	DeleteWatchlist(context.Context, string, string) error
	ListItems(context.Context, string, string) ([]watchlist.WatchlistItem, error)
	AddItem(context.Context, string, string, string) error
	RemoveItem(context.Context, string, string, string) error
}

type watchlistRoutes struct {
	service watchlistService
}

func newWatchlistRoutes(service watchlistService) watchlistRoutes {
	return watchlistRoutes{service: service}
}

func (routes watchlistRoutes) register(mux *http.ServeMux) {
	// Lists (universes).
	mux.HandleFunc("GET /api/watchlists", routes.handleWatchlistsList)
	mux.HandleFunc("POST /api/watchlists", routes.handleWatchlistsCreate)
	mux.HandleFunc("PATCH /api/watchlists/{id}", routes.handleWatchlistRename)
	mux.HandleFunc("DELETE /api/watchlists/{id}", routes.handleWatchlistDelete)
	// Members of a list.
	mux.HandleFunc("GET /api/watchlists/{id}/items", routes.handleWatchlistItems)
	mux.HandleFunc("POST /api/watchlists/{id}/items", routes.handleWatchlistItemAdd)
	mux.HandleFunc("DELETE /api/watchlists/{id}/items/{instrumentId}", routes.handleWatchlistItemRemove)
}

// GET /api/watchlists — the signed-in user's watchlists (with item counts).
func (routes watchlistRoutes) handleWatchlistsList(w http.ResponseWriter, r *http.Request) {
	lists, err := routes.service.ListWatchlists(r.Context(), principalUserID(r))
	if err != nil {
		writeWatchlistError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"watchlists": lists})
}

// POST /api/watchlists {name} — create a named list.
func (routes watchlistRoutes) handleWatchlistsCreate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	list, err := routes.service.CreateWatchlist(r.Context(), principalUserID(r), payload.Name)
	if err != nil {
		writeWatchlistError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, list)
}

// PATCH /api/watchlists/{id} {name} — rename a list.
func (routes watchlistRoutes) handleWatchlistRename(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	if err := routes.service.RenameWatchlist(r.Context(), principalUserID(r), r.PathValue("id"), payload.Name); err != nil {
		writeWatchlistError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// DELETE /api/watchlists/{id} — delete a list (items cascade).
func (routes watchlistRoutes) handleWatchlistDelete(w http.ResponseWriter, r *http.Request) {
	if err := routes.service.DeleteWatchlist(r.Context(), principalUserID(r), r.PathValue("id")); err != nil {
		writeWatchlistError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// GET /api/watchlists/{id}/items — the tickers in a list.
func (routes watchlistRoutes) handleWatchlistItems(w http.ResponseWriter, r *http.Request) {
	items, err := routes.service.ListItems(r.Context(), principalUserID(r), r.PathValue("id"))
	if err != nil {
		writeWatchlistError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// POST /api/watchlists/{id}/items {instrumentId} — add a ticker to a list.
func (routes watchlistRoutes) handleWatchlistItemAdd(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		InstrumentID string `json:"instrumentId"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	if err := routes.service.AddItem(r.Context(), principalUserID(r), r.PathValue("id"), payload.InstrumentID); err != nil {
		writeWatchlistError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

// DELETE /api/watchlists/{id}/items/{instrumentId} — remove a ticker from a list.
func (routes watchlistRoutes) handleWatchlistItemRemove(w http.ResponseWriter, r *http.Request) {
	if err := routes.service.RemoveItem(r.Context(), principalUserID(r), r.PathValue("id"), r.PathValue("instrumentId")); err != nil {
		writeWatchlistError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func writeWatchlistError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, watchlist.ErrInvalidInput):
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, err.Error(), err)
	case errors.Is(err, watchlist.ErrNotFound):
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Watchlist not found", err)
	default:
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
	}
}
