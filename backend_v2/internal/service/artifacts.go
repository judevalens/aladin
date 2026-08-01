package service

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"
)

// emptyBlocks is the canonical "no blocks" JSON value persisted alongside a
// freshly-created page that the agent or user has not yet populated.
var emptyBlocks = json.RawMessage(`[]`)

type ArtifactService interface {
	List(context.Context, ArtifactListParams) ([]ArtifactResponse, error)
	SearchPages(context.Context, PageSearchParams) ([]ArtifactResponse, error)
	// QueryByProperty is the H1c "frontmatter" read: the caller's artifacts filtered by a typed
	// property. Empty Value = every artifact carrying the key.
	QueryByProperty(context.Context, PropertyQuery) ([]ArtifactResponse, error)
	// PropertyFacets lists the property keys/values actually in use (for a filter UI).
	PropertyFacets(context.Context) ([]PropertyFacet, error)
	BrowserTree(context.Context) ([]BrowserTreeNode, error)
	CreateBrowserNode(context.Context, BrowserNodeCreateInput) (BrowserNodeCreateResponse, error)
	DeleteBrowserNode(context.Context, string) (NodeDeleteResult, error)
	Get(context.Context, string) (ArtifactResponse, error)
	Create(context.Context, ArtifactPayload) (ArtifactCreateResponse, error)
	Update(context.Context, string, ArtifactPatch) (ArtifactResponse, error)
	Delete(context.Context, string) (NodeDeleteResult, error)
	Upload(context.Context, ArtifactUploadInput, io.Reader) (ArtifactResponse, error)
	Resource(context.Context, string) (ArtifactResource, error)
	ListFolders(context.Context, *string) ([]FolderNode, error)
	FolderTree(context.Context) ([]FolderTreeNode, error)
	CreateFolder(context.Context, string, *string) (FolderNode, error)
	UpdateFolder(context.Context, string, FolderPatch) (FolderNode, error)
	GetFolder(context.Context, string) (FolderNode, error)
	FolderBreadcrumbs(context.Context, string) ([]BreadcrumbItem, error)
}

type ArtifactRepository interface {
	ListArtifacts(context.Context, ArtifactListParams) ([]ArtifactResponse, error)
	SearchPageArtifacts(context.Context, PageSearchParams) ([]ArtifactResponse, error)
	// QueryArtifactsByProperty returns the caller's artifacts carrying a typed property
	// (metadata->'properties'). An empty Value matches any value for the key.
	QueryArtifactsByProperty(context.Context, PropertyQuery) ([]ArtifactResponse, error)
	// PropertyFacets lists the distinct property keys (and their values) in use, so a filter UI
	// can offer real choices instead of free text.
	PropertyFacets(context.Context) ([]PropertyFacet, error)
	GetArtifact(context.Context, string) (ArtifactResponse, error)
	// LightNode returns a node's current light representation + seq (incl
	// tombstones) — the model a write returns so the client applies the result
	// under the seq guard. No Frame leaks into REST.
	LightNode(context.Context, string) (BrowserNodeResponse, error)
	// CreateArtifactGraph writes the artifact, its tree node, and (when
	// pageBlocks != nil) the initial page_documents row in a single
	// transaction. pageBlocks must be a JSON array of BlockNote blocks;
	// pageSearchText is the pre-derived inline text for full-text search.
	CreateArtifactGraph(ctx context.Context, rec ArtifactResponse, node TreeNodeRecord, pageBlocks json.RawMessage, pageSearchText string) error
	UpdateArtifact(context.Context, string, ArtifactPatch) error
	// CreatePageDocument inserts the page_documents row for an artifact that
	// already exists. blocks must be a JSON array; searchText is the
	// pre-derived inline text.
	CreatePageDocument(ctx context.Context, artifactID string, blocks json.RawMessage, searchText string) error
	// SavePageBlocks is the single mutation point for page blocks. expectedRev
	// of 0 means last-write-wins; >0 enforces optimistic concurrency and
	// returns ErrConflict if the stored revision is >= expectedRev.
	SavePageBlocks(ctx context.Context, artifactID string, blocks json.RawMessage, searchText string, expectedRev int64) (newRev int64, err error)
	DeleteArtifact(context.Context, string) error
	ListFolders(context.Context, *string) ([]FolderNode, error)
	ListAllFolders(context.Context) ([]FolderNode, error)
	ListAllBrowserNodes(context.Context) ([]BrowserTreeFlatNode, error)
	NextNodePosition(context.Context, *string) (int64, error)
	CreateTreeNode(context.Context, TreeNodeRecord) error
	DeleteBrowserNode(context.Context, string) error
	UpdateArtifactNodeParent(context.Context, string, *string) error
	UpdateFolderTitle(context.Context, string, string) error
	GetFolder(context.Context, string) (FolderNode, error)
	// GetContainer resolves a node that may CONTAIN others: a plain folder OR a
	// research folder (RESEARCH_SURFACE_PRD §21). Parent/destination validation uses
	// this; GetFolder stays folder-only so the folder rename/read API can't touch a
	// research node.
	GetContainer(context.Context, string) (FolderNode, error)
	FolderBreadcrumbs(context.Context, string) ([]BreadcrumbItem, error)
}

