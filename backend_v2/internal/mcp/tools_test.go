package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

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
	insertBlocks func(pageID string, position service.BlockPosition, blocks json.RawMessage, expectedRev int64) (int64, []string, error)
	deleteBlock  func(pageID, blockID string, expectedRev int64) (int64, error)
}

func (f *fakePageDocService) GetBlocks(context.Context, string) (json.RawMessage, int64, error) {
	return nil, 0, service.ErrNotFound
}
func (f *fakePageDocService) ReplaceAll(context.Context, string, json.RawMessage, int64) (int64, error) {
	return 0, service.ErrNotImplemented
}
func (f *fakePageDocService) ReplaceBlock(_ context.Context, pageID, blockID string, replacement json.RawMessage, expectedRev int64) (int64, int, error) {
	if f.replaceBlock == nil {
		return 0, 0, service.ErrNotImplemented
	}
	return f.replaceBlock(pageID, blockID, replacement, expectedRev)
}
func (f *fakePageDocService) InsertBlocks(_ context.Context, pageID string, position service.BlockPosition, blocks json.RawMessage, expectedRev int64) (int64, []string, error) {
	if f.insertBlocks == nil {
		return 0, nil, service.ErrNotImplemented
	}
	return f.insertBlocks(pageID, position, blocks, expectedRev)
}
func (f *fakePageDocService) DeleteBlock(_ context.Context, pageID, blockID string, expectedRev int64) (int64, error) {
	if f.deleteBlock == nil {
		return 0, service.ErrNotImplemented
	}
	return f.deleteBlock(pageID, blockID, expectedRev)
}

func newToolServer(artifacts service.ArtifactService) toolServer {
	return toolServer{
		artifacts: artifacts,
		pages:     &fakePageDocService{},
		converter: &fakeConverter{},
	}
}

