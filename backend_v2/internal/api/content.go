package api

import (
	"net/http"
	"strings"

	"aladin/backend_v2/internal/docsurface"
)

func (s *Server) registerContentRoutes(mux *http.ServeMux) {
	// The {path...} wildcard also matches the trailing-slash case (empty path),
	// so this single pattern covers both /content/{id}/ and /content/{id}/sub.
	mux.HandleFunc("GET /content/{pageId}/{path...}", s.handleContentServe)
}

// handleContentServe serves a built Doc Surface page as a single self-contained
// HTML document (everything inlined — see docsurface.CSP). Auth is enforced by
// authMiddleware; ownership by Artifacts().Get (scoped to the principal).
func (s *Server) handleContentServe(w http.ResponseWriter, r *http.Request) {
	pageID := strings.TrimSpace(r.PathValue("pageId"))
	if pageID == "" {
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Not found", nil)
		return
	}
	rec, err := s.deps.Artifacts().Get(r.Context(), pageID)
	if err != nil {
		if writeAccessError(w, r, err) {
			return
		}
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Not found", err)
		return
	}
	if rec.Type != "app" {
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Not found", nil)
		return
	}

	// Inline model: the only valid path is the entry document. There are no
	// sub-resources (so no sub-resource auth, and the opaque-origin CSP holds).
	if rel := r.PathValue("path"); rel != "" && rel != "index.html" {
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Not found", nil)
		return
	}

	// The token may appear in the entry URL (desktop iframe auth); never leak it via Referer.
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", docsurface.CSP)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	bundleJS, err := s.deps.DocSurfaceStore().ReadFile(r.Context(), pageID, "dist/bundle.js")
	if err != nil {
		_, _ = w.Write([]byte(docsurface.NotBuiltHTML(rec.Title)))
		return
	}
	bundleCSS, _ := s.deps.DocSurfaceStore().ReadFile(r.Context(), pageID, "dist/bundle.css")
	_, _ = w.Write([]byte(docsurface.EntryHTML(rec.Title, docsurface.TokensCSS, string(bundleCSS), string(bundleJS))))
}