type StoredArtifactResource struct {
	StorageKey       string
	ResourceKind     string
	MIMEType         string
	OriginalFilename string
	SizeBytes        int64
}

type ArtifactFileStore interface {
	SaveResource(kind string, filename string, contentType string, body io.Reader) (StoredArtifactResource, error)
	ResourcePath(string) (string, error)
}

type DefaultArtifactService struct {
	repo  ArtifactRepository
	files ArtifactFileStore
}

func NewArtifactService(repo ArtifactRepository, files ArtifactFileStore) *DefaultArtifactService {
	return &DefaultArtifactService{repo: repo, files: files}
}

func (s *DefaultArtifactService) List(ctx context.Context, params ArtifactListParams) ([]ArtifactResponse, error) {
	if err := RequireScope(ctx, ScopeArtifactsRead); err != nil {
		return nil, err
	}
	if params.FolderID != nil {
		params.FolderID = TrimStringPtr(params.FolderID)
		if params.FolderID != nil {
			if _, err := s.repo.GetContainer(ctx, *params.FolderID); err != nil {
				return nil, err
			}
		}
	}
	return s.repo.ListArtifacts(ctx, params)
}

func (s *DefaultArtifactService) SearchPages(ctx context.Context, params PageSearchParams) ([]ArtifactResponse, error) {
	if err := RequireScope(ctx, ScopeArtifactsRead); err != nil {
		return nil, err
	}
	params.Query = strings.TrimSpace(params.Query)
	if params.Query == "" {
		return nil, BadRequest("query is required")
	}
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 50 {
		params.Limit = 50
	}
	return s.repo.SearchPageArtifacts(ctx, params)
}

func (s *DefaultArtifactService) QueryByProperty(ctx context.Context, params PropertyQuery) ([]ArtifactResponse, error) {
	if err := RequireScope(ctx, ScopeArtifactsRead); err != nil {
		return nil, err
	}
	params.Key = strings.TrimSpace(params.Key)
	params.Value = strings.TrimSpace(params.Value)
	if params.Key == "" {
		return nil, BadRequest("key is required")
	}
	if params.Limit <= 0 {
		params.Limit = 100
	}
	if params.Limit > 500 {
		params.Limit = 500
	}
	return s.repo.QueryArtifactsByProperty(ctx, params)
}

func (s *DefaultArtifactService) PropertyFacets(ctx context.Context) ([]PropertyFacet, error) {
	if err := RequireScope(ctx, ScopeArtifactsRead); err != nil {
		return nil, err
	}
	return s.repo.PropertyFacets(ctx)
}

