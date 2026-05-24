package mcpserver

import (
	"context"
	"strings"

	"aladin/backend_v2/internal/service"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const pageArtifactType = "page"

type toolServer struct {
	artifacts service.ArtifactService
}

func registerTools(server *sdkmcp.Server, artifacts service.ArtifactService) {
	tools := toolServer{artifacts: artifacts}

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
		Description: "Create an Aladin markdown page in an optional folder.",
	}, tools.createPage)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "update_page",
		Description: "Update an existing Aladin markdown page.",
	}, tools.updatePage)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_page",
		Description: "Read a single Aladin markdown page.",
	}, tools.getPage)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_pages",
		Description: "List page summaries in an optional folder.",
	}, tools.listPages)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "search_pages",
		Description: "Search page titles, summaries, and markdown content.",
	}, tools.searchPages)
}

type emptyInput struct{}

type listFoldersInput struct {
	ParentID *string `json:"parent_id,omitempty"`
}

type createPageInput struct {
	Title    string      `json:"title"`
	Content  string      `json:"content"`
	FolderID *string     `json:"folder_id,omitempty"`
	Summary  *string     `json:"summary,omitempty"`
	Tags     []string    `json:"tags,omitempty"`
	Agent    *agentInput `json:"agent,omitempty"`
}

type updatePageInput struct {
	ID       string      `json:"id"`
	Title    *string     `json:"title,omitempty"`
	Content  *string     `json:"content,omitempty"`
	FolderID *string     `json:"folder_id,omitempty"`
	Summary  *string     `json:"summary,omitempty"`
	Tags     []string    `json:"tags,omitempty"`
	Agent    *agentInput `json:"agent,omitempty"`
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
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Content   string         `json:"content"`
	FolderID  *string        `json:"folder_id,omitempty"`
	Summary   *string        `json:"summary,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
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
	UpdatedAt string  `json:"updated_at"`
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
	metadata := mergePageMetadata(nil, input.Tags, input.Agent)
	rec, err := t.artifacts.Create(ctx, service.ArtifactPayload{
		Type:     pageArtifactType,
		FolderID: input.FolderID,
		Title:    input.Title,
		Content:  input.Content,
		Summary:  input.Summary,
		Metadata: metadata,
	})
	if err != nil {
		return nil, createUpdatePageOutput{}, err
	}
	return nil, createUpdatePageOutput{
		ID:        rec.Artifact.ID,
		Title:     rec.Artifact.Title,
		FolderID:  rec.Artifact.FolderID,
		UpdatedAt: rec.Artifact.UpdatedAt,
	}, nil
}

func (t toolServer) updatePage(ctx context.Context, _ *sdkmcp.CallToolRequest, input updatePageInput) (*sdkmcp.CallToolResult, createUpdatePageOutput, error) {
	if strings.TrimSpace(input.ID) == "" {
		return nil, createUpdatePageOutput{}, service.ErrNotFound
	}
	if input.Title == nil && input.Content == nil && input.FolderID == nil && input.Summary == nil && input.Tags == nil && input.Agent == nil {
		return nil, createUpdatePageOutput{}, service.BadRequest("update_page requires at least one field")
	}
	current, err := t.artifacts.Get(ctx, input.ID)
	if err != nil {
		return nil, createUpdatePageOutput{}, err
	}
	if err := requirePage(current); err != nil {
		return nil, createUpdatePageOutput{}, err
	}

	var metadata *map[string]any
	if input.Tags != nil || input.Agent != nil {
		merged := mergePageMetadata(current.Metadata, input.Tags, input.Agent)
		metadata = &merged
	}
	updated, err := t.artifacts.Update(ctx, input.ID, service.ArtifactPatch{
		Title:    input.Title,
		Content:  input.Content,
		FolderID: input.FolderID,
		Summary:  input.Summary,
		Metadata: metadata,
	})
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
	return nil, pageOutput{Page: toPageDetail(rec)}, nil
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

func toPageDetail(rec service.ArtifactResponse) pageDetail {
	return pageDetail{
		ID:        rec.ID,
		Title:     rec.Title,
		Content:   rec.Content,
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
