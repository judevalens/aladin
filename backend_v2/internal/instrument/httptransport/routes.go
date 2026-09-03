// Package httptransport owns the Instrument HTTP adapter.
package httptransport

import (
	"context"
	"net/http"
	"strconv"

	"aladin/backend_v2/internal/httpapi"
	"aladin/backend_v2/internal/instrument"
	"aladin/backend_v2/internal/market"
)

type BarReader interface {
	Get(context.Context, string, string, int) ([]market.Bar, error)
}

// Register mounts the existing instrument search and price-history routes.
func Register(mux *http.ServeMux, instruments instrument.InstrumentService, bars BarReader) {
	mux.HandleFunc("GET /api/instruments/search", func(w http.ResponseWriter, r *http.Request) {
		limit := parseLimit(r)
		hits, err := instruments.Search(r.Context(), r.URL.Query().Get("q"), limit)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, hits)
	})
	mux.HandleFunc("GET /api/instruments/{symbol}/bars", func(w http.ResponseWriter, r *http.Request) {
		items, err := bars.Get(r.Context(), r.PathValue("symbol"), r.URL.Query().Get("timeframe"), parseLimit(r))
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, items)
	})
}

func parseLimit(r *http.Request) int {
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, _ := strconv.Atoi(raw)
		return limit
	}
	return 0
}
