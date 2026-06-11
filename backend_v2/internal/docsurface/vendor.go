package docsurface

// Vendored dependencies: a curated allowlist of libraries is loaded from a shared
// /vendor route at runtime (via an import map) instead of being inlined into every
// doc's bundle. Each vendored lib is bundled ONCE into a single self-contained ESM
// file (following the esm.sh graph, like the doc build), content-addressed, and
// reused across all docs. The shared react is kept external in its consumers (via
// esm.sh's ?external= + esbuild External) so there is exactly ONE react instance.

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	esbuild "github.com/evanw/esbuild/pkg/api"
)

const (
	vendorDirName = "vendor"
	// maxVendorBytes caps a single shared vendored file. It is higher than the
	// per-user 10MiB file cap because a vendored lib is public, content-addressed,
	// and fetched once for everyone (mermaid alone is ~18MB).
	maxVendorBytes = 64 << 20
)

// reactFamily specifiers are ALWAYS vendored (loaded from /vendor, never inlined).
var reactFamily = map[string]bool{
	"react":                 true,
	"react-dom":             true,
	"react-dom/client":      true,
	"react-dom/server":      true,
	"react/jsx-runtime":     true,
	"react/jsx-dev-runtime": true,
}

// heavyLibs (by package name) are vendored when an agent imports them — large
// libraries that shouldn't be re-inlined into every doc.
var heavyLibs = map[string]bool{
	"mermaid":  true,
	"d3":       true,
	"recharts": true,
	"three":    true,
}

// isVendored reports whether a bare import specifier should be loaded from /vendor
// (externalized) rather than inlined into the doc bundle.
func isVendored(spec string) bool {
	if reactFamily[spec] {
		return true
	}
	pkg, _ := splitPackage(spec)
	return heavyLibs[pkg]
}

// vendorRef is the result of resolving a vendored specifier: a content-addressed
// /vendor path component + SRI integrity for the import map.
type vendorRef struct {
	Sha       string `json:"sha"`
	Integrity string `json:"integrity"`
	Bytes     int    `json:"bytes"`
}

// vendorStore builds each vendored library into a single self-contained ESM file
// and content-addresses it under dir, reusing across builds (in-process + on-disk).
type vendorStore struct {
	b   *builder
	dir string

	mu  sync.Mutex
	mem map[string]vendorRef // resolveKey -> ref
}

func newVendorStore(b *builder, dir string) *vendorStore {
	return &vendorStore{b: b, dir: dir, mem: map[string]vendorRef{}}
}

// resolveKey is the stable cache key for a vendored spec at a resolved version.
func resolveKey(spec string, pins map[string]string) string {
	return spec + "\x00" + esmURL(spec, pins)
}

// Resolve returns the /vendor ref for spec, building + caching it if needed.
func (vs *vendorStore) Resolve(ctx context.Context, spec string, pins map[string]string) (vendorRef, error) {
	key := resolveKey(spec, pins)

	vs.mu.Lock()
	if ref, ok := vs.mem[key]; ok {
		vs.mu.Unlock()
		return ref, nil
	}
	vs.mu.Unlock()

	// On-disk ref cache (survives restarts): refs/<sha256(key)> -> vendorRef json.
	refPath := filepath.Join(vs.dir, "refs", sha256Hex([]byte(key)))
	if data, err := os.ReadFile(refPath); err == nil {
		var ref vendorRef
		if json.Unmarshal(data, &ref) == nil && ref.Sha != "" {
			if _, err := os.Stat(filepath.Join(vs.dir, ref.Sha)); err == nil {
				vs.remember(key, ref)
				return ref, nil
			}
		}
	}

	external := map[string]bool{}
	if spec != "react" {
		external["react"] = true // share the single react across the family + consumers
	}
	body, err := vs.build(ctx, spec, external, pins)
	if err != nil {
		return vendorRef{}, err
	}
	if len(body) > maxVendorBytes {
		return vendorRef{}, fmt.Errorf("vendored %q is %d bytes, exceeds the %d byte cap", spec, len(body), maxVendorBytes)
	}
	sha := sha256Hex(body)
	if err := os.MkdirAll(vs.dir, 0o755); err != nil {
		return vendorRef{}, err
	}
	if err := atomicWrite(filepath.Join(vs.dir, sha), body); err != nil {
		return vendorRef{}, err
	}
	sum384 := sha512.Sum384(body)
	ref := vendorRef{Sha: sha, Integrity: "sha384-" + base64.StdEncoding.EncodeToString(sum384[:]), Bytes: len(body)}
	if err := os.MkdirAll(filepath.Dir(refPath), 0o755); err == nil {
		if data, err := json.Marshal(ref); err == nil {
			_ = atomicWrite(refPath, data)
		}
	}
	vs.remember(key, ref)
	return ref, nil
}

