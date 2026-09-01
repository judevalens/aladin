package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"aladin/backend_v2/internal/docsurface"
	"aladin/backend_v2/internal/service"
)

func (s *Server) registerShardRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/shard-resources/ws", s.handleShardResourceSocket)
	mux.HandleFunc("GET /api/shards/{id}/release", s.handleShardRelease)
	mux.HandleFunc("POST /api/shards/{id}/v2/{environment}/request", s.handleShardResourceRequest)
	mux.HandleFunc("GET /api/shards/{id}/v2/{environment}/ws", s.handleShardResourceSocket)
	// Build status for the live work-pane view. Authed (authMiddleware) + ownership
	// scoped via Artifacts().Get; the realtime build-status events seed/update the
	// same shape live, this GET seeds it when a shard tab first opens.
	mux.HandleFunc("GET /api/shards/{id}/build-state", s.handleShardBuildState)
	// The parsed anchor manifest (for the host: provenance, deep links, and the
	// M3 bridge's ref grant). 404 if the shard has no anchors.json.
	mux.HandleFunc("GET /api/shards/{id}/manifest", s.handleShardManifest)
	// Shard local state (design/SHARD_LOCAL_STATE.md): the host bridge's storage
	// backend. ?channel selects published (default — the user's data) or draft
	// (the agent sandbox). Writes are revision-guarded; a lost write is a 409
	// carrying the current value + revision.
	// The workspace plane: the host proxies a shard's nodes.get here, and the
	// grant (ids ⊆ anchors.json refs) is re-checked server-side — the host is
	// never the only gate.
	mux.HandleFunc("POST /api/shards/{id}/bridge/nodes", s.handleShardBridgeNodes)
	mux.HandleFunc("GET /api/shards/{id}/kv", s.handleShardKVList)
	mux.HandleFunc("GET /api/shards/{id}/kv/{key...}", s.handleShardKVGet)
	mux.HandleFunc("PUT /api/shards/{id}/kv/{key...}", s.handleShardKVSet)
	mux.HandleFunc("DELETE /api/shards/{id}/kv/{key...}", s.handleShardKVDelete)
}

func (s *Server) handleShardRelease(w http.ResponseWriter, r *http.Request) {
	if !s.ownedShard(w, r, r.PathValue("id")) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if releases := s.deps.ShardReleases(); releases != nil {
		release, err := releases.Active(r.Context(), r.PathValue("id"), docsurface.ParseChannel(r.URL.Query().Get("channel")))
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"protocol": "bridge/2", "buildId": release.BuildID, "contractHash": release.Hash})
			return
		}
		if !errors.Is(err, service.ErrNotFound) {
			writeAPIError(w, r, http.StatusServiceUnavailable, categoryInternal, "Release unavailable", err)
			return
		}
	}
	// No protected release does not imply a usable legacy build. In particular,
	// a new resource app has no published content until activation succeeds.
	entries, err := s.deps.DocSurfaceStore().ListDir(r.Context(), r.PathValue("id"), docsurface.DistDir(docsurface.ParseChannel(r.URL.Query().Get("channel"))))
	if err != nil && !errors.Is(err, service.ErrNotFound) && !os.IsNotExist(err) {
		writeAPIError(w, r, http.StatusServiceUnavailable, categoryInternal, "Release unavailable", err)
		return
	}
	available := false
	for _, entry := range entries {
		if entry.Name == "bundle.js" && !entry.IsDir {
			available = true
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"protocol": "bridge/1", "available": available})
}

// ownedShard verifies the path id names an "app" artifact owned by the principal,
// writing the appropriate error and returning false otherwise.
func (s *Server) ownedShard(w http.ResponseWriter, r *http.Request, pageID string) bool {
	if pageID == "" {
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Not found", nil)
		return false
	}
	rec, err := s.deps.Artifacts().Get(r.Context(), pageID)
	if err != nil {
		if writeAccessError(w, r, err) {
			return false
		}
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Not found", err)
		return false
	}
	if rec.Type != "app" {
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Not found", nil)
		return false
	}
	return true
}

func (s *Server) handleShardBuildState(w http.ResponseWriter, r *http.Request) {
	pageID := strings.TrimSpace(r.PathValue("id"))
	if !s.ownedShard(w, r, pageID) {
		return
	}
	channel := docsurface.ParseChannel(r.URL.Query().Get("channel"))
	st, err := s.deps.ShardBuild().State(r.Context(), pageID, channel)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryInternal, "build state unavailable", err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleShardBridgeNodes(w http.ResponseWriter, r *http.Request) {
	pageID := strings.TrimSpace(r.PathValue("id"))
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "invalid JSON body", err)
		return
	}
	nodes, missing, err := s.deps.ShardBridge().GetNodes(r.Context(), pageID, body.IDs)
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		var bad service.BadRequest
		if errors.As(err, &bad) {
			writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, bad.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrNotFound) {
			writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Not found", nil)
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, categoryInternal, "bridge read failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"nodes": nodes, "missing": missing})
}

