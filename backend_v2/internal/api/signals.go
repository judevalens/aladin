package api

import (
	"net/http"

	coreservice "aladin/backend_v2/internal/service"
)

// GET /api/signals — the Signals surface: shared claims as ranked signal cards.
// ?limit, ?offset, ?sort=recent|top.
func (s *Server) handleSignalsList(w http.ResponseWriter, r *http.Request) {
	sort := r.URL.Query().Get("sort")
	if sort != "top" {
		sort = "recent"
	}
	params := coreservice.SignalListParams{
		Limit:  min(intQuery(r, "limit", 30), 100),
		Offset: intQuery(r, "offset", 0),
		Sort:   sort,
	}
	out, err := s.deps.Signals().List(r.Context(), params)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
