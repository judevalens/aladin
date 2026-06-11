package docsurface

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aladin/backend_v2/internal/service"

	esbuild "github.com/evanw/esbuild/pkg/api"
)

// buildMetaName is the marker a successful build drops into dist/. publish_app
// gates on it (BuildMetaPath) so a page can't be published without a real,
// completed build — a stale or partial bundle.js alone is not enough.
const buildMetaName = ".build-meta.json"

// BuildMetaPath is the page-relative path of the build-success marker.
const BuildMetaPath = distDirName + "/" + buildMetaName

// buildMeta records that a build completed; written atomically via the store.
type buildMeta struct {
	PageID      string `json:"page_id"`
	BuiltAt     string `json:"built_at"`
	BundleBytes int64  `json:"bundle_bytes"`
}

// allowedCDNHost is the only host the build is allowed to fetch from. This is
// the SSRF guard: install_lib URLs and bare/URL imports in agent-authored code
// can only resolve to esm.sh — never internal IPs, localhost, or file://.
const allowedCDNHost = "esm.sh"

// Default versions for the React runtime when not pinned via install_lib. esm.sh
// serves these as ESM; the build-time plugin fetches + caches + inlines them, so
// the rendered page needs NO network (CSP connect-src 'none' holds).
var defaultVersions = map[string]string{
	"react":     "18.3.1",
	"react-dom": "18.3.1",
}

const esmHost = "https://esm.sh/"

// builder is the v1 in-process WorkspaceRuntime: esbuild bundles a page's
// index.tsx, resolving bare imports to esm.sh at build time (cached to a shared
// content-addressed store) and inlining everything into dist/bundle.js.
type builder struct {
	store    service.DocSurfaceStore
	cacheDir string
	client   *http.Client
	// builds serializes concurrent Build() calls per pageID so two agents
	// can't race on the same dist/bundle.js and corrupt it.
	builds sync.Map // pageID -> *sync.Mutex
}

func (b *builder) lockPage(pageID string) func() {
	mu, _ := b.builds.LoadOrStore(pageID, &sync.Mutex{})
	m := mu.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}

// NewBuilder returns a WorkspaceRuntime. cacheDir is the shared content-addressed
// cache for fetched CDN modules (one copy per url across all users/pages).
func NewBuilder(store service.DocSurfaceStore, cacheDir string) service.WorkspaceRuntime {
	return &builder{store: store, cacheDir: cacheDir, client: &http.Client{Timeout: 30 * time.Second}}
}

func (b *builder) Build(ctx context.Context, pageID string) (service.BuildResult, error) {
	defer b.lockPage(pageID)() // serialize builds per page (no concurrent dist writes)
	dir, err := b.store.PageDir(ctx, pageID)
	if err != nil {
		return service.BuildResult{}, err
	}
	if _, err := os.Stat(filepath.Join(dir, "index.tsx")); err != nil {
		return service.BuildResult{OK: false, Log: "index.tsx not found at page root"}, nil
	}
	libs, err := b.store.Libs(ctx, pageID)
	if err != nil {
		return service.BuildResult{}, err
	}
	pins := map[string]string{}
	for _, l := range libs {
		pins[l.Name] = l.URL
	}

	result := esbuild.Build(esbuild.BuildOptions{
		AbsWorkingDir:    dir,
		EntryPoints:      []string{"index.tsx"},
		Outfile:          filepath.Join(dir, distDirName, "bundle.js"),
		Bundle:           true,
		Write:            true,
		Format:           esbuild.FormatIIFE,
		Platform:         esbuild.PlatformBrowser,
		Target:           esbuild.ES2020,
		JSX:              esbuild.JSXAutomatic,
		MinifyWhitespace: true,
		MinifySyntax:     true,
		Loader: map[string]esbuild.Loader{
			".tsx":  esbuild.LoaderTSX,
			".ts":   esbuild.LoaderTS,
			".jsx":  esbuild.LoaderJSX,
			".js":   esbuild.LoaderJS,
			".css":  esbuild.LoaderCSS,
			".json": esbuild.LoaderJSON,
			".svg":  esbuild.LoaderDataURL,
			".png":  esbuild.LoaderDataURL,
		},
		Plugins:  []esbuild.Plugin{b.cdnPlugin(ctx, pins)},
		LogLevel: esbuild.LogLevelSilent,
	})

	if len(result.Errors) > 0 {
		return service.BuildResult{OK: false, Log: formatMessages(result.Errors)}, nil
	}
	b.writeBuildMeta(ctx, pageID, dir)
	return service.BuildResult{
		OK:        true,
		ServedURL: "/content/" + pageID + "/",
		Log:       formatMessages(result.Warnings),
	}, nil
}

