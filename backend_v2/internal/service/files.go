package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

type FileService interface {
	Upload(context.Context, FileUploadInput, io.Reader) (FileRecord, error)
	Resource(context.Context, string) (FileResource, error)
}

type FileRepository interface {
	CreateFile(context.Context, FileRecord) error
	GetFile(context.Context, string) (FileRecord, error)
}

// FileStore is the same byte-store port used by artifact resources. Keeping one
// contract prevents the legacy file endpoint and artifact upload path from
// acquiring different compensation behavior.
type FileStore = ArtifactFileStore

type FileUploadInput struct {
	Filename    string
	ContentType string
}

type FileRecord struct {
	ID         string `json:"id"`
	URL        string `json:"url"`
	UploadedAt string `json:"uploadedAt"`
	StorageKey string `json:"-"`
}

type FileResource struct {
	Path        string
	ContentType string
}

type DefaultFileService struct {
	repo  FileRepository
	store FileStore
}

func NewFileService(repo FileRepository, store FileStore) *DefaultFileService {
	return &DefaultFileService{repo: repo, store: store}
}

func (s *DefaultFileService) Upload(ctx context.Context, input FileUploadInput, body io.Reader) (FileRecord, error) {
	if err := RequireScope(ctx, ScopeArtifactsWrite); err != nil {
		return FileRecord{}, err
	}
	filename := strings.TrimSpace(input.Filename)
	if filename == "" {
		return FileRecord{}, BadRequest("filename is required")
	}
	stored, err := s.store.SaveResource("file", filename, input.ContentType, body)
	if err != nil {
		return FileRecord{}, err
	}
	rec := FileRecord{
		ID:         newID("file-"),
		UploadedAt: time.Now().UTC().Format(time.RFC3339),
		StorageKey: stored.StorageKey,
	}
	rec.URL = fileResourceURL(rec.ID)
	if err := s.repo.CreateFile(ctx, rec); err != nil {
		return FileRecord{}, cleanupStoredResource(s.store, stored.StorageKey, err)
	}
	return rec, nil
}

func cleanupStoredResource(store ArtifactFileStore, storageKey string, operationErr error) error {
	if cleanupErr := store.DeleteResource(storageKey); cleanupErr != nil {
		return errors.Join(operationErr, fmt.Errorf("compensate stored resource %q: %w", storageKey, cleanupErr))
	}
	return operationErr
}

func (s *DefaultFileService) Resource(ctx context.Context, id string) (FileResource, error) {
	if err := RequireScope(ctx, ScopeArtifactsRead); err != nil {
		return FileResource{}, err
	}
	if strings.TrimSpace(id) == "" {
		return FileResource{}, ErrNotFound
	}
	rec, err := s.repo.GetFile(ctx, id)
	if err != nil {
		return FileResource{}, err
	}
	path, err := s.store.ResourcePath(rec.StorageKey)
	if err != nil {
		return FileResource{}, err
	}
	return FileResource{Path: path}, nil
}

func fileResourceURL(id string) string {
	return "/api/files/" + id + "/resource"
}