// shardKVConflictBody is the doc's 409 shape: the latest value + revision so the
// client re-applies and retries.
type shardKVConflictBody struct {
	Error           string          `json:"error"` // always "conflict"
	Key             string          `json:"key"`
	CurrentRevision int64           `json:"currentRevision"`
	CurrentValue    json.RawMessage `json:"currentValue"`
	Deleted         bool            `json:"deleted,omitempty"`
}

// writeShardKVError maps service errors: ShardKVConflict → 409 with the current
// entry; everything else falls through to the standard access/bad-request paths.
func writeShardKVError(w http.ResponseWriter, r *http.Request, err error) {
	var conflict *service.ShardKVConflict
	if errors.As(err, &conflict) {
		writeJSON(w, http.StatusConflict, shardKVConflictBody{
			Error:           "conflict",
			Key:             conflict.Current.Key,
			CurrentRevision: conflict.Current.Revision,
			CurrentValue:    conflict.Current.Value,
			Deleted:         conflict.Current.Deleted,
		})
		return
	}
	if writeAccessError(w, r, err) {
		return
	}
	var bad service.BadRequest
	if errors.As(err, &bad) {
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, bad.Error(), nil)
		return
	}
	if errors.Is(err, service.ErrNotFound) {
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Not found", nil)
		return
	}
	writeAPIError(w, r, http.StatusInternalServerError, categoryInternal, "shard state unavailable", err)
}

func shardKVChannel(r *http.Request) service.BuildChannel {
	// Published is the default: it is the user's real data and what the pane
	// binds; draft is the agent's sandbox (preview/emulator, MCP flows).
	if r.URL.Query().Get("channel") == string(service.ChannelDraft) {
		return service.ChannelDraft
	}
	return service.ChannelPublished
}

func (s *Server) handleShardKVList(w http.ResponseWriter, r *http.Request) {
	pageID := strings.TrimSpace(r.PathValue("id"))
	entries, err := s.deps.ShardKV().List(r.Context(), pageID, shardKVChannel(r), r.URL.Query().Get("prefix"))
	if err != nil {
		writeShardKVError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (s *Server) handleShardKVGet(w http.ResponseWriter, r *http.Request) {
	pageID := strings.TrimSpace(r.PathValue("id"))
	entry, ok, err := s.deps.ShardKV().Get(r.Context(), pageID, shardKVChannel(r), r.PathValue("key"))
	if err != nil {
		writeShardKVError(w, r, err)
		return
	}
	if !ok {
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "no such key", nil)
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) handleShardKVSet(w http.ResponseWriter, r *http.Request) {
	pageID := strings.TrimSpace(r.PathValue("id"))
	var body struct {
		Value        json.RawMessage `json:"value"`
		BaseRevision int64           `json:"baseRevision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "invalid JSON body", err)
		return
	}
	entry, err := s.deps.ShardKV().Set(r.Context(), pageID, shardKVChannel(r), r.PathValue("key"), body.Value, body.BaseRevision)
	if err != nil {
		writeShardKVError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) handleShardKVDelete(w http.ResponseWriter, r *http.Request) {
	pageID := strings.TrimSpace(r.PathValue("id"))
	baseRevision, err := strconv.ParseInt(r.URL.Query().Get("baseRevision"), 10, 64)
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, categoryBadRequest, "baseRevision query param required", err)
		return
	}
	if err := s.deps.ShardKV().Delete(r.Context(), pageID, shardKVChannel(r), r.PathValue("key"), baseRevision); err != nil {
		writeShardKVError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleShardManifest(w http.ResponseWriter, r *http.Request) {
	pageID := strings.TrimSpace(r.PathValue("id"))
	if !s.ownedShard(w, r, pageID) {
		return
	}
	data, err := s.deps.DocSurfaceStore().ReadFile(r.Context(), pageID, docsurface.ManifestFileName)
	if releases := s.deps.ShardReleases(); releases != nil {
		active, activeErr := releases.Active(r.Context(), pageID, docsurface.ParseChannel(r.URL.Query().Get("channel")))
		if activeErr == nil {
			data, err = active.Files[docsurface.ManifestFileName], nil
		} else if !errors.Is(activeErr, service.ErrNotFound) {
			writeAPIError(w, r, http.StatusServiceUnavailable, categoryInternal, "Release unavailable", activeErr)
			return
		}
	}
	if err != nil {
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "no manifest", nil)
		return
	}
	m, err := docsurface.ParseManifest(data)
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, categoryServiceError, "manifest is not valid JSON", err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}
