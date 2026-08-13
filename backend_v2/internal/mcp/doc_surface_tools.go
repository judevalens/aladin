package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"aladin/backend_v2/internal/docsurface"
	"aladin/backend_v2/internal/service"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// appArtifactType is the artifact type string for Doc Surface pages (distinct
// from "page", which is a BlockNote note).
const appArtifactType = "app"

// starterIndexTSX seeds a new shard composed from @aladin/kit — the default way
// to author: kit components (Region marks addressable surfaces), Tailwind +
// Aladin token utilities for styling, hash routing for multi-view shards.
const starterIndexTSX = `import { createRoot } from "react-dom/client";
import { Page, Section, Region } from "@aladin/kit";

function App() {
  return (
    <Page>
      <Section>
        <Region anchor="intro" kind="narrative">
          <h1 className="text-2xl font-display text-ink">New shard</h1>
          <p className="mt-2 text-ink-2">
            Composed from <span className="font-mono text-amber">@aladin/kit</span>.
            Wrap addressable regions in <span className="font-mono">Region</span>,
            style with Tailwind + token utilities (bg-panel, text-ink, …), and
            declare each region in anchors.json.
          </p>
        </Region>
      </Section>
    </Page>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
`

// starterAnchorsJSON seeds the manifest so the addressable surface is declared
// from the start (the intro Region above). Agents extend this as they add regions.
const starterAnchorsJSON = `{
  "version": 1,
  "intent": "Describe what this shard is for — written so a cold agent could rebuild its idea.",
  "anchors": [
    {
      "id": "intro",
      "kind": "narrative",
      "route": "#/",
      "source": "index.tsx",
      "meaning": "The shard's intro/heading region."
    }
  ]
}
`

// kitAuthoringGuide is the @aladin/kit reference, returned from create_app so
// an agent writes VALID code (the usual failure is guessing components/props it
// doesn't know). Distilled from the kit source — keep it accurate.
const kitAuthoringGuide = `A shard is a REACT app written in TypeScript/TSX. It already has a working, buildable index.tsx (returned as current_index_tsx) — write_file a COMPLETE, valid index.tsx that EXTENDS it: a full module with the React imports at the top, your <App/> component, and the createRoot render at the bottom. Never write a fragment, markdown, or prose into a .tsx file. Import components from "@aladin/kit". Style with Tailwind + Aladin tokens ONLY — never arbitrary hex.

index.tsx MUST be a complete module and end with:
  import { createRoot } from "react-dom/client";
  createRoot(document.getElementById("root")!).render(<App />);

@aladin/kit exports (all token-styled, self-contained):
- Layout: <Page>, <Section> (centered, max-w-3xl), <Panel>, <Card>, <Toolbar>, <Divider/>
- Regions (wrap each addressable part; add a matching entry in anchors.json): <Region anchor="intro" kind="narrative|metric|chart|collection|control">…</Region>
- Routing (hash, for multi-view shards): <Route path="/x">…</Route>, <Link to="/x">…</Link>, useRoute()
- UI: <Button variant="primary|outline|ghost|danger" size="sm|md">, <Badge tone="neutral|amber|for|against">, <Callout tone="info|warn|for|against" title="…">, <Stat label={…} value={…} sub={…}/>, <Tabs tabs={[{id,label,content}]}/>, <Dialog open onClose title>, <Input>, <Textarea>, <Field label hint>
- Semantic colored text: <For>, <Against>, <Catalyst>, <Echo>

Tokens (Tailwind classes): surfaces bg-bg/bg-panel/bg-card/bg-raise/bg-field; ink text-ink/text-ink-2/text-ink-3/text-ink-4; accent text-amber, border-amber-line; lines border-line; radius rounded-card/rounded-chip/rounded-modal; fonts font-display/font-mono/font-sans.

Charts: run install_lib "recharts" first, import from "recharts", theme via the kit: <XAxis {...chartAxis()}/>, <CartesianGrid {...chartGrid()}/>, <Tooltip {...chartTooltip()}/>, and series colors from chartSeries()[i]. (import { chartAxis, chartGrid, chartTooltip, chartSeries } from "@aladin/kit")

Animations: Tailwind (transition-*, hover:*, animate-pulse) or your own CSS keyframes in a .css file you write_file and import.

Loop: after each write_file/edit_file, READ the returned build log — if it has errors, fix the exact file and write again until build.ok is true. Keep components small and valid; prefer kit primitives over hand-rolled markup.`

// docToolServer carries the deps the Doc Surface tools need. The artifact
// service scopes every Get/Create/Update to the caller's principal; the store
// scopes file IO to the same principal. Together they enforce ownership.
type docToolServer struct {
	artifacts service.ArtifactService
	store     service.DocSurfaceStore
	build     service.ShardBuildService
	preview   service.PreviewService
	// bridge audits the manifest's refs at publish (nil in tests that don't
	// exercise the workspace plane).
	bridge service.ShardBridgeService
}

