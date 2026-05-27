package mcpserver

import (
	"context"
	"encoding/json"
	"strings"

	"aladin/backend_v2/internal/blocknote"
	"aladin/backend_v2/internal/service"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const pageArtifactType = "page"

type toolServer struct {
	artifacts service.ArtifactService
	pages     service.PageDocumentService
	converter blocknote.Converter
}

func registerTools(server *sdkmcp.Server, artifacts service.ArtifactService, pages service.PageDocumentService, converter blocknote.Converter) {
	tools := toolServer{artifacts: artifacts, pages: pages, converter: converter}

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_browser_tree",
		Description: "Return the Aladin browser tree with folders and artifacts.",
	}, tools.getBrowserTree)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_folders",
		Description: "List immediate child folders for a parent folder, or root folders when omitted.",
	}, tools.listFolders)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "create_page",
		Description: "Create an Aladin page. The body is supplied as markdown and stored as BlockNote blocks; the server handles the conversion.",
	}, tools.createPage)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "update_page",
		Description: "Full-document update of an Aladin page. Provide markdown to replace all blocks (wipes block ids — prefer update_block / insert_blocks / delete_block for surgical edits).",
	}, tools.updatePage)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_page",
		Description: "Read a single Aladin page. Returns each block with its stable id, type, props, and a markdown rendering you can read or pass back to update_block.",
	}, tools.getPage)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_pages",
		Description: "List page summaries in an optional folder.",
	}, tools.listPages)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "search_pages",
		Description: "Search page titles, summaries, and page text content.",
	}, tools.searchPages)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "update_block",
		Description: "Replace a single block by id with new content provided as markdown. The markdown may parse into multiple blocks; the first one inherits the original id.",
	}, tools.updateBlock)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "insert_blocks",
		Description: "Insert one or more blocks at a position. Provide markdown for the new content. position is one of {after_id}, {before_id}, or {at: \"start\"|\"end\"}.",
	}, tools.insertBlocks)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "delete_block",
		Description: "Delete a block by id. Refuses to remove the last block on a page.",
	}, tools.deleteBlock)
}

type emptyInput struct{}

type listFoldersInput struct {
	ParentID *string `json:"parent_id,omitempty"`
}

type createPageInput struct {
	Title    string      `json:"title"`
	Markdown string      `json:"markdown"`
	FolderID *string     `json:"folder_id,omitempty"`
	Summary  *string     `json:"summary,omitempty"`
	Tags     []string    `json:"tags,omitempty"`
	Agent    *agentInput `json:"agent,omitempty"`
}

type updatePageInput struct {
	ID       string      `json:"id"`
	Title    *string     `json:"title,omitempty"`
	Markdown *string     `json:"markdown,omitempty"`
	FolderID *string     `json:"folder_id,omitempty"`
	Summary  *string     `json:"summary,omitempty"`
	Tags     []string    `json:"tags,omitempty"`
	Agent    *agentInput `json:"agent,omitempty"`
}

type updateBlockInput struct {
	PageID   string `json:"page_id"`
	BlockID  string `json:"block_id"`
	Markdown string `json:"markdown"`
	Revision int64  `json:"revision,omitempty"`
}

type insertBlocksInput struct {
	PageID   string                  `json:"page_id"`
	Position insertBlocksPositionDTO `json:"position"`
	Markdown string                  `json:"markdown"`
	Revision int64                   `json:"revision,omitempty"`
}

type insertBlocksPositionDTO struct {
	AfterID  *string `json:"after_id,omitempty"`
	BeforeID *string `json:"before_id,omitempty"`
	At       *string `json:"at,omitempty"`
}

type deleteBlockInput struct {
	PageID   string `json:"page_id"`
	BlockID  string `json:"block_id"`
	Revision int64  `json:"revision,omitempty"`
}

type getPageInput struct {
	ID string `json:"id"`
}

type listPagesInput struct {
	FolderID *string `json:"folder_id,omitempty"`
	Limit    int     `json:"limit,omitempty"`
}

type searchPagesInput struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

