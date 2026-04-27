package service

import (
	"context"
	"io"
	"strings"
	"time"
)

type ArtifactService interface {
	List(context.Context, ArtifactListParams) ([]ArtifactResponse, error)
	Get(context.Context, string) (ArtifactResponse, error)
	Create(context.Context, ArtifactPayload) (ArtifactResponse, error)
	Update(context.Context, string, ArtifactPatch) (ArtifactResponse, error)
	Delete(context.Context, string) error
	Upload(context.Context, ArtifactUploadInput, io.Reader) (ArtifactResponse, error)
	Resource(context.Context, string) (ArtifactResource, error)
	ListFolders(context.Context, *string) ([]FolderNode, error)
	CreateFolder(context.Context, string, *string) (FolderNode, error)
	GetFolder(context.Context, string) (FolderNode, error)
	FolderBreadcrumbs(context.Context, string) ([]BreadcrumbItem, error)
}

type ArtifactRepository interface {
	ListArtifacts(context.Context, ArtifactListParams) ([]ArtifactResponse, error)
	GetArtifact(context.Context, string) (ArtifactResponse, error)
	CreateArtifact(context.Context, ArtifactResponse) error
	UpdateArtifact(context.Context, string, ArtifactPatch) error
	DeleteArtifact(context.Context, string) error
	ListFolders(context.Context, *string) ([]FolderNode, error)
	CreateFolder(context.Context, FolderNode) error
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
	repo  ArtifactRepository
	files ArtifactFileStore
}

func NewArtifactService(repo ArtifactRepository, files ArtifactFileStore) *DefaultArtifactService {
	return &DefaultArtifactService{repo: repo, files: files}
}

func (s *DefaultArtifactService) List(ctx context.Context, params ArtifactListParams) ([]ArtifactResponse, error) {
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

func (s *DefaultArtifactService) Get(ctx context.Context, id string) (ArtifactResponse, error) {
	if strings.TrimSpace(id) == "" {
		return ArtifactResponse{}, ErrNotFound
	}
	return s.repo.GetArtifact(ctx, id)
}

func (s *DefaultArtifactService) Create(ctx context.Context, payload ArtifactPayload) (ArtifactResponse, error) {
	artifactType := strings.TrimSpace(payload.Type)
	if artifactType == "" {
		artifactType = "note"
	}
	if artifactType != "note" && artifactType != "link" {
		return ArtifactResponse{}, BadRequest("type must be one of: note, link")
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
	case "note":
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
	return rec, nil
}

func (s *DefaultArtifactService) Update(ctx context.Context, id string, patch ArtifactPatch) (ArtifactResponse, error) {
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
		if trimmedType != "note" && trimmedType != "link" && trimmedType != "voice" && trimmedType != "file" {
			return ArtifactResponse{}, BadRequest("unsupported type")
		}
		patch.Type = &trimmedType
	}
	if patch.Title != nil && strings.TrimSpace(*patch.Title) == "" {
		return ArtifactResponse{}, BadRequest("title cannot be empty")
	}
	if patch.Content != nil {
		trimmedContent := strings.TrimSpace(*patch.Content)
		if current.Type == "note" && trimmedContent == "" {
			return ArtifactResponse{}, BadRequest("content cannot be empty")
		}
		patch.Content = &trimmedContent
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
	if nextType == "note" && strings.TrimSpace(nextTitle) == "" && strings.TrimSpace(nextContent) == "" {
		return ArtifactResponse{}, BadRequest("title or content is required")
	}

	if err := s.repo.UpdateArtifact(ctx, id, patch); err != nil {
		return ArtifactResponse{}, err
	}
	return s.repo.GetArtifact(ctx, id)
}

func (s *DefaultArtifactService) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrNotFound
	}
	return s.repo.DeleteArtifact(ctx, id)
}

func (s *DefaultArtifactService) Upload(ctx context.Context, input ArtifactUploadInput, body io.Reader) (ArtifactResponse, error) {
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
	parentID = TrimStringPtr(parentID)
	if parentID != nil {
		if _, err := s.repo.GetFolder(ctx, *parentID); err != nil {
			return nil, err
		}
	}
	return s.repo.ListFolders(ctx, parentID)
}

func (s *DefaultArtifactService) CreateFolder(ctx context.Context, title string, parentID *string) (FolderNode, error) {
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
	if err := s.repo.CreateFolder(ctx, folder); err != nil {
		return FolderNode{}, err
	}
	return folder, nil
}

func (s *DefaultArtifactService) GetFolder(ctx context.Context, id string) (FolderNode, error) {
	if strings.TrimSpace(id) == "" {
		return FolderNode{}, ErrNotFound
	}
	return s.repo.GetFolder(ctx, id)
}

func (s *DefaultArtifactService) FolderBreadcrumbs(ctx context.Context, id string) ([]BreadcrumbItem, error) {
	if strings.TrimSpace(id) == "" {
		return nil, ErrNotFound
	}
	if _, err := s.repo.GetFolder(ctx, id); err != nil {
		return nil, err
	}
	return s.repo.FolderBreadcrumbs(ctx, id)
}