func registerDocSurfaceTools(server *sdkmcp.Server, artifacts service.ArtifactService, store service.DocSurfaceStore, build service.ShardBuildService, preview service.PreviewService, bridge service.ShardBridgeService) {
	t := docToolServer{artifacts: artifacts, store: store, build: build, preview: preview, bridge: bridge}

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "create_app",
		Description: "Create a Doc Surface page (an interactive React app rendered in a sandboxed iframe). Seeds index.tsx. Then write_file to author, build_app to compile, publish_app to make it live. NOTE: distinct from create_page (which makes a BlockNote document).",
	}, t.createApp)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_dir",
		Description: "List files/dirs in a Doc Surface page directory (relative path optional, defaults to the page root).",
	}, t.listDir)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "read_file",
		Description: "Read a file from a Doc Surface page directory.",
	}, t.readFile)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "write_file",
		Description: "Write (create or overwrite) a file in a Doc Surface page directory (index.tsx, components, css). After the write a DRAFT build runs automatically and its diagnostics come back in `build` — read them to confirm the change compiles; the user sees the draft update live. Pass build=false for bulk multi-file writes, then build once at the end.",
	}, t.writeFile)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "edit_file",
		Description: "Edit a file by exact string replacement: old_string must appear EXACTLY once (include surrounding context to disambiguate) unless replace_all=true. Errors if old_string is absent or ambiguous. Like write_file, it triggers a draft build and returns diagnostics in `build`. Prefer this over write_file for surgical changes.",
	}, t.editFile)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "delete_file",
		Description: "Delete a file from a Doc Surface page directory. Like write_file, a draft build runs afterwards and its diagnostics come back in `build`.",
	}, t.deleteFile)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "install_lib",
		Description: "Add an npm dependency to a Doc Surface page. Provide name (optionally name@version); the lib is bundled from esm.sh at build time. import it normally in your code.",
	}, t.installLib)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "build_app",
		Description: "Build a Doc Surface page (bundles index.tsx with esbuild). Returns ok + a build log; on failure, read the log, fix the files, and build again.",
	}, t.buildApp)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "verify_app",
		Description: "Verify a shard WITHOUT publishing: validates anchors.json, checks every declared ref resolves, then drives each declared route through the live preview and reports per route whether it mounted, which declared anchors are actually in the DOM, uncaught exceptions (unhandled promise rejections included), and console errors. Defaults to the draft channel; pass strict_console=true to treat console errors as failures. Run this before publish_app — publish applies the same checks, but this shows you the whole report while you can still fix it.",
	}, t.verifyAppTool)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "publish_app",
		Description: "Publish a shard: builds the published channel, VERIFIES it (every declared route mounts, throws nothing, and contains its declared anchors; every anchors.json ref resolves), records the markdown summary, and marks the page live. Publish is REFUSED if verification fails — run verify_app for the detail, fix, and retry. Refs that exist but whose kind emits no change events publish with a warning (they render but never update live). If no renderer is available it publishes UNVERIFIED and returns a warning.",
	}, t.publishApp)

	// --- interactive preview (headless inspection session) -----------------
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "preview_open",
		Description: "Build the app (draft channel by default; pass channel=\"published\" for the published build) and open it in a live headless browser tab kept alive across calls. Returns whether React mounted (#root has content), the current URL, and any console/exceptions. ALWAYS preview before publish; re-run after edits to reload the latest build. If a route doesn't mount or exceptions are non-empty, fix and preview_open again.",
	}, t.previewOpen)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "preview_navigate",
		Description: "Navigate the open preview to an in-app hash route (e.g. \"#/section/sub\" or \"section\"). Docs are ONE app with client-side hash routing; this walks to a sub-page in-place. Returns the post-navigation state.",
	}, t.previewNavigate)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "preview_snapshot",
		Description: "Return the current preview view's text plus a compact DOM outline (tag#id.class with text). Use to verify what actually rendered on a route.",
	}, t.previewSnapshot)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "preview_screenshot",
		Description: "Capture a PNG screenshot of the current preview viewport (returned as an image you can look at) plus the current state.",
	}, t.previewScreenshot)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "preview_eval",
		Description: "Evaluate a JavaScript expression in the open preview and return its JSON value. Use to inspect app state or the DOM (e.g. document.querySelectorAll('button').length).",
	}, t.previewEval)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "preview_click",
		Description: "Click the first element matching a CSS selector in the open preview, then return the resulting state. Use to exercise flows (open a menu, submit, follow an in-app link).",
	}, t.previewClick)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "preview_console",
		Description: "Return the console log lines and uncaught exceptions accumulated in the open preview since it was opened.",
	}, t.previewConsole)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "preview_close",
		Description: "Close the live preview tab for a page and free its resources. Optional — idle tabs are reaped automatically.",
	}, t.previewClose)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "preview_restart",
		Description: "Force-restart the headless preview renderer: tears down the shared browser and ALL preview tabs. Use to recover if preview_open/navigate/eval start failing with 'context canceled' or the renderer seems wedged — the next preview_open rebuilds it. Auto-recovery normally handles a crash, so this is an explicit escape hatch.",
	}, t.previewRestart)
}

// --- inputs / outputs ------------------------------------------------------

type createAppInput struct {
	Title    string      `json:"title"`
	FolderID *string     `json:"folder_id,omitempty"`
	Summary  *string     `json:"summary,omitempty"`
	Agent    *agentInput `json:"agent,omitempty"`
}
type createAppOutput struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// AuthoringGuide + CurrentIndexTSX ride back so the agent writes valid kit
	// code that EXTENDS the seeded module instead of guessing components/props.
	AuthoringGuide  string        `json:"authoring_guide"`
	CurrentIndexTSX string        `json:"current_index_tsx"`
	Citations       []citationOut `json:"citations,omitempty"`
}

type listDirInput struct {
	PageID string `json:"page_id"`
	Path   string `json:"path,omitempty"`
}
type listDirOutput struct {
	Entries []service.FileEntry `json:"entries"`
}

type readFileInput struct {
	PageID string `json:"page_id"`
	Path   string `json:"path"`
}
type readFileOutput struct {
	Content string `json:"content"`
}