type agentInput struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type pageSummary struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	FolderID  *string        `json:"folder_id,omitempty"`
	Summary   *string        `json:"summary,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
}

type pageDetail struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Blocks    []pageBlockView `json:"blocks"`
	FolderID  *string         `json:"folder_id,omitempty"`
	Summary   *string         `json:"summary,omitempty"`
	Metadata  map[string]any  `json:"metadata,omitempty"`
	Revision  int64           `json:"revision"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
}

// pageBlockView is the per-block payload an MCP client sees. The agent gets
// the stable id (for targeting), the BlockNote type/props (for reasoning),
// and a markdown rendering of the block (so it can read the content without
// needing to understand BlockNote's inline-content schema).
type pageBlockView struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Props    json.RawMessage `json:"props,omitempty"`
	Markdown string          `json:"markdown"`
}

type folderOutput struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	ParentID *string `json:"parent_id,omitempty"`
}

type createUpdatePageOutput struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	FolderID  *string `json:"folder_id,omitempty"`
	Revision  int64   `json:"revision"`
	UpdatedAt string  `json:"updated_at"`
}

type updateBlockOutput struct {
	PageID             string `json:"page_id"`
	BlockID            string `json:"block_id"`
	Revision           int64  `json:"revision"`
	ReplacedBlockCount int    `json:"replaced_block_count"`
}

type insertBlocksOutput struct {
	PageID          string   `json:"page_id"`
	Revision        int64    `json:"revision"`
	InsertedBlockIDs []string `json:"inserted_block_ids"`
}

type deleteBlockOutput struct {
	PageID   string `json:"page_id"`
	BlockID  string `json:"block_id"`
	Revision int64  `json:"revision"`
	Deleted  bool   `json:"deleted"`
}

type browserTreeNodeOutput struct {
	ID           string  `json:"id"`
	ParentID     *string `json:"parent_id,omitempty"`
	Kind         string  `json:"kind"`
	Title        string  `json:"title"`
	ArtifactID   *string `json:"artifact_id,omitempty"`
	ArtifactType *string `json:"artifact_type,omitempty"`
	UpdatedAt    *string `json:"updated_at,omitempty"`
	Depth        int     `json:"depth"`
}

type browserTreeOutput struct {
	Nodes []browserTreeNodeOutput `json:"nodes"`
}

type foldersOutput struct {
	Folders []folderOutput `json:"folders"`
}

type pageOutput struct {
	Page pageDetail `json:"page"`
}

type pagesOutput struct {
	Pages []pageSummary `json:"pages"`
}

func (t toolServer) getBrowserTree(ctx context.Context, _ *sdkmcp.CallToolRequest, _ emptyInput) (*sdkmcp.CallToolResult, browserTreeOutput, error) {
	nodes, err := t.artifacts.BrowserTree(ctx)
	if err != nil {
		return nil, browserTreeOutput{}, err
	}
	return nil, browserTreeOutput{Nodes: flattenBrowserTree(nodes)}, nil
}

func (t toolServer) listFolders(ctx context.Context, _ *sdkmcp.CallToolRequest, input listFoldersInput) (*sdkmcp.CallToolResult, foldersOutput, error) {
	folders, err := t.artifacts.ListFolders(ctx, input.ParentID)
	if err != nil {
		return nil, foldersOutput{}, err
	}
	out := make([]folderOutput, 0, len(folders))
	for _, folder := range folders {
		out = append(out, folderOutput{
			ID:       folder.ID,
			Title:    folder.Title,
			ParentID: folder.ParentID,
		})
	}
	return nil, foldersOutput{Folders: out}, nil
}

