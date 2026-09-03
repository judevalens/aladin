package mcpserver

import (
	"aladin/backend_v2/internal/artifact"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"aladin/backend_v2/internal/blocknote"
	"aladin/backend_v2/internal/page"
	"aladin/backend_v2/internal/service"
)

// fakeConverter satisfies blocknote.Converter without spinning up Node.
// Each method either returns a canned value (when set) or a deterministic
// derivative of the input.
type fakeConverter struct {
	mdToBlocksFunc      func(markdown string) (json.RawMessage, error)
	blocksToMDFunc      func(blocks json.RawMessage) (string, error)
	blocksToMDBatchFunc func(blocks []json.RawMessage) ([]string, error)
	healthErr           error
}

func (f *fakeConverter) MDToBlocks(_ context.Context, markdown string) (json.RawMessage, error) {
	if f.mdToBlocksFunc != nil {
		return f.mdToBlocksFunc(markdown)
	}
	return json.RawMessage(`[{"id":"new","type":"paragraph","content":[{"type":"text","text":"` + markdown + `"}],"children":[]}]`), nil
}
func (f *fakeConverter) BlocksToMD(_ context.Context, blocks json.RawMessage) (string, error) {
	if f.blocksToMDFunc != nil {
		return f.blocksToMDFunc(blocks)
	}
	return "(blocks)", nil
}
func (f *fakeConverter) BlocksToMDBatch(_ context.Context, blocks []json.RawMessage) ([]string, error) {
	if f.blocksToMDBatchFunc != nil {
		return f.blocksToMDBatchFunc(blocks)
	}
	out := make([]string, len(blocks))
	for i := range blocks {
		out[i] = "rendered"
	}
	return out, nil
}
func (f *fakeConverter) Healthz(_ context.Context) error { return f.healthErr }

type fakePageDocService struct {
	replaceBlock func(pageID, blockID string, replacement json.RawMessage, expectedRev int64) (int64, int, error)
	insertBlocks func(pageID string, position page.BlockPosition, blocks json.RawMessage, expectedRev int64) (int64, []string, error)
	deleteBlock  func(pageID, blockID string, expectedRev int64) (int64, error)
}

func (f *fakePageDocService) GetBlocks(context.Context, string) (json.RawMessage, int64, error) {
	return nil, 0, service.ErrNotFound
}
func (f *fakePageDocService) ReplaceAll(context.Context, string, json.RawMessage, int64) (int64, error) {
	return 0, page.ErrNotImplemented
}
func (f *fakePageDocService) ReplaceBlock(_ context.Context, pageID, blockID string, replacement json.RawMessage, expectedRev int64) (int64, int, error) {
	if f.replaceBlock == nil {
		return 0, 0, page.ErrNotImplemented
	}
	return f.replaceBlock(pageID, blockID, replacement, expectedRev)
}
func (f *fakePageDocService) InsertBlocks(_ context.Context, pageID string, position page.BlockPosition, blocks json.RawMessage, expectedRev int64) (int64, []string, error) {
	if f.insertBlocks == nil {
		return 0, nil, page.ErrNotImplemented
	}
	return f.insertBlocks(pageID, position, blocks, expectedRev)
}
func (f *fakePageDocService) DeleteBlock(_ context.Context, pageID, blockID string, expectedRev int64) (int64, error) {
	if f.deleteBlock == nil {
		return 0, page.ErrNotImplemented
	}
	return f.deleteBlock(pageID, blockID, expectedRev)
}

// fakeBridge satisfies blocknote.Bridge without standing up the sidecar. It
// records the last op applied and returns canned results.
type fakeBridge struct {
	applyFunc func(op blocknote.BridgeOp) (blocknote.BridgeResult, error)
	getFunc   func(pageID string) (blocknote.BridgePage, error)
	lastOp    blocknote.BridgeOp
}

