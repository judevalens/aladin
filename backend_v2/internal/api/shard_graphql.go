package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardv2"
)

func (s *Server) handleShardGraphQL(w http.ResponseWriter, r *http.Request) {
	if !s.ownedShard(w, r, r.PathValue("id")) {
		return
	}
	graph := s.deps.ShardGraphQL()
	if graph == nil || !graph.Enabled() {
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Shard GraphQL runtime is disabled", service.ErrNotFound)
		return
	}
	var request service.ShardGraphQLRequest
	if err := decodeShardRuntimeBody(w, r, &request); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, categoryDecodeError, "Invalid GraphQL operation", err)
		return
	}
	ctx, cancel := contextWithRuntimeTimeout(r, 35*time.Second)
	defer cancel()
	result, err := graph.Execute(ctx, resourceTarget(r), request)
	if err != nil {
		writeShardRuntimeError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result)
}

func (s *Server) handleShardLambda(w http.ResponseWriter, r *http.Request) {
	if !s.ownedShard(w, r, r.PathValue("id")) {
		return
	}
	graph := s.deps.ShardGraphQL()
	if graph == nil || !graph.Enabled() {
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Shard runtime is disabled", service.ErrNotFound)
		return
	}
	var body struct {
		Input map[string]any `json:"input"`
	}
	if err := decodeShardRuntimeBody(w, r, &body); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, categoryDecodeError, "Invalid lambda input", err)
		return
	}
	ctx, cancel := contextWithRuntimeTimeout(r, 35*time.Second)
	defer cancel()
	result, err := graph.InvokeLambda(ctx, resourceTarget(r), service.ShardLambdaRequest{Name: r.PathValue("name"), Input: body.Input})
	if err != nil {
		writeShardRuntimeError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(result)
}

func (s *Server) handleShardRuntimeCapability(w http.ResponseWriter, r *http.Request) {
	graph := s.deps.ShardGraphQL()
	if graph == nil || !graph.Enabled() {
		http.NotFound(w, r)
		return
	}
	var request service.RuntimeCapabilityRequest
	if err := decodeShardRuntimeBody(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	result, err := graph.Capability(r.Context(), r.Header.Get("Authorization"), request)
	if err != nil {
		code := service.ResourceErrorCode(err)
		status := http.StatusBadRequest
		switch code {
		case "forbidden":
			status = http.StatusForbidden
		case "not-found":
			status = http.StatusNotFound
		case "contract-changed", "conflict", "stale-cursor":
			status = http.StatusConflict
		case "rate-limited", "quota":
			status = http.StatusTooManyRequests
		case "source-unavailable":
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, map[string]string{"error": "Capability request failed", "code": code})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func decodeShardRuntimeBody(w http.ResponseWriter, r *http.Request, value any) error {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, shardv2.MaxJSONBytes))
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain exactly one JSON value")
	}
	return nil
}

func contextWithRuntimeTimeout(r *http.Request, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), timeout)
}

func writeShardRuntimeError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusBadRequest
	switch service.ResourceErrorCode(err) {
	case "forbidden":
		status = http.StatusForbidden
	case "not-found":
		status = http.StatusNotFound
	case "contract-changed":
		status = http.StatusConflict
	case "source-unavailable":
		status = http.StatusServiceUnavailable
	case "rate-limited", "quota":
		status = http.StatusTooManyRequests
	}
	code := service.ResourceErrorCode(err)
	slog.Error("api: shard runtime request failed", "component", "api", "request_id", requestIDFromContext(r.Context()), "method", r.Method, "path", r.URL.Path, "status", status, "category", string(categoryServiceError), "code", code, "err", err)
	writeJSON(w, status, map[string]string{"error": "Shard runtime request failed", "code": code})
}
