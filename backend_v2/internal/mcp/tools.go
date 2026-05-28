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
	// pages (M5/M6 PageDocumentService) is retained but unused: M8c routes
	// page reads/writes through the collab bridge. Removed in M8d with the
	// rest of the M7 page_content path.
	pages     service.PageDocumentService
	converter blocknote.Converter
	bridge    blocknote.Bridge
}

func registerTools(server *sdkmcp.Server, artifacts service.ArtifactService, pages service.PageDocumentService, converter blocknote.Converter, bridge blocknote.Bridge) {
	tools := toolServer{artifacts: artifacts, pages: pages, converter: converter, bridge: bridge}

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
}

type insertBlocksInput struct {
	PageID   string                  `json:"page_id"`
	Position insertBlocksPositionDTO `json:"position"`
	Markdown string                  `json:"markdown"`
}

type insertBlocksPositionDTO struct {
	AfterID  *string `json:"after_id,omitempty"`
	BeforeID *string `json:"before_id,omitempty"`
	At       *string `json:"at,omitempty"`
}

type deleteBlockInput struct {
	PageID  string `json:"page_id"`
	BlockID string `json:"block_id"`
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
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Props    map[string]any `json:"props,omitempty"`
	Markdown string         `json:"markdown"`
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
	ReplacedBlockCount int    `json:"replaced_block_count"`
	// Markdown renders the resulting block(s) so the agent sees its own edit
	// without a follow-up read (the read-after-own-write fix, M8.6).
	Markdown string `json:"markdown,omitempty"`
}

type insertBlocksOutput struct {
	PageID           string   `json:"page_id"`
	InsertedBlockIDs []string `json:"inserted_block_ids"`
	Markdown         string   `json:"markdown,omitempty"`
}