func (t toolServer) createPage(ctx context.Context, _ *sdkmcp.CallToolRequest, input createPageInput) (*sdkmcp.CallToolResult, createUpdatePageOutput, error) {
	if t.converter == nil {
		return nil, createUpdatePageOutput{}, service.BadRequest("blocknote-converter is not configured; cannot translate markdown to blocks")
	}
	if strings.TrimSpace(input.Title) == "" {
		return nil, createUpdatePageOutput{}, service.BadRequest("title is required")
	}
	blocks, err := t.converter.MDToBlocks(ctx, input.Markdown)
	if err != nil {
		return nil, createUpdatePageOutput{}, err
	}
	metadata := mergePageMetadata(nil, input.Tags, input.Agent)
	created, err := t.artifacts.Create(ctx, service.ArtifactPayload{
		Type:     pageArtifactType,
		FolderID: input.FolderID,
		Title:    input.Title,
		Blocks:   blocks,
		Summary:  input.Summary,
		Metadata: metadata,
	})
	if err != nil {
		return nil, createUpdatePageOutput{}, err
	}
	return nil, createUpdatePageOutput{
		ID:        created.Artifact.ID,
		Title:     created.Artifact.Title,
		FolderID:  created.Artifact.FolderID,
		Revision:  created.Artifact.Revision,
		UpdatedAt: created.Artifact.UpdatedAt,
	}, nil
}

func (t toolServer) updatePage(ctx context.Context, _ *sdkmcp.CallToolRequest, input updatePageInput) (*sdkmcp.CallToolResult, createUpdatePageOutput, error) {
	if strings.TrimSpace(input.ID) == "" {
		return nil, createUpdatePageOutput{}, service.ErrNotFound
	}
	if input.Title == nil && input.Markdown == nil && input.FolderID == nil && input.Summary == nil && input.Tags == nil && input.Agent == nil {
		return nil, createUpdatePageOutput{}, service.BadRequest("update_page requires at least one field")
	}
	current, err := t.artifacts.Get(ctx, input.ID)
	if err != nil {
		return nil, createUpdatePageOutput{}, err
	}
	if err := requirePage(current); err != nil {
		return nil, createUpdatePageOutput{}, err
	}

	patch := service.ArtifactPatch{
		Title:    input.Title,
		FolderID: input.FolderID,
		Summary:  input.Summary,
	}
	if input.Markdown != nil {
		if t.converter == nil {
			return nil, createUpdatePageOutput{}, service.BadRequest("blocknote-converter is not configured; cannot translate markdown to blocks")
		}
		blocks, err := t.converter.MDToBlocks(ctx, *input.Markdown)
		if err != nil {
			return nil, createUpdatePageOutput{}, err
		}
		patch.Blocks = &blocks
	}
	if input.Tags != nil || input.Agent != nil {
		merged := mergePageMetadata(current.Metadata, input.Tags, input.Agent)
		patch.Metadata = &merged
	}
	updated, err := t.artifacts.Update(ctx, input.ID, patch)
	if err != nil {
		return nil, createUpdatePageOutput{}, err
	}
	if err := requirePage(updated); err != nil {
		return nil, createUpdatePageOutput{}, err
	}
	return nil, createUpdatePageOutput{
		ID:        updated.ID,
		Title:     updated.Title,
		FolderID:  updated.FolderID,
		Revision:  updated.Revision,
		UpdatedAt: updated.UpdatedAt,
	}, nil
}

func (t toolServer) getPage(ctx context.Context, _ *sdkmcp.CallToolRequest, input getPageInput) (*sdkmcp.CallToolResult, pageOutput, error) {
	rec, err := t.artifacts.Get(ctx, input.ID)
	if err != nil {
		return nil, pageOutput{}, err
	}
	if err := requirePage(rec); err != nil {
		return nil, pageOutput{}, err
	}
	detail, err := t.renderPageDetail(ctx, rec)
	if err != nil {
		return nil, pageOutput{}, err
	}
	return nil, pageOutput{Page: detail}, nil
}

func (t toolServer) updateBlock(ctx context.Context, _ *sdkmcp.CallToolRequest, input updateBlockInput) (*sdkmcp.CallToolResult, updateBlockOutput, error) {
	if t.converter == nil {
		return nil, updateBlockOutput{}, service.BadRequest("blocknote-converter is not configured; cannot translate markdown to blocks")
	}
	if strings.TrimSpace(input.PageID) == "" {
		return nil, updateBlockOutput{}, service.BadRequest("page_id is required")
	}
	if strings.TrimSpace(input.BlockID) == "" {
		return nil, updateBlockOutput{}, service.BadRequest("block_id is required")
	}
	replacement, err := t.converter.MDToBlocks(ctx, input.Markdown)
	if err != nil {
		return nil, updateBlockOutput{}, err
	}
	rev, count, err := t.pages.ReplaceBlock(ctx, input.PageID, input.BlockID, replacement, input.Revision)
	if err != nil {
		return nil, updateBlockOutput{}, err
	}
	return nil, updateBlockOutput{
		PageID:             input.PageID,
		BlockID:            input.BlockID,
		Revision:           rev,
		ReplacedBlockCount: count,
	}, nil
}

