package api

import (
	"net/http"
	"strconv"
)

func (s *Server) registerInstrumentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/instruments/search", s.handleInstrumentSearch)
	mux.HandleFunc("GET /api/instruments/{symbol}/bars", s.handleInstrumentBars)
}

// GET /api/instruments/{symbol}/bars?timeframe=1Day&limit=180 — OHLCV history for the chart.
func (s *Server) handleInstrumentBars(w http.ResponseWriter, r *http.Request) {
	timeframe := r.URL.Query().Get("timeframe")
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, _ = strconv.Atoi(raw)
	}
	bars, err := s.deps.Bars().Get(r.Context(), r.PathValue("symbol"), timeframe, limit)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, bars)
}

// GET /api/instruments/search?q=…&limit=… — ticker typeahead for the command box.
func (s *Server) handleInstrumentSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, _ = strconv.Atoi(raw)
	}
	hits, err := s.deps.Instruments().Search(r.Context(), q, limit)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, hits)
}