func (f *fakeBridge) ApplyOperation(_ context.Context, op blocknote.BridgeOp) (blocknote.BridgeResult, error) {
	f.lastOp = op
	if f.applyFunc != nil {
		return f.applyFunc(op)
	}
	return blocknote.BridgeResult{BlockIDs: []string{"x"}, AffectedBlockIDs: []string{"x"}, Markdown: "(md)"}, nil
}

func (f *fakeBridge) GetPage(_ context.Context, pageID string) (blocknote.BridgePage, error) {
	if f.getFunc != nil {
		return f.getFunc(pageID)
	}
	return blocknote.BridgePage{Blocks: json.RawMessage("[]")}, nil
}

func newToolServer(artifacts artifact.ArtifactService) toolServer {
	return toolServer{
		artifacts: artifacts,
		pages:     &fakePageDocService{},
		converter: &fakeConverter{},
		bridge:    &fakeBridge{},
	}
}

func TestPageToolsEnforceScopes(t *testing.T) {
	t.Parallel()

	artifacts := &fakeArtifactService{
		getResult: artifact.ArtifactResponse{
			ID:       "page-1",
			Type:     "page",
			Title:    "Page",
			Blocks:   json.RawMessage(`[{"id":"a","type":"paragraph"}]`),
			Metadata: map[string]any{},
		},
	}
	tools := newToolServer(artifacts)
	readOnlyCtx := contextWithScopes(service.ScopeArtifactsRead)

	if _, _, err := tools.getPage(readOnlyCtx, nil, getPageInput{ID: "page-1"}); err != nil {
		t.Fatalf("getPage with read scope error: %v", err)
	}
	if _, _, err := tools.createPage(readOnlyCtx, nil, createPageInput{Title: "Draft", Markdown: "body"}); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("createPage with read scope error = %v, want forbidden", err)
	}
	if _, _, err := tools.updatePage(readOnlyCtx, nil, updatePageInput{ID: "page-1", Title: stringPtr("Next")}); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("updatePage with read scope error = %v, want forbidden", err)
	}
}

func TestCreatePage_ConvertsMarkdownToBlocks(t *testing.T) {
	t.Parallel()

	expected := json.RawMessage(`[{"id":"a","type":"heading"}]`)
	artifacts := &fakeArtifactService{}
	bridge := &fakeBridge{}
	tools := toolServer{
		artifacts: artifacts,
		pages:     &fakePageDocService{},
		converter: &fakeConverter{
			mdToBlocksFunc: func(_ string) (json.RawMessage, error) { return expected, nil },
		},
		bridge: bridge,
	}
	writeCtx := contextWithScopes(service.ScopeArtifactsRead, service.ScopeArtifactsWrite)

	_, out, err := tools.createPage(writeCtx, nil, createPageInput{
		Title:    "Hello",
		Markdown: "# Hello",
		Agent:    &agentInput{Name: "Claude Code", ID: "claude-code"},
	})
	if err != nil {
		t.Fatalf("createPage: %v", err)
	}
	if out.ID == "" {
		t.Fatal("expected non-empty page id")
	}
	// M8c: content is seeded into the Y.Doc via a replace_all bridge op, not
	// written to the artifact create payload.
	if len(artifacts.createPayload.Blocks) != 0 {
		t.Fatalf("create payload should carry no blocks, got %s", string(artifacts.createPayload.Blocks))
	}
	if bridge.lastOp.Op != "replace_all" || string(bridge.lastOp.Blocks) != string(expected) {
		t.Fatalf("bridge op = %+v, want replace_all with %s", bridge.lastOp, string(expected))
	}
	agent, _ := artifacts.createPayload.Metadata["agent"].(map[string]any)
	if agent["source"] != "mcp" || agent["name"] != "Claude Code" {
		t.Fatalf("agent metadata = %#v, want mcp/Claude Code", agent)
	}
}

func TestCreatePage_RequiresTitle(t *testing.T) {
	t.Parallel()
	tools := newToolServer(&fakeArtifactService{})
	writeCtx := contextWithScopes(service.ScopeArtifactsWrite)
	_, _, err := tools.createPage(writeCtx, nil, createPageInput{Markdown: "body"})
	var requestErr service.BadRequest
	if !errors.As(err, &requestErr) {
		t.Fatalf("createPage error = %v, want BadRequest", err)
	}
}

