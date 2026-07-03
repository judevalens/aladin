package api

import (
	"net/http"

	coreservice "aladin/backend_v2/internal/service"
)

// GET /api/signals — the Signals surface as ranked signal cards.
// ?limit, ?offset, ?sort=recent|top, ?lens=inbox|book.
// lens=inbox (default) is the shared fishing feed; lens=book is the caller's authored
// theses, each marked to market by supporting/contradicting discovered sources.
func (s *Server) handleSignalsList(w http.ResponseWriter, r *http.Request) {
	sort := r.URL.Query().Get("sort")
	if sort != "top" {
		sort = "recent"
	}
	lens := r.URL.Query().Get("lens")
	if lens != "book" {
		lens = "inbox"
	}
	params := coreservice.SignalListParams{
		Limit:       min(intQuery(r, "limit", 30), 100),
		Offset:      intQuery(r, "offset", 0),
		Sort:        sort,
		Lens:        lens,
		OwnerUserID: principalUserID(r),
	}
	out, err := s.deps.Signals().List(r.Context(), params)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
