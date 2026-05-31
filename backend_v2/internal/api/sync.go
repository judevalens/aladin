package api

import (
	"net/http"
	"strconv"

	coreservice "aladin/backend_v2/internal/service"
)

// Data-layer R1 — the sync pull endpoint (server-authoritative outbox).
// Architecture: ~/.claude/plans/data-layer-offline-readable.md.
//
// The server is the ONLY writer; the client is a read cache. Writes go through
// the typed REST endpoints (artifacts/folders/browser nodes), which append
// outbox frames transactionally. The client pulls frames since its cursor and
// replays them (live delivery rides the realtime websocket). There is no client
// push path.

func (s *Server) registerSyncRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sync/pull", s.handleSyncPull)
}

// handleSyncPull returns the outbox frames for the authenticated user since
// their cursor (?since=<xid, decimal string>, default 0 = cold start → snapshot).
// The response carries the new cursor (the log horizon) and a mode flag
// (delta | snapshot); the client advances its cursor to the returned value.
func (s *Server) handleSyncPull(w http.ResponseWriter, r *http.Request) {
	principal, ok := coreservice.PrincipalFromContext(r.Context())
	if !ok {
		writeAPIError(w, r, http.StatusUnauthorized, categoryBadRequest, "Unauthenticated", coreservice.ErrUnauthenticated)
		return
	}
	var cursor uint64
	if v := r.URL.Query().Get("since"); v != "" {
		parsed, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "invalid 'since' cursor", err)
			return
		}
		cursor = parsed
	}
	res, err := s.deps.Sync().Pull(r.Context(), principal.UserID, cursor)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
