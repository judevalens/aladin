package api

import (
	"aladin/backend_v2/internal/app"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	httpServer *http.Server
	deps       app.Dependencies
}

type contextKey string

type errorCategory string

const (
	requestIDHeader            = "X-Request-Id"
	requestIDKey    contextKey = "request_id"

	categoryDecodeError  errorCategory = "decode_error"
	categoryBadRequest   errorCategory = "bad_request"
	categoryNotFound     errorCategory = "not_found"
	categoryServiceError errorCategory = "service_error"
	categoryInternal     errorCategory = "internal_error"
)

func New(addr string, pool *pgxpool.Pool) *Server {
	return NewWithDependencies(addr, app.NewDependencies(pool))
}

func NewWithDependencies(addr string, deps app.Dependencies) *Server {
	s := &Server{deps: deps}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /api/quote", s.handleQuote)

	mux.HandleFunc("GET /api/graph", s.handleEmptyGraph)
	mux.HandleFunc("GET /api/graph-explore/full", s.handleEmptyGraph)

	mux.HandleFunc("GET /api/worker/status", s.handleWorkerStatus)

	s.registerArtifactRoutes(mux)
	s.registerPageRoutes(mux)
	s.registerFileRoutes(mux)
	s.registerRealtimeRoutes(mux)

	mux.HandleFunc("GET /api/sources/", s.handleSourcesList)
	mux.HandleFunc("POST /api/sources/", s.handleSourcesCreate)
	mux.HandleFunc("DELETE /api/sources/{id}", s.handleSourcesDelete)

	mux.HandleFunc("GET /api/records/", s.handleRecordsList)
	mux.HandleFunc("POST /api/records/", s.handleRecordsCreate)
	mux.HandleFunc("DELETE /api/records/{id}", s.handleRecordsDelete)
	mux.HandleFunc("GET /api/records/{id}/children", s.handleRecordChildren)

	mux.HandleFunc("GET /api/feed/", s.handleFeedList)
	mux.HandleFunc("GET /api/feed/topics", s.handleFeedTopics)
	mux.HandleFunc("GET /api/feed/sources", s.handleFeedSources)
	mux.HandleFunc("POST /api/feed/{id}/save", s.handleFeedSave)
	mux.HandleFunc("POST /api/feed/{id}/dismiss", s.handleFeedDismiss)
	mux.HandleFunc("POST /api/feed/{id}/unsave", s.handleFeedUnsave)

	mux.HandleFunc("GET /api/insights/", s.handleInsightsList)
	mux.HandleFunc("GET /api/insights/stats", s.handleInsightsStats)
	mux.HandleFunc("POST /api/insights/{id}/accept", s.handleInsightAccept)
	mux.HandleFunc("POST /api/insights/{id}/dismiss", s.handleInsightDismiss)

	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           cors(traceRequests(mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s
}

func (s *Server) Run() error {
	slog.Info("api: starting http server", "component", "api", "addr", s.httpServer.Addr)
	err := s.httpServer.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func traceRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get(requestIDHeader))
		if requestID == "" {
			requestID = uuid.NewString()
		}
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		w.Header().Set(requestIDHeader, requestID)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rec, r.WithContext(ctx))
		slog.Info(
			"api: request completed",
			"component", "api",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func readJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return errors.New("missing request body")
	}
	return json.NewDecoder(r.Body).Decode(dst)
}

func requestIDFromContext(ctx context.Context) string {
	if value, ok := ctx.Value(requestIDKey).(string); ok {
		return value
	}
	return ""
}

func writeAPIError(w http.ResponseWriter, r *http.Request, status int, category errorCategory, publicMessage string, err error) {
	attrs := []any{
		"component", "api",
		"request_id", requestIDFromContext(r.Context()),
		"method", r.Method,
		"path", r.URL.Path,
		"status", status,
		"category", string(category),
	}
	if err != nil {
		attrs = append(attrs, "err", err)
	} else {
		attrs = append(attrs, "message", publicMessage)
	}
	slog.Error("api: request failed", attrs...)
	writeJSON(w, status, map[string]string{"error": publicMessage})
}

func writeDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error(
		"api: request decode failed",
		"component", "api",
		"request_id", requestIDFromContext(r.Context()),
		"method", r.Method,
		"path", r.URL.Path,
		"status", http.StatusBadRequest,
		"category", string(categoryDecodeError),
		"content_type", r.Header.Get("Content-Type"),
		"err", err,
	)
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
}

func intQuery(r *http.Request, key string, fallback int) int {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func floatQuery(r *http.Request, key string, fallback float64) float64 {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return n
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

var quotes = []map[string]string{
	{"text": "The more that you read, the more things you will know.", "author": "Dr. Seuss"},
	{"text": "Knowledge is power.", "author": "Francis Bacon"},
	{"text": "An investment in knowledge pays the best interest.", "author": "Benjamin Franklin"},
	{"text": "The only true wisdom is in knowing you know nothing.", "author": "Socrates"},
	{"text": "Research is formalized curiosity.", "author": "Zora Neale Hurston"},
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "api"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.System().Ready(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "service": "api", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "api"})
}

func (s *Server) handleQuote(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, quotes[rand.Intn(len(quotes))])
}

func (s *Server) handleEmptyGraph(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"nodes": []any{}, "edges": []any{}})
}
