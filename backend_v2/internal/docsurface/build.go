package docsurface

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	"aladin/backend_v2/internal/shardv2"

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
	BuildID     string `json:"build_id"`
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
	vendor   *vendorStore
	profiles shardv2.Registry
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
func NewBuilder(store service.DocSurfaceStore, cacheDir string, profiles ...shardv2.Registry) service.WorkspaceRuntime {
	b := &builder{store: store, cacheDir: cacheDir, client: &http.Client{Timeout: 30 * time.Second}}
	if len(profiles) > 0 {
		b.profiles = profiles[0]
	}
	// Vendored libs live next to the esm cache (e.g. <data>/cache/vendor), shared
	// across all users/pages and across the api + mcp processes.
	b.vendor = newVendorStore(b, filepath.Join(filepath.Dir(cacheDir), vendorDirName))
	return b
}

// ReadVendor serves a content-addressed vendored dependency file (GET /vendor/<sha>).
func (b *builder) ReadVendor(sha string) ([]byte, error) {
	return b.vendor.ReadVendor(sha)
}

// DistDir is the page-relative dist directory for a build channel: published →
// "dist", draft → "dist/draft". All of a build's outputs (bundle.js, bundle.css,
// importmap.json, the build marker) live under it. Exported so the serve route
// can locate the right channel's artifacts.
func DistDir(channel service.BuildChannel) string {
	if channel == service.ChannelDraft {
		return distDirName + "/draft"
	}
	return distDirName
}

// ParseChannel maps a request's ?channel value to a BuildChannel, defaulting to
// published for anything other than "draft".
func ParseChannel(s string) service.BuildChannel {
	if s == string(service.ChannelDraft) {
		return service.ChannelDraft
	}
	return service.ChannelPublished
}

func (b *builder) Build(ctx context.Context, pageID string, channel service.BuildChannel) (service.BuildResult, error) {
	defer b.lockPage(pageID)() // serialize builds per page (no concurrent dist writes)
	dir, err := b.store.PageDir(ctx, pageID)
	if err != nil {
		return service.BuildResult{}, err
	}
	if _, err := os.Stat(filepath.Join(dir, "index.tsx")); err != nil {
		return service.BuildResult{OK: false, Log: "index.tsx not found at page root"}, nil
	}
	var contract []byte
	var anchors []byte
	if data, readErr := b.store.ReadFile(ctx, pageID, "contract.json"); readErr == nil {
		if b.profiles == nil {
			return service.BuildResult{Log: "Shard v2 is disabled on this server"}, nil
		}
		compiled, err := CompileContractV2(data, b.profiles)
		if err != nil {
			return service.BuildResult{Log: "contract.json: " + err.Error()}, nil
		}
		anchors, err = b.store.ReadFile(ctx, pageID, ManifestFileName)
		if err != nil {
			return service.BuildResult{Log: "V2 requires anchors.json"}, nil
		}
		var manifest Manifest
		if err := json.Unmarshal(anchors, &manifest); err != nil {
			return service.BuildResult{Log: "anchors.json: " + err.Error()}, nil
		}
		if problems := ValidateManifestBindingsV2(manifest, compiled); len(problems) > 0 {
			return service.BuildResult{Log: strings.Join(problems, "\n")}, nil
		}
		contract = data
	} else if !os.IsNotExist(readErr) && !errors.Is(readErr, service.ErrNotFound) {
		return service.BuildResult{}, readErr
	}
	// Anchor manifest: schema-validate when present (hard, both channels). Absent
	// is fine in v1 — the manifest becomes required only at the publish gate.
	if data, merr := b.store.ReadFile(ctx, pageID, ManifestFileName); merr == nil {
		if problems := ValidateManifestBytes(data); len(problems) > 0 {
			return service.BuildResult{OK: false, Log: ManifestFileName + ":\n  " + strings.Join(problems, "\n  ")}, nil
		}
	}
	libs, err := b.store.Libs(ctx, pageID)
	if err != nil {
		return service.BuildResult{}, err
	}
	pins := map[string]string{}
	for _, l := range libs {
		pins[l.Name] = l.URL
	}

	distRel := DistDir(channel)
	draft := channel == service.ChannelDraft
	opts := esbuild.BuildOptions{
		AbsWorkingDir: dir,
		EntryPoints:   []string{"index.tsx"},
		Outfile:       filepath.Join(dir, distRel, "bundle.js"),
		Bundle:        true,
		Write:         len(contract) == 0,
		Metafile:      true, // discover which deps were left external (vendored)
		Format:        esbuild.FormatESModule,
		Platform:      esbuild.PlatformBrowser,
		Target:        esbuild.ES2020,
		JSX:           esbuild.JSXAutomatic,
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
	}
	if draft {
		// Fast authoring loop: keep the source readable + map back to the agent's
		// files for debugging. No minify.
		opts.Sourcemap = esbuild.SourceMapInline
	} else {
		opts.MinifyWhitespace = true
		opts.MinifySyntax = true
	}
	result := esbuild.Build(opts)

	if len(result.Errors) > 0 {
		return service.BuildResult{OK: false, Log: formatMessages(result.Errors)}, nil
	}
	// Resolve the externalized (vendored) deps into an import map persisted next to
	// the bundle. A vendoring failure is a soft build failure so the agent iterates.
	im, vlog, ok := b.resolveImportMap(ctx, result.Metafile, pins)
	if !ok {
		return service.BuildResult{OK: false, Log: vlog}, nil
	}
	if len(contract) > 0 {
		files := map[string][]byte{"anchors.json": anchors}
		for _, file := range result.OutputFiles {
			files[filepath.Base(file.Path)] = file.Contents
		}
		files["importmap.json"], err = json.Marshal(im)
		if err != nil {
			return service.BuildResult{}, err
		}
		served := "/content/" + pageID + "/"
		if draft {
			served += "?channel=draft"
		}
		return service.BuildResult{OK: true, ServedURL: served, Log: formatMessages(result.Warnings), BuildID: service.ShardBuildIdentity(contract, files), Contract: contract, Files: files}, nil
	}
	if err := b.writeImportMap(ctx, pageID, distRel, im); err != nil {
		return service.BuildResult{}, err
	}
	buildID := b.writeBuildMeta(ctx, pageID, dir, distRel)
	served := "/content/" + pageID + "/"
	if draft {
		served += "?channel=draft"
	}
	return service.BuildResult{
		OK:        true,
		ServedURL: served,
		Log:       formatMessages(result.Warnings),
		BuildID:   buildID,
	}, nil
}