func TestUpdatePage_AcceptsMarkdownAndMetadata(t *testing.T) {
	t.Parallel()

	artifacts := &fakeArtifactService{
		getResult: artifact.ArtifactResponse{
			ID:       "page-1",
			Type:     "page",
			Title:    "Original",
			Blocks:   json.RawMessage(`[{"id":"a","type":"paragraph"}]`),
			Metadata: map[string]any{"kept": true},
		},
		updateResult: artifact.ArtifactResponse{
			ID:        "page-1",
			Type:      "page",
			Title:     "Updated",
			Revision:  4,
			UpdatedAt: "2026-05-27T00:00:00Z",
		},
	}
	bridge := &fakeBridge{}
	tools := toolServer{artifacts: artifacts, pages: &fakePageDocService{}, converter: &fakeConverter{}, bridge: bridge}
	writeCtx := contextWithScopes(service.ScopeArtifactsRead, service.ScopeArtifactsWrite)

	md := "new body"
	_, out, err := tools.updatePage(writeCtx, nil, updatePageInput{
		ID:       "page-1",
		Markdown: &md,
		Tags:     []string{"research", "  "},
		Agent:    &agentInput{Name: "Claude"},
	})
	if err != nil {
		t.Fatalf("updatePage: %v", err)
	}
	if out.Revision != 4 {
		t.Fatalf("revision = %d, want 4", out.Revision)
	}
	// M8c: content goes through the bridge (replace_all); the artifact patch
	// carries metadata only, never blocks.
	if bridge.lastOp.Op != "replace_all" {
		t.Fatalf("bridge op = %q, want replace_all", bridge.lastOp.Op)
	}
	if artifacts.updatePatch.Blocks != nil {
		t.Fatal("update patch must not carry blocks (they go via the bridge)")
	}
	if artifacts.updatePatch.Metadata == nil {
		t.Fatal("update patch missing metadata")
	}
	tags, _ := (*artifacts.updatePatch.Metadata)["tags"].([]string)
	if len(tags) != 1 || tags[0] != "research" {
		t.Fatalf("tags = %v, want [research]", tags)
	}
}

func TestUpdatePage_RequiresAField(t *testing.T) {
	t.Parallel()
	tools := newToolServer(&fakeArtifactService{
		getResult: artifact.ArtifactResponse{ID: "p", Type: "page", Title: "P"},
	})
	writeCtx := contextWithScopes(service.ScopeArtifactsRead, service.ScopeArtifactsWrite)
	_, _, err := tools.updatePage(writeCtx, nil, updatePageInput{ID: "p"})
	var requestErr service.BadRequest
	if !errors.As(err, &requestErr) {
		t.Fatalf("updatePage with no fields error = %v, want BadRequest", err)
	}
}

func TestGetPage_ReturnsBlocksWithMarkdown(t *testing.T) {
	t.Parallel()

	blocks := json.RawMessage(`[{"id":"a","type":"heading","props":{"level":1}},{"id":"b","type":"paragraph"}]`)
	artifacts := &fakeArtifactService{
		getResult: artifact.ArtifactResponse{
			ID:       "page-1",
			Type:     "page",
			Title:    "Page",
			Metadata: map[string]any{},
		},
	}
	// M8c: getPage reads fresh blocks from the live Y.Doc via the bridge.
	bridge := &fakeBridge{
		getFunc: func(string) (blocknote.BridgePage, error) {
			return blocknote.BridgePage{Blocks: blocks}, nil
		},
	}
	tools := toolServer{
		artifacts: artifacts,
		pages:     &fakePageDocService{},
		converter: &fakeConverter{
			blocksToMDBatchFunc: func(in []json.RawMessage) ([]string, error) {
				out := make([]string, len(in))
				for i := range in {
					out[i] = "rendered-" + string('a'+byte(i))
				}
				return out, nil
			},
		},
		bridge: bridge,
	}
	readCtx := contextWithScopes(service.ScopeArtifactsRead)

	_, out, err := tools.getPage(readCtx, nil, getPageInput{ID: "page-1"})
	if err != nil {
		t.Fatalf("getPage: %v", err)
	}
	if len(out.Page.Blocks) != 2 {
		t.Fatalf("blocks = %#v, want 2", out.Page.Blocks)
	}
	if out.Page.Blocks[0].ID != "a" || out.Page.Blocks[0].Type != "heading" {
		t.Fatalf("first block = %#v", out.Page.Blocks[0])
	}
	if !strings.HasPrefix(out.Page.Blocks[0].Markdown, "rendered-") {
		t.Fatalf("first block markdown = %q, want rendered-*", out.Page.Blocks[0].Markdown)
	}
	if out.Page.Blocks[0].Props["level"] != float64(1) {
		t.Fatalf("props = %#v, want level:1", out.Page.Blocks[0].Props)
	}
}