// writeBuildMeta drops the build-success marker. Best-effort: a marker write
// failure logs but does not fail an otherwise-good build.
func (b *builder) writeBuildMeta(ctx context.Context, pageID, dir string) {
	var size int64
	if fi, err := os.Stat(filepath.Join(dir, distDirName, "bundle.js")); err == nil {
		size = fi.Size()
	}
	data, err := json.Marshal(buildMeta{
		PageID:      pageID,
		BuiltAt:     time.Now().UTC().Format(time.RFC3339),
		BundleBytes: size,
	})
	if err != nil {
		slog.Warn("docsurface: marshal build meta", "page_id", pageID, "err", err)
		return
	}
	if err := b.store.WriteFile(ctx, pageID, BuildMetaPath, data); err != nil {
		slog.Warn("docsurface: write build meta", "page_id", pageID, "err", err)
	}
}

// cdnPlugin resolves bare specifiers (react, react-dom/client, react/jsx-runtime,
// or any install_lib dep) to esm.sh, fetches them at build time (cached), and
// inlines them. Imports inside fetched modules resolve against the importer URL.
func (b *builder) cdnPlugin(ctx context.Context, pins map[string]string) esbuild.Plugin {
	const ns = "cdn"
	return esbuild.Plugin{
		Name: "esm-sh-cdn",
		Setup: func(build esbuild.PluginBuild) {
			build.OnResolve(esbuild.OnResolveOptions{Filter: ".*"}, func(args esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
				// Full URL import (from agent code or within a fetched module).
				if strings.HasPrefix(args.Path, "http://") || strings.HasPrefix(args.Path, "https://") {
					return esbuild.OnResolveResult{Path: args.Path, Namespace: ns}, nil
				}
				// Import from within a fetched module: resolve against importer URL.
				if args.Namespace == ns {
					base, err := url.Parse(args.Importer)
					if err != nil {
						return esbuild.OnResolveResult{}, err
					}
					ref, err := url.Parse(args.Path)
					if err != nil {
						return esbuild.OnResolveResult{}, err
					}
					return esbuild.OnResolveResult{Path: base.ResolveReference(ref).String(), Namespace: ns}, nil
				}
				// Entry point and local relative/absolute imports -> default fs resolution.
				if args.Kind == esbuild.ResolveEntryPoint || strings.HasPrefix(args.Path, ".") || strings.HasPrefix(args.Path, "/") {
					return esbuild.OnResolveResult{}, nil
				}
				// Bare specifier from local code -> esm.sh (version-pinned).
				if !validBareSpec(args.Path) {
					return esbuild.OnResolveResult{}, fmt.Errorf("invalid import specifier %q", args.Path)
				}
				return esbuild.OnResolveResult{Path: esmURL(args.Path, pins), Namespace: ns}, nil
			})
			build.OnLoad(esbuild.OnLoadOptions{Filter: ".*", Namespace: ns}, func(args esbuild.OnLoadArgs) (esbuild.OnLoadResult, error) {
				contents, err := b.fetchCached(ctx, args.Path)
				if err != nil {
					return esbuild.OnLoadResult{}, err
				}
				loader := esbuild.LoaderJS
				if strings.HasSuffix(pathNoQuery(args.Path), ".css") {
					loader = esbuild.LoaderCSS
				}
				s := string(contents)
				return esbuild.OnLoadResult{Contents: &s, Loader: loader}, nil
			})
		},
	}
}

