// Package httptransport owns the public HTTP adapter for Market Data.
package httptransport

import (
	"net/http"

	"aladin/backend_v2/internal/httpapi"
	"aladin/backend_v2/internal/market"
)

type marketSubscribePayload struct {
	Symbols []string `json:"symbols"`
}

// POST /api/market/subscribe {symbols:[…]} — register live-quote demand. The hub dedups to a
// single upstream Alpaca subscription per symbol; ticks fan out over the market stream.
// Register installs the existing market subscription routes without changing
// their public paths or wire contracts.
func Register(mux *http.ServeMux, service market.MarketDataService) {
	handle := func(subscribe bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var payload marketSubscribePayload
			if err := httpapi.ReadJSON(r, &payload); err != nil {
				httpapi.WriteDecodeError(w, r, err)
				return
			}
			var err error
			if subscribe {
				err = service.Subscribe(r.Context(), payload.Symbols)
			} else {
				err = service.Unsubscribe(r.Context(), payload.Symbols)
			}
			if err != nil {
				httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
				return
			}
			httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
		}
	}
	mux.HandleFunc("POST /api/market/subscribe", handle(true))
	mux.HandleFunc("POST /api/market/unsubscribe", handle(false))
}
