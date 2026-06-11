package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"aladin/backend_v2/internal/docsurface"
)

func (s *Server) registerContentRoutes(mux *http.ServeMux) {
	// The {path...} wildcard also matches the trailing-slash case (empty path),
	// so this single pattern covers both /content/{id}/ and /content/{id}/sub.
	mux.HandleFunc("GET /content/{pageId}/{path...}", s.handleContentServe)
	// Public, content-addressed vendored deps (shared across all docs). {sha} is a
	// single path segment, so traversal can't reach it.
	mux.HandleFunc("GET /vendor/{sha}", s.handleVendorServe)
}

// handleVendorServe serves a content-addressed vendored dependency file. It is
// PUBLIC (no auth — public libs, no user data; see isPublicRoute) and immutable.
// It overrides the global CORS so opaque-origin (Origin: null) ES module fetches
// succeed: ACAO:* with NO credentials.
func (s *Server) handleVendorServe(w http.ResponseWriter, r *http.Request) {
	data, err := s.deps.WorkspaceRuntime().ReadVendor(strings.TrimSpace(r.PathValue("sha")))
	if err != nil {
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Not found", nil)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Del("Access-Control-Allow-Credentials")
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}

// handleContentServe serves a built Doc Surface page as a single self-contained
// HTML document (tokens + CSS + bundle inlined). When the build externalized
// vendored deps, the doc carries an import map -> /vendor/<sha> and is served as
// an ES module with the CSP script-src widened to the serving origin. Auth is
// enforced by authMiddleware; ownership by Artifacts().Get (scoped to the principal).
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

	// The entry document is the only valid path; vendored deps are served from the
	// separate public /vendor route, not from under /content.
	if rel := r.PathValue("path"); rel != "" && rel != "index.html" {
		writeAPIError(w, r, http.StatusNotFound, categoryNotFound, "Not found", nil)
		return
	}

	// The token may appear in the entry URL (desktop iframe auth); never leak it via Referer.
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	store := s.deps.DocSurfaceStore()
	bundleJS, err := store.ReadFile(r.Context(), pageID, "dist/bundle.js")
	if err != nil {
		w.Header().Set("Content-Security-Policy", docsurface.CSP)
		_, _ = w.Write([]byte(docsurface.NotBuiltHTML(rec.Title)))
		return
	}
	bundleCSS, _ := store.ReadFile(r.Context(), pageID, "dist/bundle.css")

	// Optional import map: present for ESM builds with vendored deps. Its presence
	// switches the doc to <script type=module>; its absence is the legacy inline path.
	var im docsurface.ImportMap
	if data, derr := store.ReadFile(r.Context(), pageID, "dist/importmap.json"); derr == nil {
		_ = json.Unmarshal(data, &im)
		if im.Imports == nil {
			im.Imports = map[string]string{}
		}
	}

	csp := docsurface.CSP
	if len(im.Imports) > 0 {
		if r.URL.Query().Get("client") == "tauri" {
			// Desktop app: deps load from a LOCAL cache via the custom vendor://
			// scheme (zero network after first fetch). Rewrite /vendor/<sha> ->
			// vendor://deps/<sha> and allow the scheme in script-src.
			im = rewriteToVendorScheme(im)
			csp = docsurface.CSPWithVendorScheme()
		} else if origin, oerr := derivePublicOrigin(r); oerr == nil {
			// Web: deps load from /vendor on this serving origin; the opaque-origin
			// doc needs that origin in script-src ('self' matches nothing for it).
			csp = docsurface.CSPWithVendor(origin)
		}
	}
	w.Header().Set("Content-Security-Policy", csp)
	_, _ = w.Write([]byte(docsurface.EntryHTML(rec.Title, docsurface.TokensCSS, string(bundleCSS), string(bundleJS), im)))
}

// derivePublicOrigin computes the origin the doc (and thus /vendor) is served from,
// for the CSP script-src. It is STRICTLY reconstructed from parsed parts so a
// hostile header cannot inject into the CSP. Order: env override -> X-Forwarded
// -> scheme://Host. Desktop/WKWebView (primary target) has a reliable r.Host.
func derivePublicOrigin(r *http.Request) (string, error) {
	if env := strings.TrimSpace(os.Getenv("DOC_SURFACE_ORIGIN")); env != "" {
		return reconstructOrigin(env)
	}
	if proto, host := r.Header.Get("X-Forwarded-Proto"), r.Header.Get("X-Forwarded-Host"); proto != "" && host != "" {
		return reconstructOrigin(proto + "://" + host)
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return reconstructOrigin(scheme + "://" + r.Host)
}

func reconstructOrigin(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("invalid scheme %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" || !validHostname(host) {
		return "", fmt.Errorf("invalid host %q", host)
	}
	origin := u.Scheme + "://" + host
	if port := u.Port(); port != "" { // url.Port() is digits-only per RFC parsing
		origin += ":" + port
	}
	return origin, nil
}

// rewriteToVendorScheme maps each /vendor/<sha> import to vendor://deps/<sha> for
// the desktop app's local-cache scheme. The /vendor route + build are unchanged —
// only the address form differs per client.
func rewriteToVendorScheme(im docsurface.ImportMap) docsurface.ImportMap {
	out := docsurface.ImportMap{Imports: make(map[string]string, len(im.Imports))}
	for spec, u := range im.Imports {
		out.Imports[spec] = "vendor://deps/" + strings.TrimPrefix(u, "/vendor/")
	}
	return out
}

func validHostname(h string) bool {
	for _, c := range h {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '-') {
			return false
		}
	}
	return true
}
