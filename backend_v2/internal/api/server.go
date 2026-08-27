package api

import (
	"aladin/backend_v2/internal/app"
	"aladin/backend_v2/internal/docsurface"
	coreservice "aladin/backend_v2/internal/service"
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
	mux.HandleFunc("GET /api/health/deps", s.handleDepsHealth)
	mux.HandleFunc("GET /api/quote", s.handleQuote)
	s.registerAuthRoutes(mux)

	mux.HandleFunc("GET /api/graph", s.handleEmptyGraph)
	mux.HandleFunc("GET /api/graph-explore/full", s.handleEmptyGraph)
	s.registerGraphRoutes(mux)

	mux.HandleFunc("GET /api/worker/status", s.handleWorkerStatus)
	mux.HandleFunc("GET /api/pipeline/stats", s.handlePipelineStats)

	s.registerArtifactRoutes(mux)
	s.registerPageRoutes(mux)
	s.registerFileRoutes(mux)
	s.registerUnfurlRoutes(mux)
	s.registerContentRoutes(mux)
	s.registerShardRoutes(mux)
	s.registerRelationshipRoutes(mux)
	s.registerGraphPaneRoutes(mux)
	s.registerResearchRoutes(mux)
	s.registerDocumentRoutes(mux)
	s.registerEntityTagRoutes(mux)
	s.registerInstrumentRoutes(mux)
	s.registerSearchRoutes(mux)
	s.registerMarketRoutes(mux)
	s.registerWatchlistRoutes(mux)
	s.registerReadingPositionRoutes(mux)
	s.registerAlertRoutes(mux)
	s.registerCopilotRoutes(mux)
	s.registerEntityContextRoutes(mux)
	s.registerArtifactRefRoutes(mux)
	s.registerRealtimeRoutes(mux)
	s.registerProviderConnectionRoutes(mux)
	s.registerSyncRoutes(mux)

	mux.HandleFunc("GET /api/sources/", s.handleSourcesList)
	mux.HandleFunc("POST /api/sources/", s.handleSourcesCreate)
	mux.HandleFunc("DELETE /api/sources/{id}", s.handleSourcesDelete)

	mux.HandleFunc("GET /api/records/", s.handleRecordsList)
	mux.HandleFunc("POST /api/records/", s.handleRecordsCreate)
	mux.HandleFunc("DELETE /api/records/{id}", s.handleRecordsDelete)
	mux.HandleFunc("POST /api/records/{id}/retry", s.handleRecordsRetry)
	mux.HandleFunc("GET /api/records/{id}/similar", s.handleRecordSimilar)
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
		Handler:           cors(traceRequests(s.authMiddleware(mux))),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s
}

// Handler exposes the fully-wrapped HTTP handler (auth + cors + tracing) for
// in-process integration tests.
func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
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

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
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
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.deps.Auth() == nil {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(coreservice.SessionCookieName)
		if err == nil && strings.TrimSpace(cookie.Value) != "" {
			user, authErr := s.deps.Auth().CurrentUser(r.Context(), cookie.Value)
			if authErr == nil {
				next.ServeHTTP(w, r.WithContext(coreservice.WithPrincipal(r.Context(), coreservice.NewUserSessionPrincipal(user))))
				return
			}
		}
		if authorization := strings.TrimSpace(r.Header.Get("Authorization")); authorization != "" {
			principal, authErr := coreservice.ResolveBearerPrincipal(r.Context(), s.deps.Auth(), authorization)
			if authErr == nil {
				if !contentTokenAllowed(principal, r) {
					writeAPIError(w, r, http.StatusUnauthorized, categoryBadRequest, "Unauthenticated", coreservice.ErrUnauthenticated)
					return
				}
				next.ServeHTTP(w, r.WithContext(coreservice.WithPrincipal(r.Context(), principal)))
				return
			}
		}
		// The realtime WS and Doc Surface content routes can't carry an
		// Authorization header (a WebSocket handshake / an <iframe> load), so
		// they accept the bearer token as an ?access_token query param instead.
		if isRealtimeWebSocketRoute(r) || isContentRoute(r) {
			if token := strings.TrimSpace(r.URL.Query().Get("access_token")); token != "" {
				principal, authErr := coreservice.ResolveBearerPrincipal(r.Context(), s.deps.Auth(), "Bearer "+token)
				if authErr == nil {
					if !contentTokenAllowed(principal, r) {
						writeAPIError(w, r, http.StatusUnauthorized, categoryBadRequest, "Unauthenticated", coreservice.ErrUnauthenticated)
						return
					}
					next.ServeHTTP(w, r.WithContext(coreservice.WithPrincipal(r.Context(), principal)))
					return
				}
			}
		}

		if isPublicRoute(r) {
			next.ServeHTTP(w, r)
			return
		}
		// A browser landing here is almost always a shard whose link navigated the
		// frame off its token-carrying URL; raw JSON tells that person nothing.
		if isBrowserNavigation(r) {
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(docsurface.LostCredentialHTML()))
			return
		}
		writeAPIError(w, r, http.StatusUnauthorized, categoryBadRequest, "Unauthenticated", coreservice.ErrUnauthenticated)
	})
}