func (s *DefaultArtifactService) BrowserTree(ctx context.Context) ([]BrowserTreeNode, error) {
	if err := RequireScope(ctx, ScopeArtifactsRead); err != nil {
		return nil, err
	}
	nodes, err := s.repo.ListAllBrowserNodes(ctx)
	if err != nil {
		return nil, err
	}

	childrenByParentID := make(map[string][]BrowserTreeFlatNode)
	rootNodes := make([]BrowserTreeFlatNode, 0)
	for _, node := range nodes {
		if node.ParentID == nil {
			rootNodes = append(rootNodes, node)
			continue
		}
		childrenByParentID[*node.ParentID] = append(childrenByParentID[*node.ParentID], node)
	}

	var build func(BrowserTreeFlatNode) BrowserTreeNode
	build = func(node BrowserTreeFlatNode) BrowserTreeNode {
		children := childrenByParentID[node.ID]
		out := BrowserTreeNode{
			ID:           node.ID,
			ParentID:     node.ParentID,
			Kind:         node.Kind,
			Title:        node.Title,
			ArtifactID:   node.ArtifactID,
			ArtifactType: node.ArtifactType,
			UpdatedAt:    node.UpdatedAt,
			Children:     make([]BrowserTreeNode, 0, len(children)),
		}
		for _, child := range children {
			out.Children = append(out.Children, build(child))
		}
		return out
	}

	tree := make([]BrowserTreeNode, 0, len(rootNodes))
	for _, node := range rootNodes {
		tree = append(tree, build(node))
	}
	return tree, nil
}

func (s *DefaultArtifactService) Get(ctx context.Context, id string) (ArtifactResponse, error) {
	if err := RequireScope(ctx, ScopeArtifactsRead); err != nil {
		return ArtifactResponse{}, err
	}
	if strings.TrimSpace(id) == "" {
		return ArtifactResponse{}, ErrNotFound
	}
	return s.repo.GetArtifact(ctx, id)
}

