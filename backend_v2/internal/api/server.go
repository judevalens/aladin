package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	httpServer *http.Server
	deps       Dependencies
}

func New(addr string, pool *pgxpool.Pool) *Server {
	return NewWithDependencies(addr, NewDependencies(pool))
}

func NewWithDependencies(addr string, deps Dependencies) *Server {
	s := &Server{deps: deps}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /api/quote", s.handleQuote)

	mux.HandleFunc("GET /api/graph", s.handleEmptyGraph)
	mux.HandleFunc("GET /api/graph-explore/full", s.handleEmptyGraph)

	mux.HandleFunc("GET /api/worker/status", s.handleWorkerStatus)

	mux.HandleFunc("POST /api/documents/upload", s.handleDocumentsUpload)
	mux.HandleFunc("GET /api/documents/", s.handleDocumentsList)
	mux.HandleFunc("GET /api/documents/{id}/file", s.handleDocumentFile)

	mux.HandleFunc("POST /api/audio/upload", s.handleAudioUpload)
	mux.HandleFunc("GET /api/audio/{filename}", s.handleAudioFile)

	mux.HandleFunc("POST /api/notes/", s.handleNotesCreate)
	mux.HandleFunc("PATCH /api/notes/{id}", s.handleNotesUpdate)
	mux.HandleFunc("DELETE /api/notes/{id}", s.handleNotesDelete)
	mux.HandleFunc("GET /api/notes/{id}/related", s.handleNotesRelated)

	mux.HandleFunc("GET /api/sources/", s.handleSourcesList)
	mux.HandleFunc("POST /api/sources/", s.handleSourcesCreate)
	mux.HandleFunc("DELETE /api/sources/{id}", s.handleSourcesDelete)

	mux.HandleFunc("GET /api/artifacts/", s.handleArtifactsList)
	mux.HandleFunc("POST /api/artifacts/", s.handleArtifactsCreate)
	mux.HandleFunc("DELETE /api/artifacts/{id}", s.handleArtifactsDelete)
	mux.HandleFunc("GET /api/artifacts/{id}/children", s.handleArtifactChildren)

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
		Handler:           cors(logging(mux)),
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

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("api: request", "component", "api", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
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
	if err := s.deps.System.Ready(r.Context()); err != nil {
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
