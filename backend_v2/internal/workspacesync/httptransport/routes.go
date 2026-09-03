package httptransport

import (
	"net/http"
	"strconv"

	"aladin/backend_v2/internal/auth"
	"aladin/backend_v2/internal/httpapi"
	"aladin/backend_v2/internal/workspacesync"
)

// Data-layer R1 — the sync pull endpoint (server-authoritative outbox).
// Architecture: ~/.claude/plans/data-layer-offline-readable.md.
//
// The server is the ONLY writer; the client is a read cache. Writes go through
// the typed REST endpoints (artifacts/folders/browser nodes), which append
// outbox frames transactionally. The client pulls frames since its cursor and
// replays them (live delivery rides the realtime websocket). There is no client
// push path.

type routes struct {
	service workspacesync.SyncService
}

func Register(mux *http.ServeMux, service workspacesync.SyncService) {
	h := routes{service: service}
	mux.HandleFunc("GET /api/sync/pull", h.handleSyncPull)
}

// handleSyncPull returns the outbox frames for the authenticated user since
// their cursor (?since=<xid, decimal string>, default 0 = cold start → snapshot).
// The response carries the new cursor (the log horizon) and a mode flag
// (delta | snapshot); the client advances its cursor to the returned value.
func (h routes) handleSyncPull(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "bad_request", "Unauthenticated", auth.ErrUnauthenticated)
		return
	}
	var cursor uint64
	if v := r.URL.Query().Get("since"); v != "" {
		parsed, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "bad_request", "invalid 'since' cursor", err)
			return
		}
		cursor = parsed
	}
	res, err := h.service.Pull(r.Context(), principal.UserID, cursor)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, res)
}