// esmURL maps a bare specifier to a version-pinned esm.sh URL. install_lib pins
// win; otherwise defaultVersions; otherwise the bare name (esm.sh -> latest).
func esmURL(spec string, pins map[string]string) string {
	if u, ok := pins[spec]; ok {
		return u
	}
	// Split "pkg/subpath" -> pin the package, keep the subpath.
	pkg, sub := splitPackage(spec)
	if u, ok := pins[pkg]; ok {
		return strings.TrimSuffix(u, "/") + sub
	}
	if v, ok := defaultVersions[pkg]; ok {
		return esmHost + pkg + "@" + v + sub
	}
	return esmHost + spec
}

// splitPackage splits "react-dom/client" -> ("react-dom", "/client") and
// "@scope/pkg/sub" -> ("@scope/pkg", "/sub").
func splitPackage(spec string) (pkg, sub string) {
	if strings.HasPrefix(spec, "@") {
		parts := strings.SplitN(spec, "/", 3)
		if len(parts) >= 2 {
			pkg = parts[0] + "/" + parts[1]
			if len(parts) == 3 {
				sub = "/" + parts[2]
			}
			return pkg, sub
		}
		return spec, ""
	}
	parts := strings.SplitN(spec, "/", 2)
	pkg = parts[0]
	if len(parts) == 2 {
		sub = "/" + parts[1]
	}
	return pkg, sub
}

// fetchCached returns the module bytes for url, reading from / writing to the
// shared content-addressed cache so repeated builds are offline after first
// fetch. It rejects any non-esm.sh host (the SSRF guard) and honors ctx
// cancellation so a slow CDN can't pin a build goroutine open.
func (b *builder) fetchCached(ctx context.Context, rawURL string) ([]byte, error) {
	if err := requireAllowedCDN(rawURL); err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(rawURL))
	cachePath := filepath.Join(b.cacheDir, hex.EncodeToString(sum[:]))
	if data, err := os.ReadFile(cachePath); err == nil {
		return data, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", rawURL, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFileBytes))
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(b.cacheDir, 0o755); err == nil {
		_ = atomicWrite(cachePath, data)
	}
	return data, nil
}

// requireAllowedCDN rejects any URL whose host is not the allowed CDN. https
// only. This is the build-time SSRF guard.
func requireAllowedCDN(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url %q: %w", rawURL, err)
	}
	if u.Scheme != "https" || u.Hostname() != allowedCDNHost {
		return fmt.Errorf("disallowed dependency host %q (only https://%s is permitted)", u.Host, allowedCDNHost)
	}
	return nil
}

// validBareSpec allows only sane npm specifiers (pkg, @scope/pkg, sub-paths,
// versions) so a hostile import can't smuggle traversal/garbage into the CDN URL.
func validBareSpec(spec string) bool {
	if spec == "" || strings.Contains(spec, "..") {
		return false
	}
	for _, r := range spec {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '@' || r == '/' || r == '-' || r == '_' || r == '.' || r == '+':
		default:
			return false
		}
	}
	return true
}

func pathNoQuery(p string) string {
	if i := strings.IndexByte(p, '?'); i >= 0 {
		return p[:i]
	}
	return p
}

// formatMessages renders esbuild diagnostics into a readable log for the agent.
func formatMessages(msgs []esbuild.Message) string {
	if len(msgs) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, m := range msgs {
		if m.Location != nil {
			fmt.Fprintf(&sb, "%s:%d:%d: %s\n", m.Location.File, m.Location.Line, m.Location.Column, m.Text)
			if m.Location.LineText != "" {
				fmt.Fprintf(&sb, "    %s\n", strings.TrimSpace(m.Location.LineText))
			}
		} else {
			fmt.Fprintf(&sb, "%s\n", m.Text)
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}