func TestUpdateBlock_RoundTrip(t *testing.T) {
	t.Parallel()

	bridge := &fakeBridge{
		applyFunc: func(blocknote.BridgeOp) (blocknote.BridgeResult, error) {
			return blocknote.BridgeResult{AffectedBlockIDs: []string{"block-1"}, Markdown: "## New"}, nil
		},
	}
	tools := toolServer{
		artifacts: &fakeArtifactService{},
		pages:     &fakePageDocService{},
		converter: &fakeConverter{
			mdToBlocksFunc: func(_ string) (json.RawMessage, error) {
				return json.RawMessage(`[{"id":"x","type":"heading"}]`), nil
			},
		},
		bridge: bridge,
	}
	writeCtx := contextWithScopes(service.ScopeArtifactsWrite)

	_, out, err := tools.updateBlock(writeCtx, nil, updateBlockInput{
		PageID:   "page-1",
		BlockID:  "block-1",
		Markdown: "## New",
	})
	if err != nil {
		t.Fatalf("updateBlock: %v", err)
	}
	if bridge.lastOp.Op != "replace_block" || bridge.lastOp.PageID != "page-1" || bridge.lastOp.BlockID != "block-1" {
		t.Fatalf("bridge op = %+v, want replace_block page-1/block-1", bridge.lastOp)
	}
	if !strings.Contains(string(bridge.lastOp.Blocks), `"type":"heading"`) {
		t.Fatalf("op blocks = %s, want heading", string(bridge.lastOp.Blocks))
	}
	// The original id is carried onto the first produced block.
	if !strings.Contains(string(bridge.lastOp.Blocks), `"block-1"`) {
		t.Fatalf("op blocks = %s, want first block id block-1", string(bridge.lastOp.Blocks))
	}
	if out.ReplacedBlockCount != 1 {
		t.Fatalf("count = %d, want 1", out.ReplacedBlockCount)
	}
	if out.Markdown != "## New" {
		t.Fatalf("markdown = %q, want '## New'", out.Markdown)
	}
}

func TestInsertBlocks_ReturnsInsertedIDs(t *testing.T) {
	t.Parallel()
	bridge := &fakeBridge{
		applyFunc: func(blocknote.BridgeOp) (blocknote.BridgeResult, error) {
			return blocknote.BridgeResult{AffectedBlockIDs: []string{"x", "y"}, Markdown: "* x\n* y"}, nil
		},
	}
	tools := toolServer{
		artifacts: &fakeArtifactService{},
		pages:     &fakePageDocService{},
		converter: &fakeConverter{},
		bridge:    bridge,
	}
	writeCtx := contextWithScopes(service.ScopeArtifactsWrite)

	after := "a"
	_, out, err := tools.insertBlocks(writeCtx, nil, insertBlocksInput{
		PageID:   "page-1",
		Position: insertBlocksPositionDTO{AfterID: &after},
		Markdown: "* x\n* y",
	})
	if err != nil {
		t.Fatalf("insertBlocks: %v", err)
	}
	if bridge.lastOp.Op != "insert_blocks" || bridge.lastOp.BlockID != "a" || bridge.lastOp.Placement != "after" {
		t.Fatalf("bridge op = %+v, want insert_blocks after a", bridge.lastOp)
	}
	if len(out.InsertedBlockIDs) != 2 {
		t.Fatalf("out = %#v, want 2 inserted ids", out)
	}
}