func TestPageToolsEnforceScopes(t *testing.T) {
	t.Parallel()

	artifacts := &fakeArtifactService{
		getResult: service.ArtifactResponse{
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
	tools := toolServer{
		artifacts: artifacts,
		pages:     &fakePageDocService{},
		converter: &fakeConverter{
			mdToBlocksFunc: func(_ string) (json.RawMessage, error) { return expected, nil },
		},
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
	if string(artifacts.createPayload.Blocks) != string(expected) {
		t.Fatalf("artifact create payload blocks = %s, want %s", string(artifacts.createPayload.Blocks), string(expected))
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
		getResult: service.ArtifactResponse{
			ID:       "page-1",
			Type:     "page",
			Title:    "Original",
			Blocks:   json.RawMessage(`[{"id":"a","type":"paragraph"}]`),
			Metadata: map[string]any{"kept": true},
		},
		updateResult: service.ArtifactResponse{
			ID:        "page-1",
			Type:      "page",
			Title:     "Updated",
			Revision:  4,
			UpdatedAt: "2026-05-27T00:00:00Z",
		},
	}
	tools := newToolServer(artifacts)
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
	if artifacts.updatePatch.Blocks == nil {
		t.Fatal("update patch missing blocks")
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
		getResult: service.ArtifactResponse{ID: "p", Type: "page", Title: "P"},
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

	artifacts := &fakeArtifactService{
		getResult: service.ArtifactResponse{
			ID:       "page-1",
			Type:     "page",
			Title:    "Page",
			Blocks:   json.RawMessage(`[{"id":"a","type":"heading","props":{"level":1}},{"id":"b","type":"paragraph"}]`),
			Metadata: map[string]any{},
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
	if string(out.Page.Blocks[0].Props) != `{"level":1}` {
		t.Fatalf("props = %s, want level:1", string(out.Page.Blocks[0].Props))
	}
}

func TestUpdateBlock_RoundTrip(t *testing.T) {
	t.Parallel()

	var (
		gotPageID  string
		gotBlockID string
		gotBlocks  json.RawMessage
	)
	pages := &fakePageDocService{
		replaceBlock: func(pageID, blockID string, replacement json.RawMessage, _ int64) (int64, int, error) {
			gotPageID = pageID
			gotBlockID = blockID
			gotBlocks = replacement
			return 7, 1, nil
		},
	}
	tools := toolServer{
		artifacts: &fakeArtifactService{},
		pages:     pages,
		converter: &fakeConverter{
			mdToBlocksFunc: func(_ string) (json.RawMessage, error) {
				return json.RawMessage(`[{"id":"x","type":"heading"}]`), nil
			},
		},
	}
	writeCtx := contextWithScopes(service.ScopeArtifactsWrite)

	_, out, err := tools.updateBlock(writeCtx, nil, updateBlockInput{
		PageID:  "page-1",
		BlockID: "block-1",
		Markdown: "## New",
	})
	if err != nil {
		t.Fatalf("updateBlock: %v", err)
	}
	if gotPageID != "page-1" || gotBlockID != "block-1" {
		t.Fatalf("page/block = %q/%q, want page-1/block-1", gotPageID, gotBlockID)
	}
	if !strings.Contains(string(gotBlocks), `"type":"heading"`) {
		t.Fatalf("converter output = %s, want heading", string(gotBlocks))
	}
	if out.Revision != 7 {
		t.Fatalf("revision = %d, want 7", out.Revision)
	}
	if out.ReplacedBlockCount != 1 {
		t.Fatalf("count = %d, want 1", out.ReplacedBlockCount)
	}
}

func TestInsertBlocks_ReturnsInsertedIDs(t *testing.T) {
	t.Parallel()
	pages := &fakePageDocService{
		insertBlocks: func(_ string, _ service.BlockPosition, _ json.RawMessage, _ int64) (int64, []string, error) {
			return 3, []string{"x", "y"}, nil
		},
	}
	tools := toolServer{
		artifacts: &fakeArtifactService{},
		pages:     pages,
		converter: &fakeConverter{},
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
	if out.Revision != 3 || len(out.InsertedBlockIDs) != 2 {
		t.Fatalf("out = %#v", out)
	}
}

func TestDeleteBlock_BypassesConverter(t *testing.T) {
	t.Parallel()
	pages := &fakePageDocService{
		deleteBlock: func(_ string, _ string, _ int64) (int64, error) {
			return 9, nil
		},
	}
	tools := toolServer{
		artifacts: &fakeArtifactService{},
		pages:     pages,
		converter: nil, // delete_block should not require a converter
	}
	writeCtx := contextWithScopes(service.ScopeArtifactsWrite)

	_, out, err := tools.deleteBlock(writeCtx, nil, deleteBlockInput{
		PageID:  "page-1",
		BlockID: "block-1",
	})
	if err != nil {
		t.Fatalf("deleteBlock: %v", err)
	}
	if out.Revision != 9 || !out.Deleted {
		t.Fatalf("out = %#v", out)
	}
}

func TestPageToolsRejectNonPageArtifacts(t *testing.T) {
	t.Parallel()

	tools := newToolServer(&fakeArtifactService{
		getResult: service.ArtifactResponse{ID: "link-1", Type: "link", Title: "Link", Metadata: map[string]any{}},
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
		searchResults: []service.ArtifactResponse{
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
	list          []service.ArtifactResponse
	searchResults []service.ArtifactResponse
	browserTree   []service.BrowserTreeNode
	folders       []service.FolderNode
	getResult     service.ArtifactResponse
	createResult  service.ArtifactCreateResponse
	updateResult  service.ArtifactResponse
	createPayload service.ArtifactPayload
	updatePatch   service.ArtifactPatch
	searchParams  *service.PageSearchParams
	err           error
}

func (f *fakeArtifactService) List(ctx context.Context, _ service.ArtifactListParams) ([]service.ArtifactResponse, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsRead); err != nil {
		return nil, err
	}
	return f.list, f.err
}

func (f *fakeArtifactService) SearchPages(ctx context.Context, params service.PageSearchParams) ([]service.ArtifactResponse, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsRead); err != nil {
		return nil, err
	}
	copyParams := params
	f.searchParams = &copyParams
	return f.searchResults, f.err
}

func (f *fakeArtifactService) BrowserTree(ctx context.Context) ([]service.BrowserTreeNode, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsRead); err != nil {
		return nil, err
	}
	return f.browserTree, f.err
}

func (f *fakeArtifactService) DeleteBrowserNode(context.Context, string) error {
	return nil
}

func (f *fakeArtifactService) Get(ctx context.Context, _ string) (service.ArtifactResponse, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsRead); err != nil {
		return service.ArtifactResponse{}, err
	}
	return f.getResult, f.err
}

func (f *fakeArtifactService) Create(ctx context.Context, payload service.ArtifactPayload) (service.ArtifactCreateResponse, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsWrite); err != nil {
		return service.ArtifactCreateResponse{}, err
	}
	f.createPayload = payload
	if f.createResult.Artifact.ID != "" {
		return f.createResult, f.err
	}
	return service.ArtifactCreateResponse{
		Artifact: service.ArtifactResponse{
			ID:       "page-created",
			Type:     payload.Type,
			Title:    payload.Title,
			FolderID: payload.FolderID,
			Metadata: payload.Metadata,
			Blocks:   payload.Blocks,
		},
	}, f.err
}

func (f *fakeArtifactService) Update(ctx context.Context, _ string, patch service.ArtifactPatch) (service.ArtifactResponse, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsWrite); err != nil {
		return service.ArtifactResponse{}, err
	}
	f.updatePatch = patch
	if f.updateResult.ID != "" {
		return f.updateResult, f.err
	}
	return f.getResult, f.err
}

func (f *fakeArtifactService) Delete(context.Context, string) error {
	return service.ErrForbidden
}

func (f *fakeArtifactService) Upload(context.Context, service.ArtifactUploadInput, io.Reader) (service.ArtifactResponse, error) {
	return service.ArtifactResponse{}, service.ErrForbidden
}

func (f *fakeArtifactService) Resource(context.Context, string) (service.ArtifactResource, error) {
	return service.ArtifactResource{}, service.ErrForbidden
}

func (f *fakeArtifactService) ListFolders(ctx context.Context, _ *string) ([]service.FolderNode, error) {
	if err := service.RequireScope(ctx, service.ScopeArtifactsRead); err != nil {
		return nil, err
	}
	return f.folders, f.err
}

func (f *fakeArtifactService) FolderTree(context.Context) ([]service.FolderTreeNode, error) {
	return nil, nil
}

func (f *fakeArtifactService) CreateFolder(context.Context, string, *string) (service.FolderNode, error) {
	return service.FolderNode{}, service.ErrForbidden
}

func (f *fakeArtifactService) UpdateFolder(context.Context, string, service.FolderPatch) (service.FolderNode, error) {
	return service.FolderNode{}, service.ErrForbidden
}

func (f *fakeArtifactService) GetFolder(context.Context, string) (service.FolderNode, error) {
	return service.FolderNode{}, service.ErrNotFound
}

func (f *fakeArtifactService) FolderBreadcrumbs(context.Context, string) ([]service.BreadcrumbItem, error) {
	return nil, nil
}

func (f *fakeArtifactService) CreateBrowserNode(context.Context, service.BrowserNodeCreateInput) (service.BrowserNodeCreateResponse, error) {
	return service.BrowserNodeCreateResponse{}, service.ErrForbidden
}