type deleteBlockOutput struct {
	PageID  string `json:"page_id"`
	BlockID string `json:"block_id"`
	Deleted bool   `json:"deleted"`
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
	if strings.TrimSpace(input.Title) == "" {
		return nil, createUpdatePageOutput{}, service.BadRequest("title is required")
	}
	hasContent := strings.TrimSpace(input.Markdown) != ""
	if hasContent && t.converter == nil {
		return nil, createUpdatePageOutput{}, service.BadRequest("blocknote-converter is not configured; cannot translate markdown to blocks")
	}
	metadata := mergePageMetadata(nil, input.Tags, input.Agent)
	// M8c: create the artifact row with empty blocks; content (if any) is
	// seeded into the live Y.Doc via the collab bridge once the row exists.
	created, err := t.artifacts.Create(ctx, service.ArtifactPayload{
		Type:     pageArtifactType,
		FolderID: input.FolderID,
		Title:    input.Title,
		Summary:  input.Summary,
		Metadata: metadata,
	})
	if err != nil {
		return nil, createUpdatePageOutput{}, err
	}
	if hasContent {
		blocks, err := t.converter.MDToBlocks(ctx, input.Markdown)
		if err != nil {
			return nil, createUpdatePageOutput{}, err
		}
		if _, err := t.applyBridge(ctx, blocknote.BridgeOp{
			PageID: created.Artifact.ID,
			Op:     "replace_all",
			Blocks: blocks,
			Agent:  agentToBridge(input.Agent),
		}); err != nil {
			return nil, createUpdatePageOutput{}, err
		}
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

	// Metadata (title/folder/summary/tags/agent) goes through the artifact
	// API; page content goes through the collab bridge (M8c). Independent
	// writes — the artifact API refuses block writes for pages now.
	rec := current
	if input.Title != nil || input.FolderID != nil || input.Summary != nil || input.Tags != nil || input.Agent != nil {
		patch := service.ArtifactPatch{
			Title:    input.Title,
			FolderID: input.FolderID,
			Summary:  input.Summary,
		}
		if input.Tags != nil || input.Agent != nil {
			merged := mergePageMetadata(current.Metadata, input.Tags, input.Agent)
			patch.Metadata = &merged
		}
		rec, err = t.artifacts.Update(ctx, input.ID, patch)
		if err != nil {
			return nil, createUpdatePageOutput{}, err
		}
		if err := requirePage(rec); err != nil {
			return nil, createUpdatePageOutput{}, err
		}
	}
	if input.Markdown != nil {
		if t.converter == nil {
			return nil, createUpdatePageOutput{}, service.BadRequest("blocknote-converter is not configured; cannot translate markdown to blocks")
		}
		blocks, err := t.converter.MDToBlocks(ctx, *input.Markdown)
		if err != nil {
			return nil, createUpdatePageOutput{}, err
		}
		if _, err := t.applyBridge(ctx, blocknote.BridgeOp{
			PageID: input.ID,
			Op:     "replace_all",
			Blocks: blocks,
			Agent:  agentToBridge(input.Agent),
		}); err != nil {
			return nil, createUpdatePageOutput{}, err
		}
	}
	return nil, createUpdatePageOutput{
		ID:        rec.ID,
		Title:     rec.Title,
		FolderID:  rec.FolderID,
		Revision:  rec.Revision,
		UpdatedAt: rec.UpdatedAt,
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
	// M8c: read fresh from the live Y.Doc via the bridge (not the projection)
	// — get_page is the precursor to editing and wants a current baseline.
	if t.bridge == nil {
		return nil, pageOutput{}, service.BadRequest("collab bridge is not configured; cannot read page content")
	}
	page, err := t.bridge.GetPage(ctx, input.ID)
	if err != nil {
		return nil, pageOutput{}, err
	}
	detail, err := t.renderPageDetail(ctx, rec, page.Blocks)
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
	// Keep the original id on the first produced block so the agent can keep
	// targeting it (the markdown may parse into multiple blocks).
	replacement = withFirstBlockID(replacement, input.BlockID)
	res, err := t.applyBridge(ctx, blocknote.BridgeOp{
		PageID:  input.PageID,
		Op:      "replace_block",
		BlockID: input.BlockID,
		Blocks:  replacement,
	})
	if err != nil {
		return nil, updateBlockOutput{}, err
	}
	return nil, updateBlockOutput{
		PageID:             input.PageID,
		BlockID:            input.BlockID,
		ReplacedBlockCount: len(res.AffectedBlockIDs),
		Markdown:           res.Markdown,
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
	op := blocknote.BridgeOp{
		PageID: input.PageID,
		Op:     "insert_blocks",
		Blocks: blocks,
	}
	if err := applyInsertPosition(&op, input.Position); err != nil {
		return nil, insertBlocksOutput{}, err
	}
	res, err := t.applyBridge(ctx, op)
	if err != nil {
		return nil, insertBlocksOutput{}, err
	}
	return nil, insertBlocksOutput{
		PageID:           input.PageID,
		InsertedBlockIDs: res.AffectedBlockIDs,
		Markdown:         res.Markdown,
	}, nil
}

func (t toolServer) deleteBlock(ctx context.Context, _ *sdkmcp.CallToolRequest, input deleteBlockInput) (*sdkmcp.CallToolResult, deleteBlockOutput, error) {
	if strings.TrimSpace(input.PageID) == "" {
		return nil, deleteBlockOutput{}, service.BadRequest("page_id is required")
	}
	if strings.TrimSpace(input.BlockID) == "" {
		return nil, deleteBlockOutput{}, service.BadRequest("block_id is required")
	}
	if _, err := t.applyBridge(ctx, blocknote.BridgeOp{
		PageID:  input.PageID,
		Op:      "delete_block",
		BlockID: input.BlockID,
	}); err != nil {
		return nil, deleteBlockOutput{}, err
	}
	return nil, deleteBlockOutput{
		PageID:  input.PageID,
		BlockID: input.BlockID,
		Deleted: true,
	}, nil
}

func (t toolServer) renderPageDetail(ctx context.Context, rec service.ArtifactResponse, blocks json.RawMessage) (pageDetail, error) {
	parsed, err := blocknote.ParseBlocks(blocks)
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

func extractBlockProps(raw json.RawMessage) map[string]any {
	var probe struct {
		Props map[string]any `json:"props"`
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

// --- M8c bridge helpers ----------------------------------------------------

// applyBridge guards the bridge call: a nil bridge (misconfiguration) yields a
// clear error instead of a nil-pointer panic.
func (t toolServer) applyBridge(ctx context.Context, op blocknote.BridgeOp) (blocknote.BridgeResult, error) {
	if t.bridge == nil {
		return blocknote.BridgeResult{}, service.BadRequest("collab bridge is not configured; cannot edit page content")
	}
	return t.bridge.ApplyOperation(ctx, op)
}

func agentToBridge(a *agentInput) *blocknote.BridgeAgent {
	if a == nil {
		return nil
	}
	return &blocknote.BridgeAgent{ID: a.ID, Name: a.Name}
}

// applyInsertPosition maps the MCP insert-position DTO onto the bridge op:
//
//	after_id / before_id => anchor block id + placement
//	{at: "start"}        => index 0
//	{at: "end"} / unset  => append (no anchor, no index)
//
// It rejects an ambiguous position (both after_id and before_id) and an
// unrecognized `at` value rather than silently appending in the wrong place.
func applyInsertPosition(op *blocknote.BridgeOp, pos insertBlocksPositionDTO) error {
	hasAfter := pos.AfterID != nil && strings.TrimSpace(*pos.AfterID) != ""
	hasBefore := pos.BeforeID != nil && strings.TrimSpace(*pos.BeforeID) != ""
	hasAt := pos.At != nil && strings.TrimSpace(*pos.At) != ""
	switch {
	case hasAfter && hasBefore:
		return service.BadRequest("position: set only one of after_id / before_id")
	case hasAfter:
		op.BlockID = *pos.AfterID
		op.Placement = "after"
	case hasBefore:
		op.BlockID = *pos.BeforeID
		op.Placement = "before"
	case hasAt:
		switch strings.TrimSpace(*pos.At) {
		case "start":
			zero := 0
			op.Position = &zero
		case "end":
			// append: no anchor, no index (the bridge defaults to the end)
		default:
			return service.BadRequest(`position.at must be "start" or "end"`)
		}
	}
	return nil
}

// withFirstBlockID overrides the id of the first block in a blocks array so a
// replacement keeps the target block's id. Returns the input unchanged if it
// can't be parsed as a non-empty array.
func withFirstBlockID(blocks json.RawMessage, id string) json.RawMessage {
	var arr []map[string]json.RawMessage
	if err := json.Unmarshal(blocks, &arr); err != nil || len(arr) == 0 {
		return blocks
	}
	idJSON, err := json.Marshal(id)
	if err != nil {
		return blocks
	}
	arr[0]["id"] = idJSON
	out, err := json.Marshal(arr)
	if err != nil {
		return blocks
	}
	return out
}