func TestDeleteBlock_BypassesConverter(t *testing.T) {
	t.Parallel()
	bridge := &fakeBridge{}
	tools := toolServer{
		artifacts: &fakeArtifactService{},
		pages:     &fakePageDocService{},
		converter: nil, // delete_block should not require a converter
		bridge:    bridge,
	}
	writeCtx := contextWithScopes(service.ScopeArtifactsWrite)

	_, out, err := tools.deleteBlock(writeCtx, nil, deleteBlockInput{
		PageID:  "page-1",
		BlockID: "block-1",
	})
	if err != nil {
		t.Fatalf("deleteBlock: %v", err)
	}
	if bridge.lastOp.Op != "delete_block" || bridge.lastOp.BlockID != "block-1" {
		t.Fatalf("bridge op = %+v, want delete_block block-1", bridge.lastOp)
	}
	if !out.Deleted {
		t.Fatalf("out = %#v, want deleted", out)
	}
}

func TestPageToolsRejectNonPageArtifacts(t *testing.T) {
	t.Parallel()

	tools := newToolServer(&fakeArtifactService{
		getResult: artifact.ArtifactResponse{ID: "link-1", Type: "link", Title: "Link", Metadata: map[string]any{}},
	})
	readCtx := contextWithScopes(service.ScopeArtifactsRead, service.ScopeArtifactsWrite)

	if _, _, err := tools.getPage(readCtx, nil, getPageInput{ID: "link-1"}); err == nil {
		t.Fatal("getPage non-page error = nil, want error")
	}
	if _, _, err := tools.updatePage(readCtx, nil, updatePageInput{ID: "link-1", Title: stringPtr("Next")}); err == nil {
		t.Fatal("updatePage non-page error = nil, want error")
	}
}

func TestSearchPagesValidatesAndClampsLimit(t *testing.T) {
	t.Parallel()

	artifacts := &fakeArtifactService{
		searchResults: []artifact.ArtifactResponse{
			{ID: "page-1", Type: "page", Title: "Match", Metadata: map[string]any{}},
			{ID: "link-1", Type: "link", Title: "Skip", Metadata: map[string]any{}},
		},
	}
	tools := newToolServer(artifacts)
	readCtx := contextWithScopes(service.ScopeArtifactsRead)

	if _, _, err := tools.searchPages(readCtx, nil, searchPagesInput{Query: "   "}); err == nil {
		t.Fatal("searchPages empty query error = nil, want error")
	}
	_, out, err := tools.searchPages(readCtx, nil, searchPagesInput{Query: "agent", Limit: 500})
	if err != nil {
		t.Fatalf("searchPages error: %v", err)
	}
	if artifacts.searchParams == nil || artifacts.searchParams.Limit != 50 {
		t.Fatalf("search params = %#v, want limit clamped to 50", artifacts.searchParams)
	}
	if len(out.Pages) != 1 || out.Pages[0].ID != "page-1" {
		t.Fatalf("pages = %#v, want only page results", out.Pages)
	}
}

