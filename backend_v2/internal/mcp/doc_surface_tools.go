package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"aladin/backend_v2/internal/docsurface"
	"aladin/backend_v2/internal/service"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// appArtifactType is the artifact type string for Doc Surface pages (distinct
// from "page", which is a BlockNote note).
const appArtifactType = "app"

// Compatibility aliases for the MCP-level behavior tests. History ownership
// now lives behind the Doc Surface authoring application boundary.
const (
	historyDir  = docsurface.HistoryDir
	historyKeep = docsurface.HistoryKeep
)

// starterIndexTSX is intentionally plain React. Aladin injects theme tokens and
// Tailwind utilities, while authors own their markup and visual language.
const starterIndexTSX = `import { createRoot } from "react-dom/client";

function App() {
  return (
    <main className="min-h-screen bg-bg px-8 py-10 text-ink">
      <section
        data-anchor="intro"
        data-kind="narrative"
        className="mx-auto max-w-3xl rounded-card border border-line bg-panel p-6"
      >
        <p className="font-mono text-meta uppercase tracking-wider text-amber">New shard</p>
        <h1 className="mt-2 font-display text-title">Build the interface this tool needs.</h1>
        <p className="mt-3 max-w-xl text-body text-ink-2">
          Use ordinary React, HTML and CSS. Aladin supplies theme tokens and the
          nonvisual @aladin/shard data SDK; your shard owns its UI.
        </p>
      </section>
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
`

// starterAnchorsJSON seeds the manifest so the addressable surface is declared
// from the start (the data-anchor above). Agents extend it as they add regions.
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
	// bridge audits the manifest's refs at publish (nil in tests that don't
	// exercise the workspace plane).
	bridge   service.ShardBridgeService
	releases service.ShardReleaseService
	graphql  service.ShardGraphQLService
}

func (t docToolServer) authoring() *docsurface.Authoring {
	return docsurface.NewAuthoring(t.artifacts, t.store, t.build)
}

func (t docToolServer) publication() *docsurface.Publication {
	return docsurface.NewPublication(t.artifacts, t.store, t.build, t.preview, t.bridge, t.releases)
}

func (t docToolServer) previewCommands() *docsurface.PreviewCommands {
	return docsurface.NewPreviewCommands(t.artifacts, t.store, t.build, t.preview)
}

func registerDocSurfaceTools(server *sdkmcp.Server, artifacts service.ArtifactService, store service.DocSurfaceStore, build service.ShardBuildService, preview service.PreviewService, bridge service.ShardBridgeService, releases service.ShardReleaseService, graphql ...service.ShardGraphQLService) {
	t := docToolServer{artifacts: artifacts, store: store, build: build, preview: preview, bridge: bridge, releases: releases}
	if len(graphql) > 0 {
		t.graphql = graphql[0]
	}

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "create_app",
		Description: "Create a Doc Surface page (an interactive React app rendered in a sandboxed iframe). Seeds index.tsx and the data configuration enabled on this backend. Follow its returned authoring_guide; no runtime-version choice is needed. Then write_file to author, build_app to compile, publish_app to make it live. NOTE: distinct from create_page (which makes a BlockNote document).",
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
		Annotations: destructiveTool("Delete file"),
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
		Name:        "get_authoring_guide",
		Description: "Read the currently available shard building capabilities, Tailwind theme tokens and nonvisual @aladin/shard API reference. Call this before answering capability questions or editing a shard. Without page_id it describes new shards on this backend. With page_id it selects the supported APIs for that existing shard and returns its files, anchors, contract when present, and current index.tsx. Use the returned capabilities directly; no runtime-version choice is needed.",
	}, t.getAuthoringGuide)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "verify_app",
		Description: "Verify a shard WITHOUT publishing: validates its manifest and data declarations, then drives each declared route through the live preview and reports per route whether it mounted, which declared anchors are actually in the DOM, uncaught exceptions (unhandled promise rejections included), and console errors. Defaults to the draft channel; pass strict_console=true to treat console errors as failures. Run this before publish_app — this shows you the report while you can still fix it. Follow get_authoring_guide(page_id) for this shard's data and renderer requirements.",
	}, t.verifyAppTool)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "publish_app",
		Annotations: destructiveTool("Publish app"),
		Description: "Publish a shard: builds the published channel, verifies declared routes, anchors and data sources, records the markdown summary, and marks the page live. Publication is refused when required verification fails — run verify_app for the detail, fix, and retry. Follow get_authoring_guide(page_id) for this shard's renderer and data requirements; do not assume renderer verification can be skipped.",
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
	// AuthoringGuide + CurrentIndexTSX ride back so the agent writes valid code
	// that extends the seeded module using the target's current capabilities.
	AuthoringGuide  string        `json:"authoring_guide"`
	CurrentIndexTSX string        `json:"current_index_tsx"`
	ContractJSON    string        `json:"contract_json,omitempty"`
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

