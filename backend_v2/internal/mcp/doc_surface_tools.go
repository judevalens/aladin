package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
		Name:        "publish_app",
		Description: "Publish a built Doc Surface page: records the markdown summary (the knowledge-graph spine) and marks the page live. Requires a successful build_app first. Before going live it VERIFIES the build by driving every manifest route through the live preview — publish is REFUSED if a route fails to mount or throws (fix it and retry). If no renderer is available it publishes UNVERIFIED and returns a warning.",
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
	if err := t.store.WriteFile(ctx, in.PageID, in.Path, []byte(in.Content)); err != nil {
		return nil, writeFileOutput{}, err
	}
	return nil, writeFileOutput{OK: true, Path: in.Path, Build: t.maybeAutoBuild(ctx, in.PageID, in.Build)}, nil
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

func (t docToolServer) publishApp(ctx context.Context, _ *sdkmcp.CallToolRequest, in publishAppInput) (*sdkmcp.CallToolResult, publishAppOutput, error) {
	if err := t.requireApp(ctx, in.PageID); err != nil {
		return nil, publishAppOutput{}, err
	}
	// Require a prior successful build (the marker, not just a stale bundle.js).
	if _, err := t.store.ReadFile(ctx, in.PageID, docsurface.BuildMetaPath); err != nil {
		return nil, publishAppOutput{}, service.BadRequest("no successful build found — run build_app first")
	}
	// Gate on a live mount check across the manifest's routes: a build that
	// doesn't actually render must not go live. Hard-fails (refuses to publish)
	// when the renderer is available and a route is broken; soft-warns (publishes
	// UNVERIFIED) only when there's no renderer to verify with.
	verified, warning, err := t.verifyMount(ctx, in.PageID)
	if err != nil {
		return nil, publishAppOutput{}, err
	}
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

// verifyMount drives the live preview across the page's declared manifest routes
// and confirms each mounts cleanly. Returns (verified, warning, err):
//   - err != nil   → a route failed to mount with the renderer AVAILABLE: a hard
//     gate; publish is refused. (Also surfaces a genuine build failure.)
//   - verified=false + warning → renderer UNAVAILABLE (no Chrome): a soft warn;
//     the caller publishes anyway but the result is stamped unverified.
//   - verified=true → every route mounted with no uncaught exceptions.
func (t docToolServer) verifyMount(ctx context.Context, pageID string) (bool, string, error) {
	routes := t.manifestRoutes(ctx, pageID)
	first, err := t.preview.Open(ctx, pageID, service.ChannelPublished, service.PreviewOpenOptions{})
	if err != nil {
		if docsurface.IsRendererUnavailable(err) {
			return false, "renderer unavailable — published WITHOUT mount verification; preview the routes manually before relying on this build.", nil
		}
		return false, "", err
	}

	var failed []string
	note := func(route string, st service.PreviewState) {
		switch {
		case !st.Mounted:
			failed = append(failed, route+" (did not mount)")
		case len(st.Exceptions) > 0:
			failed = append(failed, fmt.Sprintf("%s (%d uncaught exception(s): %s)", route, len(st.Exceptions), firstLine(st.Exceptions[0])))
		}
	}
	// Open landed on the app's default route ("#/"); verify it, then walk the rest.
	note(firstNonEmpty(first.URL, "#/"), first)
	for _, r := range routes {
		if r == "#/" {
			continue // already covered by the initial Open
		}
		st, nerr := t.preview.Navigate(ctx, pageID, r)
		if nerr != nil {
			if docsurface.IsRendererUnavailable(nerr) {
				return false, "renderer unavailable mid-verification — published UNVERIFIED.", nil
			}
			failed = append(failed, r+" (navigate error: "+firstLine(nerr.Error())+")")
			continue
		}
		note(r, st)
	}

	if len(failed) > 0 {
		return false, "", service.BadRequest("publish blocked — these routes did not mount cleanly:\n  - " +
			strings.Join(failed, "\n  - ") +
			"\nFix the build (use preview_open/preview_navigate to debug), then build_app + publish_app again.")
	}
	return true, "", nil
}

// manifestRoutes returns the distinct, declared routes from the page's
// anchors.json (in declaration order). Falls back to just "#/" when there is no
// manifest or no routes — the root must at least mount.
func (t docToolServer) manifestRoutes(ctx context.Context, pageID string) []string {
	data, err := t.store.ReadFile(ctx, pageID, docsurface.ManifestFileName)
	if err != nil {
		return []string{"#/"}
	}
	m, err := docsurface.ParseManifest(data)
	if err != nil {
		return []string{"#/"}
	}
	seen := map[string]bool{}
	var routes []string
	for _, a := range m.Anchors {
		r := strings.TrimSpace(a.Route)
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		routes = append(routes, r)
	}
	if len(routes) == 0 {
		return []string{"#/"}
	}
	return routes
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
