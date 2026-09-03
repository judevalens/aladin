// Package httptransport owns the global-search HTTP adapter.
package httptransport

import (
	"net/http"
	"strconv"

	"aladin/backend_v2/internal/httpapi"
	"aladin/backend_v2/internal/search"
)

// Register mounts the existing global-search contract.
func Register(mux *http.ServeMux, service search.SearchService) {
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		limit := 0
		if raw := r.URL.Query().Get("limit"); raw != "" {
			limit, _ = strconv.Atoi(raw)
		}
		out, err := service.Search(r.Context(), httpapi.PrincipalUserID(r), r.URL.Query().Get("q"), limit)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, out)
	})
}
