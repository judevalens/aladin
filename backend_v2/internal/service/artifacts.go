package service

import (
	"context"
	"io"
	"strings"
	"time"
)

type ArtifactService interface {
	List(context.Context, ArtifactListParams) ([]ArtifactResponse, error)
	BrowserTree(context.Context) ([]BrowserTreeNode, error)
	Get(context.Context, string) (ArtifactResponse, error)
	Create(context.Context, ArtifactPayload) (ArtifactResponse, error)
	Update(context.Context, string, ArtifactPatch) (ArtifactResponse, error)
	Delete(context.Context, string) error
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
	GetArtifact(context.Context, string) (ArtifactResponse, error)
	CreateArtifact(context.Context, ArtifactResponse) error
	UpdateArtifact(context.Context, string, ArtifactPatch) error
	CreatePageDocument(context.Context, string, string) error
	SavePageDocument(context.Context, string, string) error
	DeleteArtifact(context.Context, string) error
	ListFolders(context.Context, *string) ([]FolderNode, error)
	ListAllFolders(context.Context) ([]FolderNode, error)
	ListAllBrowserNodes(context.Context) ([]BrowserTreeFlatNode, error)
	NextNodePosition(context.Context, *string) (int64, error)
	CreateTreeNode(context.Context, TreeNodeRecord) error
	UpdateArtifactNodeParent(context.Context, string, *string) error
	UpdateFolderTitle(context.Context, string, string) error
	GetFolder(context.Context, string) (FolderNode, error)
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
	SaveResource(string, string, io.Reader) (StoredArtifactResource, error)
	ResourcePath(string) (string, error)
}

type DefaultArtifactService struct {
	repo     ArtifactRepository
	files    ArtifactFileStore
	realtime RealtimeEventService
}

func NewArtifactService(repo ArtifactRepository, files ArtifactFileStore, realtime ...RealtimeEventService) *DefaultArtifactService {
	var rt RealtimeEventService
	if len(realtime) > 0 {
		rt = realtime[0]
	}
	return &DefaultArtifactService{repo: repo, files: files, realtime: rt}
}

func (s *DefaultArtifactService) List(ctx context.Context, params ArtifactListParams) ([]ArtifactResponse, error) {
	if _, err := RequirePrincipal(ctx); err != nil {
		return nil, err
	}
	if params.FolderID != nil {
		params.FolderID = TrimStringPtr(params.FolderID)
		if params.FolderID != nil {
			if _, err := s.repo.GetFolder(ctx, *params.FolderID); err != nil {
				return nil, err
			}
		}
	}
	return s.repo.ListArtifacts(ctx, params)
}

