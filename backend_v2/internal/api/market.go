package api

import "net/http"

func (s *Server) registerMarketRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/market/subscribe", s.handleMarketSubscribe)
	mux.HandleFunc("POST /api/market/unsubscribe", s.handleMarketUnsubscribe)
}

type marketSubscribePayload struct {
	Symbols []string `json:"symbols"`
}

// POST /api/market/subscribe {symbols:[…]} — register live-quote demand. The hub dedups to a
// single upstream Alpaca subscription per symbol; ticks fan out over the market stream.
func (s *Server) handleMarketSubscribe(w http.ResponseWriter, r *http.Request) {
	var payload marketSubscribePayload
	if err := readJSON(r, &payload); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	if err := s.deps.MarketData().Subscribe(r.Context(), payload.Symbols); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// POST /api/market/unsubscribe {symbols:[…]} — release live-quote demand.
func (s *Server) handleMarketUnsubscribe(w http.ResponseWriter, r *http.Request) {
	var payload marketSubscribePayload
	if err := readJSON(r, &payload); err != nil {
		writeDecodeError(w, r, err)
		return
	}
	if err := s.deps.MarketData().Unsubscribe(r.Context(), payload.Symbols); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