func (vs *vendorStore) remember(key string, ref vendorRef) {
	vs.mu.Lock()
	vs.mem[key] = ref
	vs.mu.Unlock()
}

// ReadVendor returns the bytes of a content-addressed vendored file (for serving
// GET /vendor/<sha>). sha must be a bare hex string — no path separators.
func (vs *vendorStore) ReadVendor(sha string) ([]byte, error) {
	if !isHex(sha) {
		return nil, os.ErrNotExist
	}
	return os.ReadFile(filepath.Join(vs.dir, sha))
}

// build bundles spec into a single ESM file via the esm.sh graph, leaving the
// `external` specifiers (react) as bare imports the doc import map resolves.
func (vs *vendorStore) build(ctx context.Context, spec string, external map[string]bool, pins map[string]string) ([]byte, error) {
	stub := `import * as __m from ` + jsString(spec) + `;
export * from ` + jsString(spec) + `;
export default __m.default;`
	res := esbuild.Build(esbuild.BuildOptions{
		Stdin:            &esbuild.StdinOptions{Contents: stub, Loader: esbuild.LoaderJS, Sourcefile: "vendor-entry.js"},
		Bundle:           true,
		Write:            false,
		Format:           esbuild.FormatESModule,
		Platform:         esbuild.PlatformBrowser,
		Target:           esbuild.ES2020,
		MinifyWhitespace: true,
		MinifySyntax:     true,
		Plugins:          []esbuild.Plugin{vs.b.vendorPlugin(ctx, external, pins)},
		LogLevel:         esbuild.LogLevelSilent,
	})
	if len(res.Errors) > 0 {
		return nil, fmt.Errorf("vendor build %q: %s", spec, formatMessages(res.Errors))
	}
	return res.OutputFiles[0].Contents, nil
}

// vendorPlugin resolves the esm.sh graph for a vendored lib, marking `external`
// specifiers external (and telling esm.sh to externalize them via ?external=, so
// it emits a bare `import "react"` instead of inlining a second react).
func (b *builder) vendorPlugin(ctx context.Context, external map[string]bool, pins map[string]string) esbuild.Plugin {
	const ns = "cdn"
	return esbuild.Plugin{
		Name: "esm-sh-vendor",
		Setup: func(build esbuild.PluginBuild) {
			build.OnResolve(esbuild.OnResolveOptions{Filter: ".*"}, func(a esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
				if external[a.Path] {
					return esbuild.OnResolveResult{Path: a.Path, External: true}, nil
				}
				if strings.HasPrefix(a.Path, "http://") || strings.HasPrefix(a.Path, "https://") {
					return esbuild.OnResolveResult{Path: a.Path, Namespace: ns}, nil
				}
				if a.Namespace == ns {
					base, err := url.Parse(a.Importer)
					if err != nil {
						return esbuild.OnResolveResult{}, err
					}
					ref, err := url.Parse(a.Path)
					if err != nil {
						return esbuild.OnResolveResult{}, err
					}
					return esbuild.OnResolveResult{Path: base.ResolveReference(ref).String(), Namespace: ns}, nil
				}
				if a.Kind == esbuild.ResolveEntryPoint {
					return esbuild.OnResolveResult{}, nil
				}
				if !validBareSpec(a.Path) {
					return esbuild.OnResolveResult{}, fmt.Errorf("invalid import specifier %q", a.Path)
				}
				u := esmURL(a.Path, pins)
				if len(external) > 0 {
					keys := make([]string, 0, len(external))
					for k := range external {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					u += "?external=" + strings.Join(keys, ",")
				}
				return esbuild.OnResolveResult{Path: u, Namespace: ns}, nil
			})
			build.OnLoad(esbuild.OnLoadOptions{Filter: ".*", Namespace: ns}, func(a esbuild.OnLoadArgs) (esbuild.OnLoadResult, error) {
				data, err := b.fetchCached(ctx, a.Path)
				if err != nil {
					return esbuild.OnLoadResult{}, err
				}
				s := string(data)
				loader := esbuild.LoaderJS
				if strings.HasSuffix(pathNoQuery(a.Path), ".css") {
					loader = esbuild.LoaderCSS
				}
				return esbuild.OnLoadResult{Contents: &s, Loader: loader}, nil
			})
		},
	}
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func isHex(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}