func (t toolServer) insertBlocks(ctx context.Context, _ *sdkmcp.CallToolRequest, input insertBlocksInput) (*sdkmcp.CallToolResult, insertBlocksOutput, error) {
	if t.converter == nil {
		return nil, insertBlocksOutput{}, service.BadRequest("blocknote-converter is not configured; cannot translate markdown to blocks")
	}
	if strings.TrimSpace(input.PageID) == "" {
		return nil, insertBlocksOutput{}, service.BadRequest("page_id is required")
	}
	blocks, err := t.converter.MDToBlocks(ctx, input.Markdown)
	if err != nil {
		return nil, insertBlocksOutput{}, err
	}
	pos := service.BlockPosition{
		AfterID:  input.Position.AfterID,
		BeforeID: input.Position.BeforeID,
		At:       input.Position.At,
	}
	rev, ids, err := t.pages.InsertBlocks(ctx, input.PageID, pos, blocks, input.Revision)
	if err != nil {
		return nil, insertBlocksOutput{}, err
	}
	return nil, insertBlocksOutput{
		PageID:           input.PageID,
		Revision:         rev,
		InsertedBlockIDs: ids,
	}, nil
}

func (t toolServer) deleteBlock(ctx context.Context, _ *sdkmcp.CallToolRequest, input deleteBlockInput) (*sdkmcp.CallToolResult, deleteBlockOutput, error) {
	if strings.TrimSpace(input.PageID) == "" {
		return nil, deleteBlockOutput{}, service.BadRequest("page_id is required")
	}
	if strings.TrimSpace(input.BlockID) == "" {
		return nil, deleteBlockOutput{}, service.BadRequest("block_id is required")
	}
	rev, err := t.pages.DeleteBlock(ctx, input.PageID, input.BlockID, input.Revision)
	if err != nil {
		return nil, deleteBlockOutput{}, err
	}
	return nil, deleteBlockOutput{
		PageID:   input.PageID,
		BlockID:  input.BlockID,
		Revision: rev,
		Deleted:  true,
	}, nil
}

func (t toolServer) renderPageDetail(ctx context.Context, rec service.ArtifactResponse) (pageDetail, error) {
	parsed, err := blocknote.ParseBlocks(rec.Blocks)
	if err != nil {
		return pageDetail{}, err
	}
	views := make([]pageBlockView, len(parsed))
	if len(parsed) > 0 {
		blockArrays := make([]json.RawMessage, len(parsed))
		for i, b := range parsed {
			blockArrays[i] = json.RawMessage("[" + string(b.Raw) + "]")
		}
		var markdowns []string
		if t.converter != nil {
			markdowns, err = t.converter.BlocksToMDBatch(ctx, blockArrays)
			if err != nil {
				return pageDetail{}, err
			}
		} else {
			markdowns = make([]string, len(parsed))
		}
		for i, b := range parsed {
			views[i] = pageBlockView{
				ID:    b.ID,
				Type:  blockTypeOf(b.Raw),
				Props: extractBlockProps(b.Raw),
			}
			if i < len(markdowns) {
				views[i].Markdown = markdowns[i]
			}
		}
	}
	return pageDetail{
		ID:        rec.ID,
		Title:     rec.Title,
		Blocks:    views,
		FolderID:  rec.FolderID,
		Summary:   rec.Summary,
		Metadata:  rec.Metadata,
		Revision:  rec.Revision,
		CreatedAt: rec.CreatedAt,
		UpdatedAt: rec.UpdatedAt,
	}, nil
}