type verifyReport = docsurface.VerificationReport
type verifyRoute = docsurface.VerificationRoute
type refsSummary = docsurface.ReferenceSummary

type authoringGuideInput struct {
	// PageID is optional: with it the guide comes back alongside the shard's
	// current files and manifest, which is what an EDIT actually needs.
	PageID string `json:"page_id,omitempty"`
}

type authoringGuideOutput struct {
	AuthoringGuide string   `json:"authoring_guide"`
	Files          []string `json:"files,omitempty"`
	Anchors        string   `json:"anchors_json,omitempty"`
	IndexTSX       string   `json:"current_index_tsx,omitempty"`
	ContractJSON   string   `json:"contract_json,omitempty"`
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

func (t docToolServer) createApp(ctx context.Context, _ *sdkmcp.CallToolRequest, in createAppInput) (*sdkmcp.CallToolResult, createAppOutput, error) {
	resources := t.resourceAuthoringEnabled()
	contract := ""
	files := map[string][]byte{
		"index.tsx":                 []byte(starterIndexTSX),
		docsurface.ManifestFileName: []byte(starterAnchorsJSON),
	}
	if resources {
		contract = starterResourceContractJSON
		files["contract.json"] = []byte(contract)
	}
	created, err := t.authoring().Create(ctx, docsurface.CreateCommand{
		Artifact: service.ArtifactPayload{
			FolderID: in.FolderID,
			Title:    in.Title,
			Summary:  in.Summary,
			Metadata: mergePageMetadata(nil, nil, in.Agent),
		},
		Files: files,
	})
	if err != nil {
		return nil, createAppOutput{}, err
	}
	return nil, createAppOutput{
		ContractJSON:    contract,
		ID:              created.ID,
		Title:           created.Title,
		AuthoringGuide:  shardAuthoringGuide(resources, t.runtimeAuthoringEnabled()),
		CurrentIndexTSX: starterIndexTSX,
		Citations:       []citationOut{{Kind: "shard", ID: created.ID, Title: created.Title}},
	}, nil
}

func (t docToolServer) deleteFile(ctx context.Context, _ *sdkmcp.CallToolRequest, in deleteFileInput) (*sdkmcp.CallToolResult, deleteFileOutput, error) {
	result, err := t.authoring().DeleteFile(ctx, docsurface.DeleteCommand{PageID: in.PageID, Path: in.Path, Build: in.Build})
	if err != nil {
		return nil, deleteFileOutput{}, err
	}
	return nil, deleteFileOutput{OK: result.OK, Deleted: result.Deleted, Build: result.Build}, nil
}

func (t docToolServer) listDir(ctx context.Context, _ *sdkmcp.CallToolRequest, in listDirInput) (*sdkmcp.CallToolResult, listDirOutput, error) {
	entries, err := t.authoring().ListDir(ctx, in.PageID, in.Path)
	if err != nil {
		return nil, listDirOutput{}, err
	}
	return nil, listDirOutput{Entries: entries}, nil
}

func (t docToolServer) readFile(ctx context.Context, _ *sdkmcp.CallToolRequest, in readFileInput) (*sdkmcp.CallToolResult, readFileOutput, error) {
	data, err := t.authoring().ReadFile(ctx, in.PageID, in.Path)
	if err != nil {
		return nil, readFileOutput{}, err
	}
	return nil, readFileOutput{Content: string(data)}, nil
}

func (t docToolServer) writeFile(ctx context.Context, _ *sdkmcp.CallToolRequest, in writeFileInput) (*sdkmcp.CallToolResult, writeFileOutput, error) {
	result, err := t.authoring().WriteFile(ctx, docsurface.WriteCommand{
		PageID: in.PageID, Path: in.Path, Content: in.Content, Build: in.Build, Overwrite: in.Overwrite,
	})
	if err != nil {
		return nil, writeFileOutput{}, err
	}
	return nil, writeFileOutput{OK: result.OK, Path: result.Path, Build: result.Build}, nil
}

func (t docToolServer) editFile(ctx context.Context, _ *sdkmcp.CallToolRequest, in editFileInput) (*sdkmcp.CallToolResult, editFileOutput, error) {
	result, err := t.authoring().EditFile(ctx, docsurface.EditCommand{
		PageID: in.PageID, Path: in.Path, OldString: in.OldString, NewString: in.NewString,
		ReplaceAll: in.ReplaceAll, Build: in.Build,
	})
	if err != nil {
		return nil, editFileOutput{}, err
	}
	return nil, editFileOutput{OK: result.OK, Path: result.Path, Replacements: result.Replacements, Build: result.Build}, nil
}

var (
	errEditNotFound  = docsurface.ErrEditNotFound
	errEditAmbiguous = docsurface.ErrEditAmbiguous
)

// applyStringEdit performs an exact-string replacement. old_string must occur
// exactly once unless replaceAll is set; absent → errEditNotFound, multiple
// without replaceAll → errEditAmbiguous (with the match count). Returns the new
// content and the number of replacements.
func applyStringEdit(content, oldStr, newStr string, replaceAll bool) (string, int, error) {
	return docsurface.ApplyStringEdit(content, oldStr, newStr, replaceAll)
}

func (t docToolServer) installLib(ctx context.Context, _ *sdkmcp.CallToolRequest, in installLibInput) (*sdkmcp.CallToolResult, installLibOutput, error) {
	libs, err := t.authoring().InstallLib(ctx, in.PageID, in.Name, in.URL)
	if err != nil {
		return nil, installLibOutput{}, err
	}
	return nil, installLibOutput{Libs: libs}, nil
}

func (t docToolServer) buildApp(ctx context.Context, _ *sdkmcp.CallToolRequest, in buildAppInput) (*sdkmcp.CallToolResult, service.BuildResult, error) {
	res, err := t.authoring().Build(ctx, in.PageID, service.ChannelPublished)
	if err != nil {
		return nil, service.BuildResult{}, err
	}
	return nil, res, nil
}

// getAuthoringGuide hands back the current visual and data authoring contract on
// demand. create_app returns it too, but an agent EDITING an existing shard never
// went through create_app. With a page_id it also returns that shard's current
// files, manifest and index.tsx, which is the context an edit actually needs.
func (t docToolServer) getAuthoringGuide(ctx context.Context, _ *sdkmcp.CallToolRequest, in authoringGuideInput) (*sdkmcp.CallToolResult, authoringGuideOutput, error) {
	out := authoringGuideOutput{AuthoringGuide: shardAuthoringGuide(t.resourceAuthoringEnabled(), t.runtimeAuthoringEnabled())}
	if strings.TrimSpace(in.PageID) == "" {
		return nil, out, nil
	}
	context, err := t.publication().AuthoringContext(ctx, in.PageID)
	if err != nil {
		return nil, authoringGuideOutput{}, err
	}
	switch context.Mode {
	case docsurface.AuthoringUnavailable:
		out.AuthoringGuide = unavailableShardGuide
	case docsurface.AuthoringResources:
		out.AuthoringGuide = shardAuthoringGuide(true, t.runtimeAuthoringEnabled())
		if context.ContractMissing {
			out.AuthoringGuide += "\nThe authoring contract file is missing. Restore contract.json from the returned protected contract before building; do not change this shard's storage API.\n"
		}
	default:
		out.AuthoringGuide = shardAuthoringGuide(false, t.runtimeAuthoringEnabled())
	}
	out.ContractJSON, out.Files, out.Anchors, out.IndexTSX = context.Contract, context.Files, context.Anchors, context.IndexTSX
	return nil, out, nil
}

// verifyAppTool is the agent-facing verification pass: the full structured
// report (manifest, refs, per-route mount/anchors/exceptions/console) without
// publishing. Previewed code may mutate draft resources. Run it before publish_app,
// but this one tells you exactly what is wrong while you can still fix it.
func (t docToolServer) verifyAppTool(ctx context.Context, _ *sdkmcp.CallToolRequest, in verifyAppInput) (*sdkmcp.CallToolResult, verifyReport, error) {
	channel := service.ChannelDraft
	if in.Channel == string(service.ChannelPublished) {
		channel = service.ChannelPublished
	}
	report, err := t.publication().Verify(ctx, in.PageID, channel, in.StrictConsole)
	if err != nil {
		return nil, verifyReport{}, err
	}
	return nil, report, nil
}

func (t docToolServer) publishApp(ctx context.Context, _ *sdkmcp.CallToolRequest, in publishAppInput) (*sdkmcp.CallToolResult, publishAppOutput, error) {
	result, err := t.publication().Publish(ctx, in.PageID, in.Summary)
	if err != nil {
		return nil, publishAppOutput{}, err
	}
	return nil, publishAppOutput{
		OK: result.OK, ServedURL: result.ServedURL, Verified: result.Verified, Warning: result.Warning,
		Citations: []citationOut{{Kind: "shard", ID: in.PageID}},
	}, nil
}

// Compatibility wrappers keep focused MCP behavior tests readable while the
// implementation and report types belong to the Doc Surface verification owner.
func (t docToolServer) verifyApp(ctx context.Context, pageID string, channel service.BuildChannel, strictConsole bool, builds ...*service.BuildResult) (verifyReport, error) {
	var built *service.BuildResult
	if len(builds) > 0 {
		built = builds[0]
	}
	return docsurface.NewVerification(t.store, t.preview).Verify(ctx, pageID, channel, strictConsole, built)
}

func verifyFailure(report verifyReport) string {
	return docsurface.FailureSummary(report)
}

func (t docToolServer) manifestAnchorsByRoute(ctx context.Context, pageID string, snapshots ...[]byte) (map[string][]string, []string) {
	return docsurface.NewVerification(t.store, t.preview).ManifestAnchorsByRoute(ctx, pageID, snapshots...)
}

// --- preview handlers ------------------------------------------------------

// previewSummary is a one-line human-readable digest of a preview state, used as
// the text block alongside a screenshot image.
func previewSummary(st service.PreviewState) string {
	return fmt.Sprintf("url=%s mounted=%t console=%d exceptions=%d", st.URL, st.Mounted, len(st.Console), len(st.Exceptions))
}

func (t docToolServer) previewOpen(ctx context.Context, _ *sdkmcp.CallToolRequest, in previewOpenInput) (*sdkmcp.CallToolResult, service.PreviewState, error) {
	channel := service.ChannelDraft
	if in.Channel == string(service.ChannelPublished) {
		channel = service.ChannelPublished
	}
	st, err := t.previewCommands().Open(ctx, in.PageID, channel, in.Theme)
	if err != nil {
		return nil, service.PreviewState{}, err
	}
	return nil, st, nil
}

func (t docToolServer) previewNavigate(ctx context.Context, _ *sdkmcp.CallToolRequest, in previewNavigateInput) (*sdkmcp.CallToolResult, service.PreviewState, error) {
	st, err := t.previewCommands().Navigate(ctx, in.PageID, in.Route)
	if err != nil {
		return nil, service.PreviewState{}, err
	}
	return nil, st, nil
}

func (t docToolServer) previewSnapshot(ctx context.Context, _ *sdkmcp.CallToolRequest, in previewSnapshotInput) (*sdkmcp.CallToolResult, service.PreviewState, error) {
	st, err := t.previewCommands().Snapshot(ctx, in.PageID)
	if err != nil {
		return nil, service.PreviewState{}, err
	}
	return nil, st, nil
}

func (t docToolServer) previewScreenshot(ctx context.Context, _ *sdkmcp.CallToolRequest, in previewScreenshotInput) (*sdkmcp.CallToolResult, service.PreviewState, error) {
	png, st, err := t.previewCommands().Screenshot(ctx, in.PageID)
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
	st, err := t.previewCommands().Eval(ctx, in.PageID, in.Expr)
	if err != nil {
		return nil, service.PreviewState{}, err
	}
	return nil, st, nil
}

func (t docToolServer) previewClick(ctx context.Context, _ *sdkmcp.CallToolRequest, in previewClickInput) (*sdkmcp.CallToolResult, service.PreviewState, error) {
	st, err := t.previewCommands().Click(ctx, in.PageID, in.Selector)
	if err != nil {
		return nil, service.PreviewState{}, err
	}
	return nil, st, nil
}

func (t docToolServer) previewConsole(ctx context.Context, _ *sdkmcp.CallToolRequest, in previewConsoleInput) (*sdkmcp.CallToolResult, service.PreviewState, error) {
	st, err := t.previewCommands().Console(ctx, in.PageID)
	if err != nil {
		return nil, service.PreviewState{}, err
	}
	return nil, st, nil
}

func (t docToolServer) previewClose(ctx context.Context, _ *sdkmcp.CallToolRequest, in previewCloseInput) (*sdkmcp.CallToolResult, previewCloseOutput, error) {
	if err := t.previewCommands().Close(ctx, in.PageID); err != nil {
		return nil, previewCloseOutput{}, err
	}
	return nil, previewCloseOutput{OK: true}, nil
}

func (t docToolServer) previewRestart(ctx context.Context, _ *sdkmcp.CallToolRequest, in previewRestartInput) (*sdkmcp.CallToolResult, previewRestartOutput, error) {
	if err := t.previewCommands().Restart(ctx, in.PageID); err != nil {
		return nil, previewRestartOutput{}, err
	}
	return nil, previewRestartOutput{OK: true}, nil
}