type writeFileInput struct {
	PageID  string `json:"page_id"`
	Path    string `json:"path"`
	Content string `json:"content"`
	// Build defaults to true: after the write, a draft build runs and its
	// diagnostics ride back in the result. Set false for bulk multi-file writes
	// (build once at the end, or via build_app).
	Build *bool `json:"build,omitempty"`
	// Overwrite must be set to replace an EXISTING file. Without it a write to
	// an existing path is refused — the common agent failure is clobbering a
	// file it never read (use read_file/edit_file for a surgical change).
	Overwrite bool `json:"overwrite,omitempty"`
}
type writeFileOutput struct {
	OK    bool                 `json:"ok"`
	Path  string               `json:"path"`
	Build *service.BuildResult `json:"build,omitempty"`
}

type editFileInput struct {
	PageID     string `json:"page_id"`
	Path       string `json:"path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
	Build      *bool  `json:"build,omitempty"`
}
type editFileOutput struct {
	OK           bool                 `json:"ok"`
	Path         string               `json:"path"`
	Replacements int                  `json:"replacements"`
	Build        *service.BuildResult `json:"build,omitempty"`
}

type deleteFileInput struct {
	PageID string `json:"page_id"`
	Path   string `json:"path"`
	Build  *bool  `json:"build,omitempty"`
}
type deleteFileOutput struct {
	OK      bool                 `json:"ok"`
	Deleted string               `json:"deleted"`
	Build   *service.BuildResult `json:"build,omitempty"`
}

type installLibInput struct {
	PageID string `json:"page_id"`
	Name   string `json:"name"`
	URL    string `json:"url,omitempty"`
}
type installLibOutput struct {
	Libs []service.LibEntry `json:"libs"`
}

type buildAppInput struct {
	PageID string `json:"page_id"`
}

type publishAppInput struct {
	PageID  string `json:"page_id"`
	Summary string `json:"summary"`
}
type publishAppOutput struct {
	OK        bool   `json:"ok"`
	ServedURL string `json:"served_url"`
	// Verified is true when every manifest route was driven through the live
	// preview and mounted cleanly before publishing. False means the renderer
	// was unavailable and the build shipped UNVERIFIED (see Warning).
	Verified  bool          `json:"verified"`
	Warning   string        `json:"warning,omitempty"`
	Citations []citationOut `json:"citations,omitempty"`
}

// verifyReport is the structured result of a verification pass — the same shape
// verify_app returns and publish_app gates on.
type verifyReport struct {
	OK                bool          `json:"ok"`
	Channel           string        `json:"channel"`
	RendererAvailable bool          `json:"renderer_available"`
	Warning           string        `json:"warning,omitempty"`
	ManifestProblems  []string      `json:"manifest_problems,omitempty"`
	Refs              *refsSummary  `json:"refs,omitempty"`
	Routes            []verifyRoute `json:"routes,omitempty"`
}

type verifyRoute struct {
	Route          string         `json:"route"`
	OK             bool           `json:"ok"`
	Mounted        bool           `json:"mounted"`
	AnchorsFound   map[string]int `json:"anchors_found,omitempty"`
	AnchorsMissing []string       `json:"anchors_missing,omitempty"`
	Exceptions     []string       `json:"exceptions,omitempty"`
	ConsoleErrors  []string       `json:"console_errors,omitempty"`
	NavigateError  string         `json:"navigate_error,omitempty"`
}

type refsSummary struct {
	OK           bool     `json:"ok"`
	Total        int      `json:"total"`
	Missing      []string `json:"missing,omitempty"`
	UnknownKind  []string `json:"unknown_kind,omitempty"`
	Unobservable []string `json:"unobservable,omitempty"`
}

type verifyAppInput struct {
	PageID        string `json:"page_id"`
	Channel       string `json:"channel,omitempty"`        // "draft" (default) | "published"
	StrictConsole bool   `json:"strict_console,omitempty"` // fail on console.error too
}

type previewOpenInput struct {
	PageID  string `json:"page_id"`
	Channel string `json:"channel,omitempty"` // "draft" (default) | "published"
	Theme   string `json:"theme,omitempty"`   // Aladin theme name to render in (e.g. "light"); default dark
}
type previewNavigateInput struct {
	PageID string `json:"page_id"`
	Route  string `json:"route"`
}
type previewSnapshotInput struct {
	PageID string `json:"page_id"`
}
type previewScreenshotInput struct {
	PageID string `json:"page_id"`
}
type previewEvalInput struct {
	PageID string `json:"page_id"`
	Expr   string `json:"expr"`
}
type previewClickInput struct {
	PageID   string `json:"page_id"`
	Selector string `json:"selector"`
}
type previewConsoleInput struct {
	PageID string `json:"page_id"`
}
type previewCloseInput struct {
	PageID string `json:"page_id"`
}
type previewCloseOutput struct {
	OK bool `json:"ok"`
}
type previewRestartInput struct {
	PageID string `json:"page_id"`
}
type previewRestartOutput struct {
	OK bool `json:"ok"`
}

// --- handlers --------------------------------------------------------------

func (t docToolServer) requireApp(ctx context.Context, pageID string) error {
	if strings.TrimSpace(pageID) == "" {
		return service.BadRequest("page_id is required")
	}
	rec, err := t.artifacts.Get(ctx, pageID)
	if err != nil {
		return err
	}
	if rec.Type != appArtifactType {
		return service.BadRequest("artifact is not a Doc Surface page")
	}
	return nil
}

func (t docToolServer) createApp(ctx context.Context, _ *sdkmcp.CallToolRequest, in createAppInput) (*sdkmcp.CallToolResult, createAppOutput, error) {
	if strings.TrimSpace(in.Title) == "" {
		return nil, createAppOutput{}, service.BadRequest("title is required")
	}
	created, err := t.artifacts.Create(ctx, service.ArtifactPayload{
		Type:     appArtifactType,
		FolderID: in.FolderID,
		Title:    in.Title,
		Summary:  in.Summary,
		Metadata: mergePageMetadata(nil, nil, in.Agent),
	})
	if err != nil {
		return nil, createAppOutput{}, err
	}
	id := created.Artifact.ID
	if _, err := t.store.EnsurePageDir(ctx, id); err != nil {
		// Roll back the orphaned row if the dir can't be created.
		_, _ = t.artifacts.Delete(ctx, id)
		return nil, createAppOutput{}, err
	}
	if err := t.store.WriteFile(ctx, id, "index.tsx", []byte(starterIndexTSX)); err != nil {
		_, _ = t.artifacts.Delete(ctx, id)
		return nil, createAppOutput{}, err
	}
	if err := t.store.WriteFile(ctx, id, docsurface.ManifestFileName, []byte(starterAnchorsJSON)); err != nil {
		_, _ = t.artifacts.Delete(ctx, id)
		return nil, createAppOutput{}, err
	}
	return nil, createAppOutput{
		ID:              id,
		Title:           created.Artifact.Title,
		AuthoringGuide:  kitAuthoringGuide,
		CurrentIndexTSX: starterIndexTSX,
		Citations:       []citationOut{{Kind: "shard", ID: id, Title: created.Artifact.Title}},
	}, nil
}

func (t docToolServer) deleteFile(ctx context.Context, _ *sdkmcp.CallToolRequest, in deleteFileInput) (*sdkmcp.CallToolResult, deleteFileOutput, error) {
	if err := t.requireApp(ctx, in.PageID); err != nil {
		return nil, deleteFileOutput{}, err
	}
	if strings.TrimSpace(in.Path) == "" {
		return nil, deleteFileOutput{}, service.BadRequest("path is required")
	}
	if existing, rerr := t.store.ReadFile(ctx, in.PageID, in.Path); rerr == nil {
		t.snapshotFile(ctx, in.PageID, in.Path, existing, "delete")
	}
	if err := t.store.DeleteFile(ctx, in.PageID, in.Path); err != nil {
		return nil, deleteFileOutput{}, err
	}
	return nil, deleteFileOutput{OK: true, Deleted: in.Path, Build: t.maybeAutoBuild(ctx, in.PageID, in.Build)}, nil
}

func (t docToolServer) listDir(ctx context.Context, _ *sdkmcp.CallToolRequest, in listDirInput) (*sdkmcp.CallToolResult, listDirOutput, error) {
	if err := t.requireApp(ctx, in.PageID); err != nil {
		return nil, listDirOutput{}, err
	}
	entries, err := t.store.ListDir(ctx, in.PageID, in.Path)
	if err != nil {
		return nil, listDirOutput{}, err
	}
	return nil, listDirOutput{Entries: entries}, nil
}

func (t docToolServer) readFile(ctx context.Context, _ *sdkmcp.CallToolRequest, in readFileInput) (*sdkmcp.CallToolResult, readFileOutput, error) {
	if err := t.requireApp(ctx, in.PageID); err != nil {
		return nil, readFileOutput{}, err
	}
	if strings.TrimSpace(in.Path) == "" {
		return nil, readFileOutput{}, service.BadRequest("path is required")
	}
	data, err := t.store.ReadFile(ctx, in.PageID, in.Path)
	if err != nil {
		return nil, readFileOutput{}, err
	}
	return nil, readFileOutput{Content: string(data)}, nil
}

func (t docToolServer) writeFile(ctx context.Context, _ *sdkmcp.CallToolRequest, in writeFileInput) (*sdkmcp.CallToolResult, writeFileOutput, error) {
	if err := t.requireApp(ctx, in.PageID); err != nil {
		return nil, writeFileOutput{}, err
	}
	if strings.TrimSpace(in.Path) == "" {
		return nil, writeFileOutput{}, service.BadRequest("path is required")
	}
	if existing, rerr := t.store.ReadFile(ctx, in.PageID, in.Path); rerr == nil {
		if !in.Overwrite {
			return nil, writeFileOutput{}, service.BadRequest(fmt.Sprintf(
				"%s already exists (%d bytes) — read it first, then use edit_file for a targeted change, or pass overwrite:true to replace it wholesale.",
				in.Path, len(existing)))
		}
		t.snapshotFile(ctx, in.PageID, in.Path, existing, "write")
	}
	if err := t.store.WriteFile(ctx, in.PageID, in.Path, []byte(in.Content)); err != nil {
		return nil, writeFileOutput{}, err
	}
	return nil, writeFileOutput{OK: true, Path: in.Path, Build: t.maybeAutoBuild(ctx, in.PageID, in.Build)}, nil
}

// historyDir holds pre-change snapshots. Deliberately minimal: a copy of the
// previous bytes under a sortable timestamp, inspectable and restorable with
// the tools that already exist (list_dir / read_file / write_file), pruned to a
// small cap. Not version control — just enough that an overwrite or delete is
// recoverable. It lives outside the build graph (only imports are bundled).
const historyDir = ".history"

const historyKeep = 20

// snapshotFile copies a file's current bytes into .history before it is
// replaced or removed. Each snapshot is ONE flat file —
// ".history/<stamp>-<op>-<path with / as __>" — so pruning is a plain file
// delete (removing a directory tree is not something the store can do).
// Best-effort by design: losing a snapshot must never fail the write the agent
// actually asked for.
func (t docToolServer) snapshotFile(ctx context.Context, pageID, path string, content []byte, op string) {
	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	dest := fmt.Sprintf("%s/%s-%s-%s", historyDir, stamp, op, strings.ReplaceAll(path, "/", "__"))
	if err := t.store.WriteFile(ctx, pageID, dest, content); err != nil {
		return
	}
	t.pruneHistory(ctx, pageID)
}

// pruneHistory keeps the newest historyKeep snapshots. Names begin with a
// fixed-width UTC timestamp, so lexical order is chronological.
func (t docToolServer) pruneHistory(ctx context.Context, pageID string) {
	entries, err := t.store.ListDir(ctx, pageID, historyDir)
	if err != nil || len(entries) <= historyKeep {
		return
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir {
			names = append(names, e.Name)
		}
	}
	if len(names) <= historyKeep {
		return
	}
	sort.Strings(names)
	for _, name := range names[:len(names)-historyKeep] {
		_ = t.store.DeleteFile(ctx, pageID, historyDir+"/"+name)
	}
}

func (t docToolServer) editFile(ctx context.Context, _ *sdkmcp.CallToolRequest, in editFileInput) (*sdkmcp.CallToolResult, editFileOutput, error) {
	if err := t.requireApp(ctx, in.PageID); err != nil {
		return nil, editFileOutput{}, err
	}
	if strings.TrimSpace(in.Path) == "" {
		return nil, editFileOutput{}, service.BadRequest("path is required")
	}
	if in.OldString == "" {
		return nil, editFileOutput{}, service.BadRequest("old_string is required")
	}
	if in.OldString == in.NewString {
		return nil, editFileOutput{}, service.BadRequest("old_string and new_string are identical")
	}
	data, err := t.store.ReadFile(ctx, in.PageID, in.Path)
	if err != nil {
		return nil, editFileOutput{}, err
	}
	updated, count, err := applyStringEdit(string(data), in.OldString, in.NewString, in.ReplaceAll)
	switch {
	case errors.Is(err, errEditNotFound):
		return nil, editFileOutput{}, service.BadRequest("old_string not found in " + in.Path)
	case errors.Is(err, errEditAmbiguous):
		return nil, editFileOutput{}, service.BadRequest(fmt.Sprintf(
			"old_string matches %d times in %s; add surrounding context to make it unique, or set replace_all", count, in.Path))
	case err != nil:
		return nil, editFileOutput{}, err
	}
	if err := t.store.WriteFile(ctx, in.PageID, in.Path, []byte(updated)); err != nil {
		return nil, editFileOutput{}, err
	}
	return nil, editFileOutput{OK: true, Path: in.Path, Replacements: count, Build: t.maybeAutoBuild(ctx, in.PageID, in.Build)}, nil
}

var (
	errEditNotFound  = errors.New("old_string not found")
	errEditAmbiguous = errors.New("old_string ambiguous")
)

// applyStringEdit performs an exact-string replacement. old_string must occur
// exactly once unless replaceAll is set; absent → errEditNotFound, multiple
// without replaceAll → errEditAmbiguous (with the match count). Returns the new
// content and the number of replacements.
func applyStringEdit(content, oldStr, newStr string, replaceAll bool) (string, int, error) {
	count := strings.Count(content, oldStr)
	switch {
	case count == 0:
		return "", 0, errEditNotFound
	case count > 1 && !replaceAll:
		return "", count, errEditAmbiguous
	case replaceAll:
		return strings.ReplaceAll(content, oldStr, newStr), count, nil
	default:
		return strings.Replace(content, oldStr, newStr, 1), 1, nil
	}
}

// maybeAutoBuild runs a synchronous DRAFT build after a write (unless build is
// explicitly false), returning the result so diagnostics ride back inline. A Go
// error from the build is folded into a failed BuildResult rather than failing
// the write — the file IS written; the agent reads the log and iterates.
func (t docToolServer) maybeAutoBuild(ctx context.Context, pageID string, build *bool) *service.BuildResult {
	if build != nil && !*build {
		return nil
	}
	res, err := t.build.Build(ctx, pageID, service.ChannelDraft)
	if err != nil {
		return &service.BuildResult{OK: false, Log: err.Error()}
	}
	return &res
}

func (t docToolServer) installLib(ctx context.Context, _ *sdkmcp.CallToolRequest, in installLibInput) (*sdkmcp.CallToolResult, installLibOutput, error) {
	if err := t.requireApp(ctx, in.PageID); err != nil {
		return nil, installLibOutput{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, installLibOutput{}, service.BadRequest("name is required")
	}
	url := strings.TrimSpace(in.URL)
	if url == "" {
		url = "https://esm.sh/" + name
	} else if !strings.HasPrefix(url, "https://esm.sh/") {
		// Build-time SSRF guard: deps may only come from esm.sh.
		return nil, installLibOutput{}, service.BadRequest("url must be an https://esm.sh/ URL")
	}
	// The manifest is keyed by the bare package name; strip any @version for the key.
	key := name
	if at := strings.LastIndex(name, "@"); at > 0 {
		key = name[:at]
	}
	libs, err := t.store.InstallLib(ctx, in.PageID, service.LibEntry{Name: key, URL: url})
	if err != nil {
		return nil, installLibOutput{}, err
	}
	return nil, installLibOutput{Libs: libs}, nil
}

func (t docToolServer) buildApp(ctx context.Context, _ *sdkmcp.CallToolRequest, in buildAppInput) (*sdkmcp.CallToolResult, service.BuildResult, error) {
	if err := t.requireApp(ctx, in.PageID); err != nil {
		return nil, service.BuildResult{}, err
	}
	res, err := t.build.Build(ctx, in.PageID, service.ChannelPublished)
	if err != nil {
		return nil, service.BuildResult{}, err
	}
	return nil, res, nil
}

// verifyAppTool is the agent-facing verification pass: the full structured
// report (manifest, refs, per-route mount/anchors/exceptions/console) with no
// side effects. Run it before publish_app — publish applies the same checks,
// but this one tells you exactly what is wrong while you can still fix it.
func (t docToolServer) verifyAppTool(ctx context.Context, _ *sdkmcp.CallToolRequest, in verifyAppInput) (*sdkmcp.CallToolResult, verifyReport, error) {
	if err := t.requireApp(ctx, in.PageID); err != nil {
		return nil, verifyReport{}, err
	}
	channel := service.ChannelDraft
	if in.Channel == string(service.ChannelPublished) {
		channel = service.ChannelPublished
	}
	report, err := t.verifyApp(ctx, in.PageID, channel, in.StrictConsole)
	if err != nil {
		return nil, verifyReport{}, err
	}
	if t.bridge != nil {
		refs, rerr := t.bridge.CheckRefs(ctx, in.PageID)
		if rerr != nil {
			return nil, verifyReport{}, rerr
		}
		report.Refs = &refsSummary{
			OK:           refs.OK,
			Total:        refs.Total,
			Missing:      refs.Missing,
			UnknownKind:  refs.UnknownKind,
			Unobservable: refs.Unobservable,
		}
		if !refs.OK {
			report.OK = false
		}
	}
	return nil, report, nil
}

func (t docToolServer) publishApp(ctx context.Context, _ *sdkmcp.CallToolRequest, in publishAppInput) (*sdkmcp.CallToolResult, publishAppOutput, error) {
	if err := t.requireApp(ctx, in.PageID); err != nil {
		return nil, publishAppOutput{}, err
	}
	// Build the PUBLISHED channel here rather than trusting a marker file. The
	// old flow gated on dist/.build-meta.json merely EXISTING, while every
	// write auto-builds the DRAFT channel — so a shard edited after its last
	// build_app could publish stale bytes that no check ever saw. Building now
	// makes "what was verified" and "what goes live" the same artifact.
	if t.build != nil {
		res, berr := t.build.Build(ctx, in.PageID, service.ChannelPublished)
		if berr != nil {
			return nil, publishAppOutput{}, berr
		}
		if !res.OK {
			return nil, publishAppOutput{}, service.BadRequest("publish blocked — the published build failed:\n" + res.Log)
		}
	} else if _, err := t.store.ReadFile(ctx, in.PageID, docsurface.BuildMetaPath); err != nil {
		return nil, publishAppOutput{}, service.BadRequest("no successful build found — run build_app first")
	}
	// Gate on the full verification pass against what was just built: mounts,
	// no uncaught exceptions/rejections, and every declared anchor present.
	// Hard-fails when the renderer is available; soft-warns (publishes
	// UNVERIFIED) only when there's no renderer to verify with.
	report, err := t.verifyApp(ctx, in.PageID, service.ChannelPublished, false)
	if err != nil {
		return nil, publishAppOutput{}, err
	}
	if problems := verifyFailure(report); problems != "" {
		return nil, publishAppOutput{}, service.BadRequest("publish blocked — verification failed:\n  - " + problems +
			"\nRun verify_app for the full report, fix, then publish_app again.")
	}
	verified, warning := report.RendererAvailable, report.Warning
	// Gate on the manifest's REFS too: a shard that declares data it can't read
	// renders an empty region for the user with no error anywhere. A missing or
	// unknown-kind ref is a hard refusal; a ref whose kind emits no sync frames
	// publishes with a warning (renderable, but the region can never go live).
	if t.bridge != nil {
		refs, rerr := t.bridge.CheckRefs(ctx, in.PageID)
		if rerr != nil {
			return nil, publishAppOutput{}, rerr
		}
		if !refs.OK {
			var problems []string
			if len(refs.Missing) > 0 {
				problems = append(problems, "not found: "+strings.Join(refs.Missing, ", "))
			}
			if len(refs.UnknownKind) > 0 {
				problems = append(problems, "unknown kind: "+strings.Join(refs.UnknownKind, ", "))
			}
			return nil, publishAppOutput{}, service.BadRequest(
				"publish blocked — anchors.json refs don't resolve (" + strings.Join(problems, "; ") +
					"). Fix the ids or drop them from refs.")
		}
		if len(refs.Unobservable) > 0 {
			unobservable := "these refs can be read but never update live (their kind emits no change events): " +
				strings.Join(refs.Unobservable, ", ")
			if warning == "" {
				warning = unobservable
			} else {
				warning += "; " + unobservable
			}
		}
	}
	summary := strings.TrimSpace(in.Summary)
	if summary != "" {
		if _, err := t.artifacts.Update(ctx, in.PageID, service.ArtifactPatch{Summary: &summary}); err != nil {
			return nil, publishAppOutput{}, err
		}
	}
	// Reconcile the DRAFT build-state. A successful publish proves the current
	// source builds; without this, a stale 'failed' left in the draft channel
	// from mid-authoring (fixed before publish but never rebuilt on draft) keeps
	// the work pane showing "build failed" for a shard that is in fact live. This
	// rebuilds draft through the state-recording path. Best-effort: a published
	// shard stays published even if the refresh hiccups.
	if t.build != nil {
		_, _ = t.build.Build(ctx, in.PageID, service.ChannelDraft)
	}
	return nil, publishAppOutput{
		OK:        true,
		ServedURL: "/content/" + in.PageID + "/",
		Verified:  verified,
		Warning:   warning,
		Citations: []citationOut{{Kind: "shard", ID: in.PageID}},
	}, nil
}

// verifyApp drives the live preview across the page's declared routes and
// checks, per route: it mounts, it threw no uncaught exceptions (unhandled
// promise rejections included — the preview captures those as console.error),
// and every anchor the manifest declares for that route actually exists in the
// DOM. console.error lines are always REPORTED; they only fail the pass when
// strictConsole is set, because vendored libraries legitimately log errors.
//
// The report is the shared truth: verify_app returns it as-is, and publish
// turns it into a refusal.
func (t docToolServer) verifyApp(ctx context.Context, pageID string, channel service.BuildChannel, strictConsole bool) (verifyReport, error) {
	report := verifyReport{Channel: string(channel)}

	// Structure first: a manifest that doesn't parse can't be checked against,
	// and must never silently degrade to "just check the root".
	if data, err := t.store.ReadFile(ctx, pageID, docsurface.ManifestFileName); err == nil {
		report.ManifestProblems = docsurface.ValidateManifestBytes(data)
		if len(report.ManifestProblems) > 0 {
			return report, nil // no point driving a browser against a broken manifest
		}
	}
	byRoute, routes := t.manifestAnchorsByRoute(ctx, pageID)

	first, err := t.preview.Open(ctx, pageID, channel, service.PreviewOpenOptions{})
	if err != nil {
		if docsurface.IsRendererUnavailable(err) {
			report.RendererAvailable = false
			report.Warning = "renderer unavailable — nothing was verified; preview the routes manually before relying on this build."
			return report, nil
		}
		return report, err
	}
	report.RendererAvailable = true

	check := func(route string, st service.PreviewState) verifyRoute {
		vr := verifyRoute{
			Route:      route,
			Mounted:    st.Mounted,
			Exceptions: st.Exceptions,
		}
		if errs, cerr := t.preview.ConsoleErrors(ctx, pageID); cerr == nil {
			vr.ConsoleErrors = errs
		}
		declared := byRoute[route]
		if len(declared) > 0 {
			counts, aerr := t.preview.CheckAnchors(ctx, pageID, declared)
			if aerr == nil {
				vr.AnchorsFound = counts
				for _, id := range declared {
					if counts[id] == 0 {
						vr.AnchorsMissing = append(vr.AnchorsMissing, id)
					}
				}
			}
		}
		vr.OK = vr.Mounted && len(vr.Exceptions) == 0 && len(vr.AnchorsMissing) == 0 &&
			(!strictConsole || len(vr.ConsoleErrors) == 0)
		return vr
	}

	// Open landed on the app's default route ("#/"); check it, then walk the rest.
	firstRoute := firstNonEmpty(routeOf(first.URL), "#/")
	report.Routes = append(report.Routes, check(firstRoute, first))
	for _, r := range routes {
		if r == firstRoute {
			continue // already covered by the initial Open
		}
		st, nerr := t.preview.Navigate(ctx, pageID, r)
		if nerr != nil {
			if docsurface.IsRendererUnavailable(nerr) {
				report.RendererAvailable = false
				report.Warning = "renderer unavailable mid-verification — the remaining routes were not checked."
				return report, nil
			}
			report.Routes = append(report.Routes, verifyRoute{Route: r, NavigateError: firstLine(nerr.Error())})
			continue
		}
		report.Routes = append(report.Routes, check(r, st))
	}

	report.OK = len(report.ManifestProblems) == 0
	for _, r := range report.Routes {
		if !r.OK {
			report.OK = false
		}
	}
	return report, nil
}

// verifyFailure renders a report's problems as the message a publish refusal
// carries; "" when nothing is wrong.
func verifyFailure(report verifyReport) string {
	if report.OK || !report.RendererAvailable {
		return ""
	}
	var lines []string
	for _, p := range report.ManifestProblems {
		lines = append(lines, "anchors.json: "+p)
	}
	for _, r := range report.Routes {
		if r.OK {
			continue
		}
		switch {
		case r.NavigateError != "":
			lines = append(lines, r.Route+" (navigate error: "+r.NavigateError+")")
		case !r.Mounted:
			lines = append(lines, r.Route+" (did not mount)")
		case len(r.Exceptions) > 0:
			lines = append(lines, fmt.Sprintf("%s (%d uncaught exception(s): %s)", r.Route, len(r.Exceptions), firstLine(r.Exceptions[0])))
		case len(r.AnchorsMissing) > 0:
			lines = append(lines, fmt.Sprintf("%s (declared anchors not in the DOM: %s)", r.Route, strings.Join(r.AnchorsMissing, ", ")))
		case len(r.ConsoleErrors) > 0:
			lines = append(lines, fmt.Sprintf("%s (%d console error(s): %s)", r.Route, len(r.ConsoleErrors), firstLine(r.ConsoleErrors[0])))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n  - ")
}

// routeOf extracts the hash route from a preview URL ("about:blank#/x" → "#/x").
func routeOf(url string) string {
	if i := strings.IndexByte(url, '#'); i >= 0 {
		return url[i:]
	}
	return ""
}

// manifestAnchorsByRoute groups declared anchor ids by their route, and returns
// the distinct routes in declaration order. Falls back to just "#/" when there
// is no manifest — the root must at least mount.
func (t docToolServer) manifestAnchorsByRoute(ctx context.Context, pageID string) (map[string][]string, []string) {
	data, err := t.store.ReadFile(ctx, pageID, docsurface.ManifestFileName)
	if err != nil {
		return nil, []string{"#/"}
	}
	m, err := docsurface.ParseManifest(data)
	if err != nil {
		return nil, []string{"#/"}
	}
	byRoute := map[string][]string{}
	seen := map[string]bool{}
	var routes []string
	for _, a := range m.Anchors {
		r := strings.TrimSpace(a.Route)
		if r == "" {
			continue
		}
		if !seen[r] {
			seen[r] = true
			routes = append(routes, r)
		}
		if id := strings.TrimSpace(a.ID); id != "" {
			byRoute[r] = append(byRoute[r], id)
		}
	}
	if len(routes) == 0 {
		return byRoute, []string{"#/"}
	}
	return byRoute, routes
}

// firstLine returns s up to its first newline, trimmed and length-capped, for a
// compact one-line failure note.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	if len(s) > 160 {
		s = s[:160] + "…"
	}
	return s
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// --- preview handlers ------------------------------------------------------

// previewSummary is a one-line human-readable digest of a preview state, used as
// the text block alongside a screenshot image.
func previewSummary(st service.PreviewState) string {
	return fmt.Sprintf("url=%s mounted=%t console=%d exceptions=%d", st.URL, st.Mounted, len(st.Console), len(st.Exceptions))
}

func (t docToolServer) previewOpen(ctx context.Context, _ *sdkmcp.CallToolRequest, in previewOpenInput) (*sdkmcp.CallToolResult, service.PreviewState, error) {
	if err := t.requireApp(ctx, in.PageID); err != nil {
		return nil, service.PreviewState{}, err
	}
	channel := service.ChannelDraft
	if in.Channel == string(service.ChannelPublished) {
		channel = service.ChannelPublished
	}
	st, err := t.preview.Open(ctx, in.PageID, channel, service.PreviewOpenOptions{Theme: in.Theme})
	if err != nil {
		return nil, service.PreviewState{}, err
	}
	return nil, st, nil
}

func (t docToolServer) previewNavigate(ctx context.Context, _ *sdkmcp.CallToolRequest, in previewNavigateInput) (*sdkmcp.CallToolResult, service.PreviewState, error) {
	if err := t.requireApp(ctx, in.PageID); err != nil {
		return nil, service.PreviewState{}, err
	}
	st, err := t.preview.Navigate(ctx, in.PageID, in.Route)
	if err != nil {
		return nil, service.PreviewState{}, err
	}
	return nil, st, nil
}

func (t docToolServer) previewSnapshot(ctx context.Context, _ *sdkmcp.CallToolRequest, in previewSnapshotInput) (*sdkmcp.CallToolResult, service.PreviewState, error) {
	if err := t.requireApp(ctx, in.PageID); err != nil {
		return nil, service.PreviewState{}, err
	}
	st, err := t.preview.Snapshot(ctx, in.PageID)
	if err != nil {
		return nil, service.PreviewState{}, err
	}
	return nil, st, nil
}

func (t docToolServer) previewScreenshot(ctx context.Context, _ *sdkmcp.CallToolRequest, in previewScreenshotInput) (*sdkmcp.CallToolResult, service.PreviewState, error) {
	if err := t.requireApp(ctx, in.PageID); err != nil {
		return nil, service.PreviewState{}, err
	}
	png, st, err := t.preview.Screenshot(ctx, in.PageID)
	if err != nil {
		return nil, service.PreviewState{}, err
	}
	// Return the image + a human summary as Content; the SDK fills
	// StructuredContent from the typed PreviewState.
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{
			&sdkmcp.ImageContent{Data: png, MIMEType: "image/png"},
			&sdkmcp.TextContent{Text: previewSummary(st)},
		},
	}, st, nil
}

func (t docToolServer) previewEval(ctx context.Context, _ *sdkmcp.CallToolRequest, in previewEvalInput) (*sdkmcp.CallToolResult, service.PreviewState, error) {
	if err := t.requireApp(ctx, in.PageID); err != nil {
		return nil, service.PreviewState{}, err
	}
	st, err := t.preview.Eval(ctx, in.PageID, in.Expr)
	if err != nil {
		return nil, service.PreviewState{}, err
	}
	return nil, st, nil
}

func (t docToolServer) previewClick(ctx context.Context, _ *sdkmcp.CallToolRequest, in previewClickInput) (*sdkmcp.CallToolResult, service.PreviewState, error) {
	if err := t.requireApp(ctx, in.PageID); err != nil {
		return nil, service.PreviewState{}, err
	}
	st, err := t.preview.Click(ctx, in.PageID, in.Selector)
	if err != nil {
		return nil, service.PreviewState{}, err
	}
	return nil, st, nil
}

func (t docToolServer) previewConsole(ctx context.Context, _ *sdkmcp.CallToolRequest, in previewConsoleInput) (*sdkmcp.CallToolResult, service.PreviewState, error) {
	if err := t.requireApp(ctx, in.PageID); err != nil {
		return nil, service.PreviewState{}, err
	}
	st, err := t.preview.Console(ctx, in.PageID)
	if err != nil {
		return nil, service.PreviewState{}, err
	}
	return nil, st, nil
}

func (t docToolServer) previewClose(ctx context.Context, _ *sdkmcp.CallToolRequest, in previewCloseInput) (*sdkmcp.CallToolResult, previewCloseOutput, error) {
	if err := t.requireApp(ctx, in.PageID); err != nil {
		return nil, previewCloseOutput{}, err
	}
	if err := t.preview.Close(ctx, in.PageID); err != nil {
		return nil, previewCloseOutput{}, err
	}
	return nil, previewCloseOutput{OK: true}, nil
}

func (t docToolServer) previewRestart(ctx context.Context, _ *sdkmcp.CallToolRequest, in previewRestartInput) (*sdkmcp.CallToolResult, previewRestartOutput, error) {
	if err := t.requireApp(ctx, in.PageID); err != nil {
		return nil, previewRestartOutput{}, err
	}
	if err := t.preview.Reset(ctx); err != nil {
		return nil, previewRestartOutput{}, err
	}
	return nil, previewRestartOutput{OK: true}, nil
}