func (s *DefaultArtifactService) Create(ctx context.Context, payload ArtifactPayload) (ArtifactCreateResponse, error) {
	if err := RequireScope(ctx, ScopeArtifactsWrite); err != nil {
		return ArtifactCreateResponse{}, err
	}
	artifactType := strings.TrimSpace(payload.Type)
	if artifactType == "" {
		return ArtifactCreateResponse{}, BadRequest("type is required")
	}
	if artifactType != "page" && artifactType != "link" && artifactType != "app" {
		return ArtifactCreateResponse{}, BadRequest("type must be one of: page, link, app")
	}
	payload.FolderID = TrimStringPtr(payload.FolderID)
	if payload.FolderID != nil {
		if _, err := s.repo.GetContainer(ctx, *payload.FolderID); err != nil {
			return ArtifactCreateResponse{}, err
		}
	}

	title := strings.TrimSpace(payload.Title)
	content := strings.TrimSpace(payload.Content)
	sourceURL := TrimStringPtr(payload.SourceURL)

	var (
		pageBlocks     json.RawMessage
		pageSearchText string
	)

	switch artifactType {
	case "page":
		if title == "" {
			return ArtifactCreateResponse{}, BadRequest("title is required")
		}
		// M8c: page content is owned by the collaborative Y.Doc, not written
		// through this API. Always create an empty document; the editor (or,
		// for an agent, a bridge replace_all op issued after this row exists)
		// supplies the first content. payload.Blocks is intentionally ignored.
		pageBlocks = emptyBlocks
		pageSearchText = ""
		content = "" // content is unused for pages
	case "link":
		if sourceURL == nil {
			return ArtifactCreateResponse{}, BadRequest("sourceUrl is required")
		}
		if title == "" {
			return ArtifactCreateResponse{}, BadRequest("title is required")
		}
	case "app":
		// Doc Surface page. Source lives on the data volume (not Postgres);
		// this row carries only metadata + (later) the MD summary. No blocks.
		if title == "" {
			return ArtifactCreateResponse{}, BadRequest("title is required")
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	id := strings.TrimSpace(payload.ID)
	if id == "" {
		id = newID("artifact-")
	}
	rec := ArtifactResponse{
		ID:        id,
		Type:      artifactType,
		FolderID:  payload.FolderID,
		Title:     title,
		Content:   content,
		Blocks:    pageBlocks,
		Summary:   TrimStringPtr(payload.Summary),
		SourceURL: sourceURL,
		Metadata:  payload.Metadata,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if rec.Metadata == nil {
		rec.Metadata = map[string]any{}
	}
	position, err := s.repo.NextNodePosition(ctx, payload.FolderID)
	if err != nil {
		return ArtifactCreateResponse{}, err
	}
	artifactID := rec.ID
	node := TreeNodeRecord{
		ID:         rec.ID,
		ParentID:   payload.FolderID,
		Kind:       "artifact",
		ArtifactID: &artifactID,
		Position:   position,
	}
	if err := s.repo.CreateArtifactGraph(ctx, rec, node, pageBlocks, pageSearchText); err != nil {
		return ArtifactCreateResponse{}, err
	}
	// Return the committed light node (incl. its seq) so the caller can apply the
	// create locally under the seq guard — read atomically post-write so seq and
	// fields are coherent.
	nodeResponse, err := s.repo.LightNode(ctx, node.ID)
	if err != nil {
		return ArtifactCreateResponse{}, err
	}
	return ArtifactCreateResponse{
		Artifact: rec,
		Node:     nodeResponse,
	}, nil
}

func (s *DefaultArtifactService) CreateBrowserNode(ctx context.Context, input BrowserNodeCreateInput) (BrowserNodeCreateResponse, error) {
	if err := RequireScope(ctx, ScopeArtifactsWrite); err != nil {
		return BrowserNodeCreateResponse{}, err
	}
	kind := strings.TrimSpace(input.Kind)
	switch kind {
	case "folder":
		node, err := s.createFolderNode(ctx, input.ID, input.Title, input.ParentID)
		if err != nil {
			return BrowserNodeCreateResponse{}, err
		}
		return BrowserNodeCreateResponse{Node: node}, nil
	case "artifact":
		if input.Artifact == nil {
			return BrowserNodeCreateResponse{}, BadRequest("artifact payload is required")
		}
		created, err := s.Create(ctx, ArtifactPayload{
			Type:      input.Artifact.Type,
			ID:        firstNonEmpty(input.Artifact.ID, input.ID),
			FolderID:  input.ParentID,
			Title:     input.Title,
			Content:   input.Artifact.Content,
			Blocks:    input.Artifact.Blocks,
			Summary:   input.Artifact.Summary,
			SourceURL: input.Artifact.SourceURL,
			Metadata:  input.Artifact.Metadata,
		})
		if err != nil {
			return BrowserNodeCreateResponse{}, err
		}
		artifact := created.Artifact
		return BrowserNodeCreateResponse{
			Node:     created.Node,
			Artifact: &artifact,
		}, nil
	default:
		return BrowserNodeCreateResponse{}, BadRequest("kind must be one of: folder, artifact")
	}
}

func (s *DefaultArtifactService) DeleteBrowserNode(ctx context.Context, id string) (NodeDeleteResult, error) {
	if err := RequireScope(ctx, ScopeArtifactsWrite); err != nil {
		return NodeDeleteResult{}, err
	}
	if strings.TrimSpace(id) == "" {
		return NodeDeleteResult{}, ErrNotFound
	}
	if err := s.repo.DeleteBrowserNode(ctx, id); err != nil {
		return NodeDeleteResult{}, err
	}
	// Return the root tombstone's version; descendants (if any) heal via the WS
	// frame / next pull.
	node, err := s.repo.LightNode(ctx, id)
	if err != nil {
		return NodeDeleteResult{}, err
	}
	return NodeDeleteResult{ID: id, Seq: node.Seq}, nil
}

func (s *DefaultArtifactService) Update(ctx context.Context, id string, patch ArtifactPatch) (ArtifactResponse, error) {
	if err := RequireScope(ctx, ScopeArtifactsWrite); err != nil {
		return ArtifactResponse{}, err
	}
	if strings.TrimSpace(id) == "" {
		return ArtifactResponse{}, ErrNotFound
	}
	current, err := s.repo.GetArtifact(ctx, id)
	if err != nil {
		return ArtifactResponse{}, err
	}
	if patch.Type != nil {
		trimmedType := strings.TrimSpace(*patch.Type)
		if trimmedType == "" {
			return ArtifactResponse{}, BadRequest("type cannot be empty")
		}
		if trimmedType != "page" && trimmedType != "link" && trimmedType != "voice" && trimmedType != "file" && trimmedType != "app" {
			return ArtifactResponse{}, BadRequest("unsupported type")
		}
		patch.Type = &trimmedType
	}
	if patch.Title != nil && strings.TrimSpace(*patch.Title) == "" {
		return ArtifactResponse{}, BadRequest("title cannot be empty")
	}
	if patch.Content != nil {
		content := *patch.Content
		patch.Content = &content
	}
	patch.Title = TrimStringPtr(patch.Title)
	patch.Summary = TrimStringPtr(patch.Summary)
	patch.SourceURL = TrimStringPtr(patch.SourceURL)
	patch.FolderID = TrimStringPtr(patch.FolderID)
	if patch.FolderID != nil {
		if _, err := s.repo.GetContainer(ctx, *patch.FolderID); err != nil {
			return ArtifactResponse{}, err
		}
	}

	nextType := current.Type
	if patch.Type != nil {
		nextType = *patch.Type
	}
	// app artifacts are backed by files on the data volume (not page_documents);
	// converting to/from another type would orphan that storage. Forbid it.
	if current.Type != nextType && (current.Type == "app" || nextType == "app") {
		return ArtifactResponse{}, BadRequest("an app artifact's type cannot be changed")
	}
	nextTitle := current.Title
	if patch.Title != nil {
		nextTitle = *patch.Title
	}
	nextSourceURL := current.SourceURL
	if patch.SourceURL != nil {
		nextSourceURL = patch.SourceURL
	}
	if nextType == "link" && nextSourceURL == nil {
		return ArtifactResponse{}, BadRequest("sourceUrl is required")
	}
	if nextType == "link" && strings.TrimSpace(nextTitle) == "" {
		return ArtifactResponse{}, BadRequest("title is required")
	}
	if nextType == "page" && strings.TrimSpace(nextTitle) == "" {
		return ArtifactResponse{}, BadRequest("title is required")
	}
	if nextType == "app" && strings.TrimSpace(nextTitle) == "" {
		return ArtifactResponse{}, BadRequest("title is required")
	}
	if current.Type == "page" && patch.Content != nil {
		return ArtifactResponse{}, BadRequest("page content is edited collaboratively, not via the artifact API")
	}

	if patch.FolderID != nil {
		if err := s.repo.UpdateArtifactNodeParent(ctx, id, patch.FolderID); err != nil {
			return ArtifactResponse{}, err
		}
	}
	// M8c seam guard: page blocks are owned by the collaborative Y.Doc. A
	// direct block write through the artifact API would silently diverge from
	// (or be clobbered by) the Hocuspocus doc + its projection. Agents edit
	// pages via the MCP collab bridge; the editor edits via the Y.Doc.
	if current.Type == "page" && patch.Blocks != nil {
		return ArtifactResponse{}, BadRequest("page blocks are edited via the collab bridge, not the artifact API")
	}
	if err := s.repo.UpdateArtifact(ctx, id, patch); err != nil {
		return ArtifactResponse{}, err
	}
	updated, err := s.repo.GetArtifact(ctx, id)
	if err != nil {
		return ArtifactResponse{}, err
	}
	// Stamp the post-write version so the caller can apply the rename locally
	// under the seq guard.
	node, err := s.repo.LightNode(ctx, id)
	if err != nil {
		return ArtifactResponse{}, err
	}
	updated.Seq = node.Seq
	return updated, nil
}

func (s *DefaultArtifactService) Delete(ctx context.Context, id string) (NodeDeleteResult, error) {
	if err := RequireScope(ctx, ScopeArtifactsWrite); err != nil {
		return NodeDeleteResult{}, err
	}
	if strings.TrimSpace(id) == "" {
		return NodeDeleteResult{}, ErrNotFound
	}
	if err := s.repo.DeleteArtifact(ctx, id); err != nil {
		return NodeDeleteResult{}, err
	}
	node, err := s.repo.LightNode(ctx, id)
	if err != nil {
		return NodeDeleteResult{}, err
	}
	return NodeDeleteResult{ID: id, Seq: node.Seq}, nil
}

func (s *DefaultArtifactService) Upload(ctx context.Context, input ArtifactUploadInput, body io.Reader) (ArtifactResponse, error) {
	if err := RequireScope(ctx, ScopeArtifactsWrite); err != nil {
		return ArtifactResponse{}, err
	}
	artifactType := strings.TrimSpace(input.Type)
	if artifactType != "voice" && artifactType != "file" {
		return ArtifactResponse{}, BadRequest("type must be one of: voice, file")
	}
	input.FolderID = TrimStringPtr(input.FolderID)
	if input.FolderID != nil {
		if _, err := s.repo.GetContainer(ctx, *input.FolderID); err != nil {
			return ArtifactResponse{}, err
		}
	}
	filename := strings.TrimSpace(input.Filename)
	if filename == "" {
		return ArtifactResponse{}, BadRequest("filename is required")
	}
	stored, err := s.files.SaveResource(artifactType, filename, input.ContentType, body)
	if err != nil {
		return ArtifactResponse{}, err
	}
	title := filename
	if trimmedTitle := TrimStringPtr(input.Title); trimmedTitle != nil {
		title = *trimmedTitle
	}
	now := time.Now().UTC().Format(time.RFC3339)
	metadata := map[string]any{
		"resourceKind":     stored.ResourceKind,
		"storageKey":       stored.StorageKey,
		"mimeType":         stored.MIMEType,
		"originalFilename": stored.OriginalFilename,
		"sizeBytes":        stored.SizeBytes,
	}
	rec := ArtifactResponse{
		ID:        newID("artifact-"),
		Type:      artifactType,
		FolderID:  input.FolderID,
		Title:     title,
		Content:   "",
		Summary:   TrimStringPtr(input.Summary),
		Metadata:  metadata,
		CreatedAt: now,
		UpdatedAt: now,
	}
	position, err := s.repo.NextNodePosition(ctx, input.FolderID)
	if err != nil {
		return ArtifactResponse{}, err
	}
	artifactID := rec.ID
	node := TreeNodeRecord{
		ID:         rec.ID,
		ParentID:   input.FolderID,
		Kind:       "artifact",
		ArtifactID: &artifactID,
		Position:   position,
	}
	if err := s.repo.CreateArtifactGraph(ctx, rec, node, nil, ""); err != nil {
		return ArtifactResponse{}, err
	}
	return rec, nil
}

func (s *DefaultArtifactService) Resource(ctx context.Context, id string) (ArtifactResource, error) {
	rec, err := s.Get(ctx, id)
	if err != nil {
		return ArtifactResource{}, err
	}
	if rec.Type != "voice" && rec.Type != "file" {
		return ArtifactResource{}, BadRequest("artifact does not have a resource")
	}
	storageKey, _ := rec.Metadata["storageKey"].(string)
	if strings.TrimSpace(storageKey) == "" {
		return ArtifactResource{}, ErrNotFound
	}
	path, err := s.files.ResourcePath(storageKey)
	if err != nil {
		return ArtifactResource{}, err
	}
	contentType, _ := rec.Metadata["mimeType"].(string)
	return ArtifactResource{
		Path:        path,
		ContentType: contentType,
	}, nil
}

func (s *DefaultArtifactService) ListFolders(ctx context.Context, parentID *string) ([]FolderNode, error) {
	if err := RequireScope(ctx, ScopeArtifactsRead); err != nil {
		return nil, err
	}
	parentID = TrimStringPtr(parentID)
	if parentID != nil {
		if _, err := s.repo.GetContainer(ctx, *parentID); err != nil {
			return nil, err
		}
	}
	return s.repo.ListFolders(ctx, parentID)
}

func (s *DefaultArtifactService) FolderTree(ctx context.Context) ([]FolderTreeNode, error) {
	if err := RequireScope(ctx, ScopeArtifactsRead); err != nil {
		return nil, err
	}
	folders, err := s.repo.ListAllFolders(ctx)
	if err != nil {
		return nil, err
	}

	childrenByParentID := make(map[string][]FolderNode)
	rootFolders := make([]FolderNode, 0)
	for _, folder := range folders {
		if folder.ParentID == nil {
			rootFolders = append(rootFolders, folder)
			continue
		}
		childrenByParentID[*folder.ParentID] = append(childrenByParentID[*folder.ParentID], folder)
	}

	var build func(FolderNode) FolderTreeNode
	build = func(folder FolderNode) FolderTreeNode {
		children := childrenByParentID[folder.ID]
		node := FolderTreeNode{
			ID:       folder.ID,
			ParentID: folder.ParentID,
			Title:    folder.Title,
			Children: make([]FolderTreeNode, 0, len(children)),
		}
		for _, child := range children {
			node.Children = append(node.Children, build(child))
		}
		return node
	}

	tree := make([]FolderTreeNode, 0, len(rootFolders))
	for _, folder := range rootFolders {
		tree = append(tree, build(folder))
	}
	return tree, nil
}

func (s *DefaultArtifactService) CreateFolder(ctx context.Context, title string, parentID *string) (FolderNode, error) {
	node, err := s.createFolderNode(ctx, "", title, parentID)
	if err != nil {
		return FolderNode{}, err
	}
	return FolderNode{
		ID:       node.ID,
		ParentID: node.ParentID,
		Title:    node.Title,
		Seq:      node.Seq,
	}, nil
}

func (s *DefaultArtifactService) createFolderNode(ctx context.Context, id string, title string, parentID *string) (BrowserNodeResponse, error) {
	if err := RequireScope(ctx, ScopeArtifactsWrite); err != nil {
		return BrowserNodeResponse{}, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return BrowserNodeResponse{}, BadRequest("title is required")
	}
	parentID = TrimStringPtr(parentID)
	if parentID != nil {
		if _, err := s.repo.GetContainer(ctx, *parentID); err != nil {
			return BrowserNodeResponse{}, err
		}
	}
	folderID := strings.TrimSpace(id)
	if folderID == "" {
		folderID = newID("folder-")
	}
	position, err := s.repo.NextNodePosition(ctx, parentID)
	if err != nil {
		return BrowserNodeResponse{}, err
	}
	folderTitle := title
	if err := s.repo.CreateTreeNode(ctx, TreeNodeRecord{
		ID:       folderID,
		ParentID: parentID,
		Kind:     "folder",
		Title:    &folderTitle,
		Position: position,
	}); err != nil {
		return BrowserNodeResponse{}, err
	}
	// Return the committed light node (incl. seq) so the caller can apply locally
	// under the seq guard.
	return s.repo.LightNode(ctx, folderID)
}

func (s *DefaultArtifactService) UpdateFolder(ctx context.Context, id string, patch FolderPatch) (FolderNode, error) {
	if err := RequireScope(ctx, ScopeArtifactsWrite); err != nil {
		return FolderNode{}, err
	}
	if strings.TrimSpace(id) == "" {
		return FolderNode{}, ErrNotFound
	}
	if patch.Title == nil {
		return s.repo.GetFolder(ctx, id)
	}
	title := strings.TrimSpace(*patch.Title)
	if title == "" {
		return FolderNode{}, BadRequest("title cannot be empty")
	}
	if err := s.repo.UpdateFolderTitle(ctx, id, title); err != nil {
		return FolderNode{}, err
	}
	updated, err := s.repo.GetFolder(ctx, id)
	if err != nil {
		return FolderNode{}, err
	}
	node, err := s.repo.LightNode(ctx, id)
	if err != nil {
		return FolderNode{}, err
	}
	updated.Seq = node.Seq
	return updated, nil
}

func (s *DefaultArtifactService) GetFolder(ctx context.Context, id string) (FolderNode, error) {
	if err := RequireScope(ctx, ScopeArtifactsRead); err != nil {
		return FolderNode{}, err
	}
	if strings.TrimSpace(id) == "" {
		return FolderNode{}, ErrNotFound
	}
	return s.repo.GetFolder(ctx, id)
}

func (s *DefaultArtifactService) FolderBreadcrumbs(ctx context.Context, id string) ([]BreadcrumbItem, error) {
	if err := RequireScope(ctx, ScopeArtifactsRead); err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" {
		return nil, ErrNotFound
	}
	if _, err := s.repo.GetFolder(ctx, id); err != nil {
		return nil, err
	}
	return s.repo.FolderBreadcrumbs(ctx, id)
}