func blockTypeOf(raw json.RawMessage) string {
	var probe struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(raw, &probe)
	return probe.Type
}

func extractBlockProps(raw json.RawMessage) json.RawMessage {
	var probe struct {
		Props json.RawMessage `json:"props"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil
	}
	return probe.Props
}

func (t toolServer) listPages(ctx context.Context, _ *sdkmcp.CallToolRequest, input listPagesInput) (*sdkmcp.CallToolResult, pagesOutput, error) {
	limit := clampLimit(input.Limit)
	recs, err := t.artifacts.List(ctx, service.ArtifactListParams{FolderID: input.FolderID})
	if err != nil {
		return nil, pagesOutput{}, err
	}
	pages := make([]pageSummary, 0, min(len(recs), limit))
	for _, rec := range recs {
		if rec.Type != pageArtifactType {
			continue
		}
		pages = append(pages, toPageSummary(rec))
		if len(pages) == limit {
			break
		}
	}
	return nil, pagesOutput{Pages: pages}, nil
}

func (t toolServer) searchPages(ctx context.Context, _ *sdkmcp.CallToolRequest, input searchPagesInput) (*sdkmcp.CallToolResult, pagesOutput, error) {
	input.Query = strings.TrimSpace(input.Query)
	if input.Query == "" {
		return nil, pagesOutput{}, service.BadRequest("query is required")
	}
	recs, err := t.artifacts.SearchPages(ctx, service.PageSearchParams{
		Query: input.Query,
		Limit: clampLimit(input.Limit),
	})
	if err != nil {
		return nil, pagesOutput{}, err
	}
	pages := make([]pageSummary, 0, len(recs))
	for _, rec := range recs {
		if rec.Type == pageArtifactType {
			pages = append(pages, toPageSummary(rec))
		}
	}
	return nil, pagesOutput{Pages: pages}, nil
}

func requirePage(rec service.ArtifactResponse) error {
	if rec.Type != pageArtifactType {
		return service.BadRequest("artifact is not a page")
	}
	return nil
}

func flattenBrowserTree(nodes []service.BrowserTreeNode) []browserTreeNodeOutput {
	out := make([]browserTreeNodeOutput, 0)
	var walk func([]service.BrowserTreeNode, int)
	walk = func(children []service.BrowserTreeNode, depth int) {
		for _, node := range children {
			out = append(out, browserTreeNodeOutput{
				ID:           node.ID,
				ParentID:     node.ParentID,
				Kind:         node.Kind,
				Title:        node.Title,
				ArtifactID:   node.ArtifactID,
				ArtifactType: node.ArtifactType,
				UpdatedAt:    node.UpdatedAt,
				Depth:        depth,
			})
			walk(node.Children, depth+1)
		}
	}
	walk(nodes, 0)
	return out
}

func mergePageMetadata(base map[string]any, tags []string, agent *agentInput) map[string]any {
	metadata := make(map[string]any, len(base)+2)
	for key, value := range base {
		metadata[key] = value
	}
	if tags != nil {
		cleanTags := make([]string, 0, len(tags))
		for _, tag := range tags {
			if trimmed := strings.TrimSpace(tag); trimmed != "" {
				cleanTags = append(cleanTags, trimmed)
			}
		}
		metadata["tags"] = cleanTags
	}
	if agent != nil {
		agentMetadata := map[string]any{"source": "mcp"}
		if name := strings.TrimSpace(agent.Name); name != "" {
			agentMetadata["name"] = name
		}
		if id := strings.TrimSpace(agent.ID); id != "" {
			agentMetadata["id"] = id
		}
		metadata["agent"] = agentMetadata
	}
	return metadata
}

func toPageSummary(rec service.ArtifactResponse) pageSummary {
	return pageSummary{
		ID:        rec.ID,
		Title:     rec.Title,
		FolderID:  rec.FolderID,
		Summary:   rec.Summary,
		Metadata:  rec.Metadata,
		CreatedAt: rec.CreatedAt,
		UpdatedAt: rec.UpdatedAt,
	}
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 50 {
		return 50
	}
	return limit
}
