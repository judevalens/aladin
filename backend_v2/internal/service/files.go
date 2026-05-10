package service

import (
	"context"
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

type FileStore interface {
	SaveResource(string, string, io.Reader) (StoredArtifactResource, error)
	ResourcePath(string) (string, error)
}

type FileUploadInput struct {
	Filename string
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
	if _, err := RequirePrincipal(ctx); err != nil {
		return FileRecord{}, err
	}
	filename := strings.TrimSpace(input.Filename)
	if filename == "" {
		return FileRecord{}, BadRequest("filename is required")
	}
	stored, err := s.store.SaveResource("file", filename, body)
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
		return FileRecord{}, err
	}
	return rec, nil
}

func (s *DefaultFileService) Resource(ctx context.Context, id string) (FileResource, error) {
	if _, err := RequirePrincipal(ctx); err != nil {
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