func (s *DefaultArtifactService) BrowserTree(ctx context.Context) ([]BrowserTreeNode, error) {
	if _, err := RequirePrincipal(ctx); err != nil {
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
	if _, err := RequirePrincipal(ctx); err != nil {
		return ArtifactResponse{}, err
	}
	if strings.TrimSpace(id) == "" {
		return ArtifactResponse{}, ErrNotFound
	}
	return s.repo.GetArtifact(ctx, id)
}

func (s *DefaultArtifactService) Create(ctx context.Context, payload ArtifactPayload) (ArtifactResponse, error) {
	if _, err := RequirePrincipal(ctx); err != nil {
		return ArtifactResponse{}, err
	}
	artifactType := strings.TrimSpace(payload.Type)
	if artifactType == "" {
		artifactType = "page"
	}
	if artifactType != "page" && artifactType != "link" {
		return ArtifactResponse{}, BadRequest("type must be one of: page, link")
	}
	payload.FolderID = TrimStringPtr(payload.FolderID)
	if payload.FolderID != nil {
		if _, err := s.repo.GetFolder(ctx, *payload.FolderID); err != nil {
			return ArtifactResponse{}, err
		}
	}

	title := strings.TrimSpace(payload.Title)
	content := strings.TrimSpace(payload.Content)
	sourceURL := TrimStringPtr(payload.SourceURL)

	switch artifactType {
	case "page":
		if title == "" {
			title = content
		}
		if title == "" {
			return ArtifactResponse{}, BadRequest("title or content is required")
		}
		if content == "" {
			content = title
		}
	case "link":
		if sourceURL == nil {
			return ArtifactResponse{}, BadRequest("sourceUrl is required")
		}
		if title == "" {
			return ArtifactResponse{}, BadRequest("title is required")
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	rec := ArtifactResponse{
		ID:        newID("artifact-"),
		Type:      artifactType,
		FolderID:  payload.FolderID,
		Title:     title,
		Content:   content,
		Summary:   TrimStringPtr(payload.Summary),
		SourceURL: sourceURL,
		Metadata:  payload.Metadata,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if rec.Metadata == nil {
		rec.Metadata = map[string]any{}
	}
	if err := s.repo.CreateArtifact(ctx, rec); err != nil {
		return ArtifactResponse{}, err
	}
	if artifactType == "page" {
		if err := s.repo.CreatePageDocument(ctx, rec.ID, content); err != nil {
			return ArtifactResponse{}, err
		}
	}
	position, err := s.repo.NextNodePosition(ctx, payload.FolderID)
	if err != nil {
		return ArtifactResponse{}, err
	}
	artifactID := rec.ID
	if err := s.repo.CreateTreeNode(ctx, TreeNodeRecord{
		ID:         rec.ID,
		ParentID:   payload.FolderID,
		Kind:       "artifact",
		ArtifactID: &artifactID,
		Position:   position,
	}); err != nil {
		return ArtifactResponse{}, err
	}
	s.publishWorkspaceEvent(ctx, "artifact", rec.ID, "created", rec)
	return rec, nil
}

func (s *DefaultArtifactService) Update(ctx context.Context, id string, patch ArtifactPatch) (ArtifactResponse, error) {
	if _, err := RequirePrincipal(ctx); err != nil {
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
		if trimmedType != "page" && trimmedType != "link" && trimmedType != "voice" && trimmedType != "file" {
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
		if _, err := s.repo.GetFolder(ctx, *patch.FolderID); err != nil {
			return ArtifactResponse{}, err
		}
	}

	nextType := current.Type
	if patch.Type != nil {
		nextType = *patch.Type
	}
	nextTitle := current.Title
	if patch.Title != nil {
		nextTitle = *patch.Title
	}
	nextContent := current.Content
	if patch.Content != nil {
		nextContent = *patch.Content
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
	if nextType == "page" && strings.TrimSpace(nextTitle) == "" && strings.TrimSpace(nextContent) == "" {
		return ArtifactResponse{}, BadRequest("title or content is required")
	}

	if patch.FolderID != nil {
		if err := s.repo.UpdateArtifactNodeParent(ctx, id, patch.FolderID); err != nil {
			return ArtifactResponse{}, err
		}
	}
	if current.Type == "page" && patch.Content != nil {
		if err := s.repo.SavePageDocument(ctx, id, *patch.Content); err != nil {
			return ArtifactResponse{}, err
		}
	}
	if err := s.repo.UpdateArtifact(ctx, id, patch); err != nil {
		return ArtifactResponse{}, err
	}
	updated, err := s.repo.GetArtifact(ctx, id)
	if err != nil {
		return ArtifactResponse{}, err
	}
	s.publishWorkspaceEvent(ctx, "artifact", updated.ID, "updated", updated)
	return updated, nil
}

func (s *DefaultArtifactService) Delete(ctx context.Context, id string) error {
	if _, err := RequirePrincipal(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return ErrNotFound
	}
	if err := s.repo.DeleteArtifact(ctx, id); err != nil {
		return err
	}
	s.publishWorkspaceEvent(ctx, "artifact", id, "deleted", map[string]string{"id": id})
	return nil
}

func (s *DefaultArtifactService) Upload(ctx context.Context, input ArtifactUploadInput, body io.Reader) (ArtifactResponse, error) {
	if _, err := RequirePrincipal(ctx); err != nil {
		return ArtifactResponse{}, err
	}
	artifactType := strings.TrimSpace(input.Type)
	if artifactType != "voice" && artifactType != "file" {
		return ArtifactResponse{}, BadRequest("type must be one of: voice, file")
	}
	input.FolderID = TrimStringPtr(input.FolderID)
	if input.FolderID != nil {
		if _, err := s.repo.GetFolder(ctx, *input.FolderID); err != nil {
			return ArtifactResponse{}, err
		}
	}
	filename := strings.TrimSpace(input.Filename)
	if filename == "" {
		return ArtifactResponse{}, BadRequest("filename is required")
	}
	stored, err := s.files.SaveResource(artifactType, filename, body)
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
	if err := s.repo.CreateArtifact(ctx, rec); err != nil {
		return ArtifactResponse{}, err
	}
	position, err := s.repo.NextNodePosition(ctx, input.FolderID)
	if err != nil {
		return ArtifactResponse{}, err
	}
	artifactID := rec.ID
	if err := s.repo.CreateTreeNode(ctx, TreeNodeRecord{
		ID:         rec.ID,
		ParentID:   input.FolderID,
		Kind:       "artifact",
		ArtifactID: &artifactID,
		Position:   position,
	}); err != nil {
		return ArtifactResponse{}, err
	}
	s.publishWorkspaceEvent(ctx, "artifact", rec.ID, "created", rec)
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
	if _, err := RequirePrincipal(ctx); err != nil {
		return nil, err
	}
	parentID = TrimStringPtr(parentID)
	if parentID != nil {
		if _, err := s.repo.GetFolder(ctx, *parentID); err != nil {
			return nil, err
		}
	}
	return s.repo.ListFolders(ctx, parentID)
}

func (s *DefaultArtifactService) FolderTree(ctx context.Context) ([]FolderTreeNode, error) {
	if _, err := RequirePrincipal(ctx); err != nil {
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
	if _, err := RequirePrincipal(ctx); err != nil {
		return FolderNode{}, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return FolderNode{}, BadRequest("title is required")
	}
	parentID = TrimStringPtr(parentID)
	if parentID != nil {
		if _, err := s.repo.GetFolder(ctx, *parentID); err != nil {
			return FolderNode{}, err
		}
	}
	folder := FolderNode{
		ID:       newID("folder-"),
		ParentID: parentID,
		Title:    title,
	}
	position, err := s.repo.NextNodePosition(ctx, parentID)
	if err != nil {
		return FolderNode{}, err
	}
	folderTitle := folder.Title
	if err := s.repo.CreateTreeNode(ctx, TreeNodeRecord{
		ID:       folder.ID,
		ParentID: parentID,
		Kind:     "folder",
		Title:    &folderTitle,
		Position: position,
	}); err != nil {
		return FolderNode{}, err
	}
	s.publishWorkspaceEvent(ctx, "folder", folder.ID, "created", folder)
	return folder, nil
}

func (s *DefaultArtifactService) UpdateFolder(ctx context.Context, id string, patch FolderPatch) (FolderNode, error) {
	if _, err := RequirePrincipal(ctx); err != nil {
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
	s.publishWorkspaceEvent(ctx, "folder", updated.ID, "updated", updated)
	return updated, nil
}

func (s *DefaultArtifactService) GetFolder(ctx context.Context, id string) (FolderNode, error) {
	if _, err := RequirePrincipal(ctx); err != nil {
		return FolderNode{}, err
	}
	if strings.TrimSpace(id) == "" {
		return FolderNode{}, ErrNotFound
	}
	return s.repo.GetFolder(ctx, id)
}

func (s *DefaultArtifactService) FolderBreadcrumbs(ctx context.Context, id string) ([]BreadcrumbItem, error) {
	if _, err := RequirePrincipal(ctx); err != nil {
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

func (s *DefaultArtifactService) publishWorkspaceEvent(ctx context.Context, resourceKind string, resourceID string, operation string, payload any) {
	if s.realtime == nil {
		return
	}
	_ = s.realtime.Publish(ctx, PublishTarget{
		TenantID:     DefaultRealtimeTenantID,
		Stream:       WorkspaceStream,
		ResourceKind: resourceKind,
		ResourceID:   resourceID,
		Operation:    operation,
	}, payload)
}