func TestFolderToolsCreateRenameAndMoveArtifact(t *testing.T) {
	t.Parallel()

	parentID := "folder-parent"
	targetID := "folder-target"
	artifacts := &fakeArtifactService{
		createFolderResult: artifact.FolderNode{ID: "folder-created", ParentID: &parentID, Title: "Planning", Seq: 2},
		updateFolderResult: artifact.FolderNode{ID: targetID, Title: "Docs", Seq: 3},
		moveArtifactResult: artifact.ArtifactResponse{
			ID:        "artifact-1",
			Type:      "app",
			Title:     "Dashboard",
			FolderID:  &targetID,
			UpdatedAt: "2026-08-27T00:00:00Z",
			Seq:       4,
		},
	}
	tools := newToolServer(artifacts)
	writeCtx := contextWithScopes(service.ScopeArtifactsRead, service.ScopeArtifactsWrite)

	_, created, err := tools.createFolder(writeCtx, nil, createFolderInput{Title: "Planning", ParentID: &parentID})
	if err != nil {
		t.Fatalf("createFolder: %v", err)
	}
	if created.Folder.ID != "folder-created" || created.Folder.ParentID == nil || *created.Folder.ParentID != parentID {
		t.Fatalf("created folder = %#v, want parent %q", created.Folder, parentID)
	}
	if artifacts.createFolderTitle != "Planning" || artifacts.createFolderParentID == nil || *artifacts.createFolderParentID != parentID {
		t.Fatalf("create folder args title=%q parent=%v", artifacts.createFolderTitle, artifacts.createFolderParentID)
	}

	_, renamed, err := tools.renameFolder(writeCtx, nil, renameFolderInput{ID: targetID, Title: "Docs"})
	if err != nil {
		t.Fatalf("renameFolder: %v", err)
	}
	if renamed.Folder.ID != targetID || renamed.Folder.Title != "Docs" {
		t.Fatalf("renamed folder = %#v, want %q/Docs", renamed.Folder, targetID)
	}
	if artifacts.updateFolderID != targetID || artifacts.updateFolderPatch.Title == nil || *artifacts.updateFolderPatch.Title != "Docs" {
		t.Fatalf("rename args id=%q patch=%#v", artifacts.updateFolderID, artifacts.updateFolderPatch)
	}

	_, moved, err := tools.moveArtifact(writeCtx, nil, moveArtifactInput{ArtifactID: "artifact-1", FolderID: &targetID})
	if err != nil {
		t.Fatalf("moveArtifact: %v", err)
	}
	if moved.ID != "artifact-1" || moved.FolderID == nil || *moved.FolderID != targetID || moved.Citations[0].Kind != "shard" {
		t.Fatalf("moved artifact = %#v, want artifact in %q with shard citation", moved, targetID)
	}
	if artifacts.moveArtifactID != "artifact-1" || artifacts.moveArtifactFolderID == nil || *artifacts.moveArtifactFolderID != targetID {
		t.Fatalf("move args id=%q folder=%v", artifacts.moveArtifactID, artifacts.moveArtifactFolderID)
	}
}

func TestFolderToolOutputsEncodeSeqAsNumber(t *testing.T) {
	t.Parallel()

	payloads := []any{
		folderActionOutput{Folder: folderOutput{ID: "folder-1", Title: "Docs", Seq: 2}},
		movedArtifactOutput{ID: "artifact-1", Title: "Doc", Type: "page", UpdatedAt: "2026-08-27T00:00:00Z", Seq: 3},
	}
	for _, payload := range payloads {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		if folder, ok := decoded["folder"].(map[string]any); ok {
			decoded = folder
		}
		if _, ok := decoded["seq"].(float64); !ok {
			t.Fatalf("seq encoded as %T in %s, want JSON number", decoded["seq"], raw)
		}
	}
}

func TestMoveArtifactCanMoveToRoot(t *testing.T) {
	t.Parallel()

	artifacts := &fakeArtifactService{
		moveArtifactResult: artifact.ArtifactResponse{ID: "artifact-1", Type: "page", Title: "Doc", UpdatedAt: "2026-08-27T00:00:00Z"},
	}
	tools := newToolServer(artifacts)
	writeCtx := contextWithScopes(service.ScopeArtifactsWrite)

	_, out, err := tools.moveArtifact(writeCtx, nil, moveArtifactInput{ArtifactID: "artifact-1"})
	if err != nil {
		t.Fatalf("moveArtifact root: %v", err)
	}
	if out.FolderID != nil {
		t.Fatalf("folder id = %v, want root", out.FolderID)
	}
	if artifacts.moveArtifactID != "artifact-1" || artifacts.moveArtifactFolderID != nil {
		t.Fatalf("move args id=%q folder=%v, want nil folder", artifacts.moveArtifactID, artifacts.moveArtifactFolderID)
	}
}

