package api

import (
	"net/http"
	"strconv"

	coreservice "aladin/backend_v2/internal/service"
)

// Data-layer redesign, Phase A — the read/convergence endpoint.
// Plan: ~/.claude/plans/data-layer-sync-model.md.

func (s *Server) registerSyncRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sync/pull", s.handleSyncPull)
}

// handleSyncPull returns the coalesced change-feed delta for the authenticated
// user since their cursor (?since=<seq>, default 0). The client advances its
// cursor to the returned cursor. (Snapshot fallback for a too-old cursor is the
// next increment.)
func (s *Server) handleSyncPull(w http.ResponseWriter, r *http.Request) {
	principal, ok := coreservice.PrincipalFromContext(r.Context())
	if !ok {
		writeAPIError(w, r, http.StatusUnauthorized, categoryBadRequest, "Unauthenticated", coreservice.ErrUnauthenticated)
		return
	}
	var cursor int64
	if v := r.URL.Query().Get("since"); v != "" {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil || parsed < 0 {
			writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "invalid 'since' cursor", err)
			return
		}
		cursor = parsed
	}
	res, err := s.deps.Sync().PullDelta(r.Context(), principal.UserID, cursor)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
