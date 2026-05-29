package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"aladin/backend_v2/internal/repo"
	coreservice "aladin/backend_v2/internal/service"
)

// Data-layer redesign — the sync endpoints.
// Plan: ~/.claude/plans/data-layer-sync-model.md.
// Phase A: GET /api/sync/pull (read/convergence). Phase B: POST /api/sync/push
// (the generic, idempotent write path).

func (s *Server) registerSyncRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sync/pull", s.handleSyncPull)
	mux.HandleFunc("POST /api/sync/push", s.handleSyncPush)
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

type syncPushRequest struct {
	Mutations []repo.Mutation `json:"mutations"`
}

type syncPushResult struct {
	MutationID string `json:"mutationId"`
	Status     string `json:"status"` // "applied" | "duplicate" | "error"
	Error      string `json:"error,omitempty"`
}

type syncPushResponse struct {
	Results []syncPushResult `json:"results"`
}

// handleSyncPush applies a batch of client mutations (the generic write path)
// for the authenticated user and returns a per-mutation result. Each mutation
// is applied independently and idempotently (per-client high-water dedup), so a
// retried batch yields "duplicate" for what already landed; a permanently bad
// mutation yields "error" without blocking the rest. The client marks
// applied/duplicate as done and retries (bounded) or drops on error.
func (s *Server) handleSyncPush(w http.ResponseWriter, r *http.Request) {
	principal, ok := coreservice.PrincipalFromContext(r.Context())
	if !ok {
		writeAPIError(w, r, http.StatusUnauthorized, categoryBadRequest, "Unauthenticated", coreservice.ErrUnauthenticated)
		return
	}
	var req syncPushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "invalid request body", err)
		return
	}

	results := make([]syncPushResult, 0, len(req.Mutations))
	applied := 0
	for _, m := range req.Mutations {
		ack, err := s.deps.Sync().ApplyMutation(r.Context(), principal.UserID, m)
		if err != nil {
			results = append(results, syncPushResult{MutationID: m.MutationID, Status: "error", Error: err.Error()})
			continue
		}
		if ack.Status == "applied" {
			applied++
		}
		results = append(results, syncPushResult{MutationID: ack.MutationID, Status: ack.Status})
	}

	// Data-layer redesign, Phase C — poke the user's other clients to pull the
	// feed immediately (rather than waiting for their poll). We publish on the
	// workspace stream, which their realtime subscription already covers; the
	// client treats any inbound event as a "pull now" signal. Publish is
	// user-scoped via the request principal. Best-effort (nil in tests).
	if applied > 0 {
		if rt := s.deps.Realtime(); rt != nil {
			_ = rt.Publish(r.Context(), coreservice.PublishTarget{
				Stream:       coreservice.WorkspaceStream,
				ResourceKind: "folder",
				ResourceID:   principal.UserID,
				Operation:    "updated",
			}, map[string]any{"poke": true})
		}
	}

	writeJSON(w, http.StatusOK, syncPushResponse{Results: results})
}
