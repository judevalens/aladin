package service

import (
	"context"

	"aladin/backend_v2/internal/shardresource"
)

// Doc Surface ("app" artifacts) — agent-authored multi-file React pages built
// server-side with esbuild and rendered in a sandboxed iframe. Source lives as
// files on a data volume (users/{userId}/pages/{pageId}/...), NOT in Postgres;
// Postgres holds only the artifact row + the MD summary (the KG spine).
//
// These interfaces are declared here (the service layer) so process composition can
// expose them; the concrete impls live in internal/docsurface.

// FileEntry is one item in a directory listing.
type FileEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size,omitempty"`
}

// LibEntry is one installed dependency (resolved to an esm.sh URL at build time
// and emitted as an external ESM import the browser loads at runtime).
type LibEntry struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// BuildResult is what build_app returns to the agent. A failed build is OK=false
// with a non-empty Log (compiler errors) — NOT a Go error — so the agent can
// read the errors and iterate.
type BuildResult struct {
	OK        bool   `json:"ok"`
	ServedURL string `json:"served_url,omitempty"`
	Log       string `json:"log,omitempty"`
	// BuildID is a content hash of this build's output (bundle.js + css). It
	// identifies exactly which build produced the served artifact — recorded in
	// the build-status event and (later) embedded for the inspect runtime.
	BuildID string `json:"build_id,omitempty"`
	// V2 outputs are staged by the build service, never written to mutable dist.
	Contract []byte            `json:"-"`
	Files    map[string][]byte `json:"-"`
}

// BuildChannel selects which artifact a build produces. The two channels coexist
// per page: draft is the fast authoring loop (unminified, inline source maps,
// auto-built on every write) written to dist/draft/; published is the promoted
// artifact publish_app gates on, written to dist/ (minified). They differ only
// in minification, source maps, and output dir.
type BuildChannel = shardresource.Environment

const (
	ChannelPublished = shardresource.EnvironmentPublished
	ChannelDraft     = shardresource.EnvironmentDraft
)

// DocSurfaceStore is per-user, per-page file IO over the data volume. The userID
// is ALWAYS derived from the ctx Principal inside the impl — never passed by a
// caller — and combined with pageID through a traversal-safe path join.
type DocSurfaceStore interface {
	// EnsurePageDir creates users/{userId}/pages/{pageId}/ and returns its abs path.
	EnsurePageDir(ctx context.Context, pageID string) (string, error)
	// PageDir returns the abs path for (userID-from-ctx, pageID) without creating it.
	PageDir(ctx context.Context, pageID string) (string, error)
	ListDir(ctx context.Context, pageID, relPath string) ([]FileEntry, error)
	ReadFile(ctx context.Context, pageID, relPath string) ([]byte, error)
	WriteFile(ctx context.Context, pageID, relPath string, data []byte) error
	DeleteFile(ctx context.Context, pageID, relPath string) error
	SearchFiles(ctx context.Context, pageID, query string, limit int) ([]string, error)
	// InstallLib appends name->url to install_lib.json (dedupe by name) and
	// returns the full lib list.
	InstallLib(ctx context.Context, pageID string, lib LibEntry) ([]LibEntry, error)
	// Libs reads the current install_lib.json manifest.
	Libs(ctx context.Context, pageID string) ([]LibEntry, error)
}

// WorkspaceRuntime is the build/exec seam. v1 impl runs esbuild in-process
// (in the MCP process); a Docker-per-user impl can swap in later unchanged.
type WorkspaceRuntime interface {
	// Build bundles pageID's index.tsx into the channel's dist dir and returns the
	// result. channel selects draft (dist/draft/, unminified + inline source maps)
	// vs published (dist/, minified).
	Build(ctx context.Context, pageID string, channel BuildChannel) (BuildResult, error)
	// ReadVendor returns the bytes of a content-addressed vendored dependency file
	// (served publicly at GET /vendor/<sha>). sha is a bare hex string; the impl
	// rejects anything else. Vendored libs are shared across all users/pages.
	ReadVendor(sha string) ([]byte, error)
}