func TestFolderToolsEnforceWriteScope(t *testing.T) {
	t.Parallel()

	tools := newToolServer(&fakeArtifactService{})
	readOnlyCtx := contextWithScopes(service.ScopeArtifactsRead)

	if _, _, err := tools.createFolder(readOnlyCtx, nil, createFolderInput{Title: "Docs"}); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("createFolder read-only error = %v, want forbidden", err)
	}
	if _, _, err := tools.renameFolder(readOnlyCtx, nil, renameFolderInput{ID: "folder-1", Title: "Docs"}); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("renameFolder read-only error = %v, want forbidden", err)
	}
	if _, _, err := tools.moveArtifact(readOnlyCtx, nil, moveArtifactInput{ArtifactID: "artifact-1"}); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("moveArtifact read-only error = %v, want forbidden", err)
	}
}

func testPrincipal(scopes ...string) service.Principal {
	return service.Principal{
		UserID:    "user-1",
		ActorType: service.ActorTypeIntegrationToken,
		ActorID:   "token-1",
		Email:     "user@example.com",
		Scopes:    scopes,
	}
}

func contextWithScopes(scopes ...string) context.Context {
	return service.WithPrincipal(context.Background(), testPrincipal(scopes...))
}

func stringPtr(value string) *string {
	return &value
}

type fakeArtifactService struct {
	list                 []artifact.ArtifactResponse
	searchResults        []artifact.ArtifactResponse
	browserTree          []artifact.BrowserTreeNode
	folders              []artifact.FolderNode
	getResult            artifact.ArtifactResponse
	createResult         artifact.ArtifactCreateResponse
	updateResult         artifact.ArtifactResponse
	createFolderResult   artifact.FolderNode
	updateFolderResult   artifact.FolderNode
	moveArtifactResult   artifact.ArtifactResponse
	createPayload        artifact.ArtifactPayload
	updatePatch          artifact.ArtifactPatch
	updateFolderPatch    artifact.FolderPatch
	searchParams         *artifact.PageSearchParams
	createFolderTitle    string
	createFolderParentID *string
	updateFolderID       string
	moveArtifactID       string
	moveArtifactFolderID *string
	err                  error
}

func (f *fakeArtifactService) QueryByProperty(context.Context, artifact.PropertyQuery) ([]artifact.ArtifactResponse, error) {
	return nil, nil
}

func (f *fakeArtifactService) PropertyFacets(context.Context) ([]artifact.PropertyFacet, error) {
	return nil, nil
}

func (f *fakeArtifactService) List(ctx context.Context, _ artifact.ArtifactListParams) ([]artifact.ArtifactResponse, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsRead); err != nil {
		return nil, err
	}
	return f.list, f.err
}

func (f *fakeArtifactService) SearchPages(ctx context.Context, params artifact.PageSearchParams) ([]artifact.ArtifactResponse, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsRead); err != nil {
		return nil, err
	}
	copyParams := params
	f.searchParams = &copyParams
	return f.searchResults, f.err
}

func (f *fakeArtifactService) BrowserTree(ctx context.Context) ([]artifact.BrowserTreeNode, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsRead); err != nil {
		return nil, err
	}
	return f.browserTree, f.err
}

func (f *fakeArtifactService) DeleteBrowserNode(context.Context, string) (artifact.NodeDeleteResult, error) {
	return artifact.NodeDeleteResult{}, nil
}

func (f *fakeArtifactService) Get(ctx context.Context, _ string) (artifact.ArtifactResponse, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsRead); err != nil {
		return artifact.ArtifactResponse{}, err
	}
	return f.getResult, f.err
}