// buildID is a content hash of the build outputs (bundle.js + bundle.css). It
// changes iff the served bytes change, so it cleanly identifies a build.
func buildIDFor(dir, distRel string) string {
	h := sha256.New()
	for _, name := range []string{"bundle.js", "bundle.css"} {
		if data, err := os.ReadFile(filepath.Join(dir, distRel, name)); err == nil {
			h.Write(data)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// importMapPath is the page-relative path of the persisted import map.
const importMapPath = distDirName + "/importmap.json"

// resolveImportMap builds + caches a vendored file for each externalized specifier
// the bundle imports (discovered from the esbuild metafile) and returns the import
// map. Returns ok=false with a readable log on a vendoring failure.
func (b *builder) resolveImportMap(ctx context.Context, metafile string, pins map[string]string) (ImportMap, string, bool) {
	im := ImportMap{Imports: map[string]string{}}
	var failLog string
	ensure := func(spec string) bool {
		if _, has := im.Imports[spec]; has {
			return true
		}
		ref, err := b.vendor.Resolve(ctx, spec, pins)
		if err != nil {
			failLog = "vendoring " + spec + ": " + err.Error()
			return false
		}
		im.Imports[spec] = "/" + vendorDirName + "/" + ref.Sha
		return true
	}

	needReact := false
	usesKit := false
	for _, spec := range parseExternalImports(metafile) {
		if !isVendored(spec) {
			continue // a stray external (shouldn't happen) — leave it unmapped
		}
		if !ensure(spec) {
			return ImportMap{}, failLog, false
		}
		if reactFamily[spec] && spec != "react" {
			needReact = true // its bare `import "react"` needs the shared react mapped
		}
		if spec == kitSpec {
			usesKit = true
		}
	}
	// The kit imports react + react/jsx-runtime at runtime; map them even if the
	// shard's own code didn't pull them in directly.
	if usesKit {
		needReact = true
		if !ensure("react/jsx-runtime") {
			return ImportMap{}, failLog, false
		}
	}
	if needReact && !ensure("react") {
		return ImportMap{}, failLog, false
	}
	return im, "", true
}

func (b *builder) writeImportMap(ctx context.Context, pageID, distRel string, im ImportMap) error {
	data, err := json.Marshal(im)
	if err != nil {
		return err
	}
	return b.store.WriteFile(ctx, pageID, distRel+"/importmap.json", data)
}

// parseExternalImports returns the unique specifiers the bundle imports that
// esbuild left external (i.e. the vendored deps), from the build metafile.
func parseExternalImports(metafile string) []string {
	var m struct {
		Outputs map[string]struct {
			Imports []struct {
				Path     string `json:"path"`
				External bool   `json:"external"`
			} `json:"imports"`
		} `json:"outputs"`
	}
	if json.Unmarshal([]byte(metafile), &m) != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, o := range m.Outputs {
		for _, imp := range o.Imports {
			if imp.External && !seen[imp.Path] {
				seen[imp.Path] = true
				out = append(out, imp.Path)
			}
		}
	}
	return out
}

// writeBuildMeta drops the build-success marker into the channel's dist dir.
// Best-effort: a marker write failure logs but does not fail an otherwise-good
// build. publish_app gates on the PUBLISHED marker (BuildMetaPath = dist/...).
// writeBuildMeta computes the build id, writes the marker, and returns the id.
func (b *builder) writeBuildMeta(ctx context.Context, pageID, dir, distRel string) string {
	var size int64
	if fi, err := os.Stat(filepath.Join(dir, distRel, "bundle.js")); err == nil {
		size = fi.Size()
	}
	buildID := buildIDFor(dir, distRel)
	data, err := json.Marshal(buildMeta{
		PageID:      pageID,
		BuiltAt:     time.Now().UTC().Format(time.RFC3339),
		BundleBytes: size,
		BuildID:     buildID,
	})
	if err != nil {
		slog.Warn("docsurface: marshal build meta", "page_id", pageID, "err", err)
		return buildID
	}
	if err := b.store.WriteFile(ctx, pageID, distRel+"/"+buildMetaName, data); err != nil {
		slog.Warn("docsurface: write build meta", "page_id", pageID, "err", err)
	}
	return buildID
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
				// Vendored deps (react family + heavy libs) load from /vendor at
				// runtime via the import map — keep them external, don't inline.
				if isVendored(args.Path) {
					return esbuild.OnResolveResult{Path: args.Path, External: true}, nil
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