// PreviewState is the agent's view of the live headless preview tab after an op.
// One shape covers every preview_* tool (returned as StructuredContent): the
// always-present fields (URL/Title/Mounted/Console/Exceptions) plus the op-
// specific field (Outline for snapshot, EvalResult for eval).
type PreviewState struct {
	PageID     string   `json:"page_id"`
	URL        string   `json:"url"`                   // current location incl. hash, e.g. "about:blank#/section"
	Title      string   `json:"title,omitempty"`       // document.title
	Mounted    bool     `json:"mounted"`               // #root has child nodes (React mounted)
	Outline    string   `json:"outline,omitempty"`     // snapshot op: innerText + compact element outline
	EvalResult string   `json:"eval_result,omitempty"` // eval op: JSON-stringified return value
	Console    []string `json:"console,omitempty"`     // console lines accumulated since open (capped)
	Exceptions []string `json:"exceptions,omitempty"`  // uncaught errors accumulated since open (capped)
}

// PreviewService drives a live headless browser tab per (userID-from-ctx, pageID)
// so an agent can interactively inspect a built Doc Surface page across MCP calls:
// open it, walk its in-app hash routes, snapshot / screenshot / eval / click, and
// read the console — before publishing. It is best-effort: where no Chrome binary
// is available every method returns a BadRequest ("renderer unavailable"), never
// a fatal error, so the rest of the MCP surface keeps working. The tab loads the
// exact same inlined bundle under the exact same opaque-origin CSP as the served
// iframe, so eval/click run against untrusted agent code with zero host reach.
// PreviewOpenOptions tunes how a preview document is composed. Theme, when a
// valid theme name, stamps data-theme on the preview doc so screenshots/checks
// can run in any of the app's themes; empty = default dark.
type PreviewOpenOptions struct {
	Theme string
	// Exact already-built release to verify; prevents a second build racing publish.
	Build *BuildResult
}

type PreviewService interface {
	// Open (re)builds pageID on the given channel then loads the freshest inlined
	// HTML into the tab (reusing the page's existing tab if present, else creating
	// one). A build failure is returned as an error carrying the build log.
	Open(ctx context.Context, pageID string, channel BuildChannel, opts PreviewOpenOptions) (PreviewState, error)
	// Navigate sets the in-app hash route (e.g. "#/section/sub") and waits for the
	// view to settle.
	Navigate(ctx context.Context, pageID, route string) (PreviewState, error)
	// Snapshot fills Outline with the current view's text + a compact DOM outline.
	Snapshot(ctx context.Context, pageID string) (PreviewState, error)
	// Screenshot returns a viewport PNG plus the current state.
	Screenshot(ctx context.Context, pageID string) ([]byte, PreviewState, error)
	// Eval evaluates a JS expression in the page and returns its JSON result.
	Eval(ctx context.Context, pageID, expr string) (PreviewState, error)
	// Click clicks the first element matching a CSS selector.
	Click(ctx context.Context, pageID, selector string) (PreviewState, error)
	// Console returns the console + exception lines accumulated since open.
	Console(ctx context.Context, pageID string) (PreviewState, error)
	// CheckAnchors counts elements carrying each anchor id on the current route
	// (the manifest's structural claim, checked against the live DOM).
	CheckAnchors(ctx context.Context, pageID string, anchorIDs []string) (map[string]int, error)
	// ConsoleErrors returns just the console.error lines since open, so a verify
	// pass can report (or gate on) them separately from the full transcript.
	ConsoleErrors(ctx context.Context, pageID string) ([]string, error)
	// EscapingLinks returns the hrefs on the current route that would navigate the
	// frame off its own authenticated document (anything that is not a fragment
	// link or an explicit scheme). A served shard's credential lives in its URL
	// query, so such a link replaces the shard with an auth error when clicked.
	EscapingLinks(ctx context.Context, pageID string) ([]string, error)
	// Close frees the page's tab (no-op if none open).
	Close(ctx context.Context, pageID string) error
	// Reset force-tears down the shared browser + every tab so the next Open
	// rebuilds a fresh renderer — the manual escape hatch for a wedged browser
	// that auto-recovery hasn't yet noticed.
	Reset(ctx context.Context) error
	// CloseAll tears down every tab + the browser; called on MCP shutdown.
	CloseAll(ctx context.Context) error
}