func (f *fakeArtifactService) Create(ctx context.Context, payload artifact.ArtifactPayload) (artifact.ArtifactCreateResponse, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsWrite); err != nil {
		return artifact.ArtifactCreateResponse{}, err
	}
	f.createPayload = payload
	if f.createResult.Artifact.ID != "" {
		return f.createResult, f.err
	}
	return artifact.ArtifactCreateResponse{
		Artifact: artifact.ArtifactResponse{
			ID:       "page-created",
			Type:     payload.Type,
			Title:    payload.Title,
			FolderID: payload.FolderID,
			Metadata: payload.Metadata,
			Blocks:   payload.Blocks,
		},
	}, f.err
}

func (f *fakeArtifactService) Update(ctx context.Context, _ string, patch artifact.ArtifactPatch) (artifact.ArtifactResponse, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsWrite); err != nil {
		return artifact.ArtifactResponse{}, err
	}
	f.updatePatch = patch
	if f.updateResult.ID != "" {
		return f.updateResult, f.err
	}
	return f.getResult, f.err
}

func (f *fakeArtifactService) MoveArtifact(ctx context.Context, id string, folderID *string) (artifact.ArtifactResponse, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsWrite); err != nil {
		return artifact.ArtifactResponse{}, err
	}
	f.moveArtifactID = id
	f.moveArtifactFolderID = folderID
	if f.moveArtifactResult.ID != "" {
		return f.moveArtifactResult, f.err
	}
	rec := f.getResult
	rec.FolderID = folderID
	return rec, f.err
}

func (f *fakeArtifactService) Delete(context.Context, string) (artifact.NodeDeleteResult, error) {
	return artifact.NodeDeleteResult{}, service.ErrForbidden
}

func (f *fakeArtifactService) Upload(context.Context, artifact.ArtifactUploadInput, io.Reader) (artifact.ArtifactResponse, error) {
	return artifact.ArtifactResponse{}, service.ErrForbidden
}

func (f *fakeArtifactService) Resource(context.Context, string) (artifact.ArtifactResource, error) {
	return artifact.ArtifactResource{}, service.ErrForbidden
}

func (f *fakeArtifactService) ListFolders(ctx context.Context, _ *string) ([]artifact.FolderNode, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsRead); err != nil {
		return nil, err
	}
	return f.folders, f.err
}

func (f *fakeArtifactService) FolderTree(context.Context) ([]artifact.FolderTreeNode, error) {
	return nil, nil
}

func (f *fakeArtifactService) CreateFolder(ctx context.Context, title string, parentID *string) (artifact.FolderNode, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsWrite); err != nil {
		return artifact.FolderNode{}, err
	}
	f.createFolderTitle = title
	f.createFolderParentID = parentID
	if f.createFolderResult.ID != "" {
		return f.createFolderResult, f.err
	}
	return artifact.FolderNode{ID: "folder-created", ParentID: parentID, Title: title, Seq: 1}, f.err
}

func (f *fakeArtifactService) UpdateFolder(ctx context.Context, id string, patch artifact.FolderPatch) (artifact.FolderNode, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsWrite); err != nil {
		return artifact.FolderNode{}, err
	}
	f.updateFolderID = id
	f.updateFolderPatch = patch
	if f.updateFolderResult.ID != "" {
		return f.updateFolderResult, f.err
	}
	title := ""
	if patch.Title != nil {
		title = *patch.Title
	}
	return artifact.FolderNode{ID: id, Title: title, Seq: 1}, f.err
}

func (f *fakeArtifactService) GetFolder(context.Context, string) (artifact.FolderNode, error) {
	return artifact.FolderNode{}, service.ErrNotFound
}

func (f *fakeArtifactService) FolderBreadcrumbs(context.Context, string) ([]artifact.BreadcrumbItem, error) {
	return nil, nil
}

func (f *fakeArtifactService) CreateBrowserNode(context.Context, artifact.BrowserNodeCreateInput) (artifact.BrowserNodeCreateResponse, error) {
	return artifact.BrowserNodeCreateResponse{}, service.ErrForbidden
}
