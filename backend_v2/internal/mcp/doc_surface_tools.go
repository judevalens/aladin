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

// docToolServer carries the deps the Doc Surface tools need. The artifact
// service scopes every Get/Create/Update to the caller's principal; the store
// scopes file IO to the same principal. Together they enforce ownership.
type docToolServer struct {
	artifacts service.ArtifactService
	store     service.DocSurfaceStore
	build     service.ShardBuildService
	preview   service.PreviewService
}

func registerDocSurfaceTools(server *sdkmcp.Server, artifacts service.ArtifactService, store service.DocSurfaceStore, build service.ShardBuildService, preview service.PreviewService) {
	t := docToolServer{artifacts: artifacts, store: store, build: build, preview: preview}

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
		Name:        "install_lib",
		Description: "Add an npm dependency to a Doc Surface page. Provide name (optionally name@version); the lib is bundled from esm.sh at build time. import it normally in your code.",
	}, t.installLib)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "build_app",
		Description: "Build a Doc Surface page (bundles index.tsx with esbuild). Returns ok + a build log; on failure, read the log, fix the files, and build again.",
	}, t.buildApp)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "publish_app",
		Description: "Publish a built Doc Surface page: records the markdown summary (the knowledge-graph spine) and marks the page live. Requires a successful build_app first.",
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
}

type previewOpenInput struct {
	PageID  string `json:"page_id"`
	Channel string `json:"channel,omitempty"` // "draft" (default) | "published"
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
	return nil, createAppOutput{ID: id, Title: created.Artifact.Title}, nil
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
	summary := strings.TrimSpace(in.Summary)
	if summary != "" {
		if _, err := t.artifacts.Update(ctx, in.PageID, service.ArtifactPatch{Summary: &summary}); err != nil {
			return nil, publishAppOutput{}, err
		}
	}
	return nil, publishAppOutput{OK: true, ServedURL: "/content/" + in.PageID + "/"}, nil
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
	st, err := t.preview.Open(ctx, in.PageID, channel)
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