// isBrowserNavigation reports whether this is a top-level document load by a
// browser (as opposed to an API/XHR call), so an auth failure can answer in the
// medium the caller actually renders. Accept is the signal: a navigation asks for
// text/html first; fetch/XHR sends application/json or */*.
func isBrowserNavigation(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	for _, part := range strings.Split(r.Header.Get("Accept"), ",") {
		media := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		switch media {
		case "text/html":
			return true
		case "application/json":
			return false
		}
	}
	return false
}

func isPublicRoute(r *http.Request) bool {
	path := r.URL.Path
	if path == "/api/health" || path == "/healthz" || path == "/readyz" || path == "/api/health/deps" || path == "/api/quote" {
		return true
	}
	if path == "/api/auth/register" || path == "/api/auth/login" || path == "/api/auth/logout" {
		return true
	}
	if path == "/api/auth/desktop/register" || path == "/api/auth/desktop/login" {
		return true
	}
	if path == "/api/provider-connections/nango/webhook" {
		return true
	}
	// Vendored Doc Surface deps are public, content-addressed static files.
	if strings.HasPrefix(path, "/vendor/") {
		return true
	}
	return false
}

func isRealtimeWebSocketRoute(r *http.Request) bool {
	return r.Method == http.MethodGet && r.URL.Path == "/api/events/ws"
}

func isContentRoute(r *http.Request) bool {
	return r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/content/")
}

// contentTokenAllowed is the choke point that makes a shard's URL credential
// safe: a content-token principal may ONLY fetch shard documents. The token is
// readable by the shard's own JS (it is in the frame URL) and the shard CSP
// permits outbound calls, so without this a shard could act as the viewer
// against /api. Every other principal type passes through untouched.
func contentTokenAllowed(principal coreservice.Principal, r *http.Request) bool {
	if principal.ActorType != coreservice.ActorTypeContentToken {
		return true
	}
	return isContentRoute(r)
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

func writeAccessError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, coreservice.ErrUnauthenticated):
		writeAPIError(w, r, http.StatusUnauthorized, categoryBadRequest, "Unauthenticated", err)
		return true
	case errors.Is(err, coreservice.ErrForbidden):
		writeAPIError(w, r, http.StatusForbidden, categoryBadRequest, "Forbidden", err)
		return true
	default:
		return false
	}
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

// handleDepsHealth reports each dependency's health for observability — NON-gating: it always
// returns 200 and `ok` reflects only the HARD dependency (Postgres), so a copilot-sidecar blip is
// visible here without failing /readyz and pulling the api out of an LB (that's /readyz's job).
func (s *Server) handleDepsHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	deps := map[string]any{}

	pgOK := s.deps.System().Ready(ctx) == nil
	if pgOK {
		deps["postgres"] = map[string]any{"ok": true}
	} else {
		deps["postgres"] = map[string]any{"ok": false}
	}

	// Copilot-agent sidecar (soft): Status never errors — degraded/unconfigured is a valid state.
	copilot := s.deps.Copilot()
	if !copilot.Configured() {
		deps["copilotAgent"] = map[string]any{"configured": false}
	} else {
		deps["copilotAgent"] = copilot.Status(ctx) // {configured, sidecar, mcp}
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": pgOK, "service": "api", "dependencies": deps})
}

func (s *Server) handleQuote(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, quotes[rand.Intn(len(quotes))])
}

func (s *Server) handleEmptyGraph(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"nodes": []any{}, "edges": []any{}})
}
