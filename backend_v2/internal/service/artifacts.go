package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"time"
)

type ArtifactService interface {
	List(context.Context) ([]ArtifactResponse, error)
	Get(context.Context, string) (ArtifactResponse, error)
	Create(context.Context, ArtifactPayload) (ArtifactResponse, error)
	Update(context.Context, string, ArtifactPatch) (ArtifactResponse, error)
	Delete(context.Context, string) error
	CreateNote(context.Context, string, string) (NoteRecord, error)
	UpdateNote(context.Context, string, *string, *string) (NoteRecord, error)
	DeleteNote(context.Context, string) error
	RelatedNotes(context.Context, string, int) (RelatedNotesResponse, error)
	UploadDocument(context.Context, string, []byte) (DocumentRecord, error)
	ListDocuments(context.Context) ([]DocumentRecord, error)
	DocumentFilePath(string) (string, error)
	UploadAudio(context.Context, string, io.Reader) (AudioUploadRecord, error)
	AudioFilePath(string) (string, error)
}

type ArtifactRepository interface {
	ListArtifacts(context.Context) ([]ArtifactResponse, error)
	GetArtifact(context.Context, string) (ArtifactResponse, error)
	CreateArtifact(context.Context, ArtifactResponse) error
	UpdateArtifact(context.Context, string, ArtifactPatch) error
	DeleteArtifact(context.Context, string) error
	FindDocument(context.Context, string) (DocumentRecord, bool, error)
	CreateDocument(context.Context, DocumentRecord, int64) error
	ListDocuments(context.Context) ([]DocumentRecord, error)
	CreateNote(context.Context, NoteRecord) error
	NoteExists(context.Context, string) (bool, error)
	UpdateNote(context.Context, string, *string, *string) error
	DeleteNote(context.Context, string) error
	FetchNote(context.Context, string) (NoteRecord, error)
	HasNoteEmbedding(context.Context, string) (bool, error)
	RelatedNotes(context.Context, string, int) ([]RelatedRecord, error)
}

type ArtifactFileStore interface {
	SaveDocument(string, []byte) error
	DocumentPath(string) (string, error)
	SaveAudio(string, io.Reader) (string, string, error)
	AudioPath(string) (string, error)
}

type DefaultArtifactService struct {
	repo  ArtifactRepository
	files ArtifactFileStore
}

func NewArtifactService(repo ArtifactRepository, files ArtifactFileStore) *DefaultArtifactService {
	return &DefaultArtifactService{repo: repo, files: files}
}

func (s *DefaultArtifactService) List(ctx context.Context) ([]ArtifactResponse, error) {
	return s.repo.ListArtifacts(ctx)
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
	title := strings.TrimSpace(payload.Title)
	content := strings.TrimSpace(payload.Content)
	if title == "" {
		title = content
	}
	if title == "" && payload.SourceURL != nil {
		title = strings.TrimSpace(*payload.SourceURL)
	}
	if title == "" {
		return ArtifactResponse{}, BadRequest("title is required")
	}
	if content == "" {
		content = title
	}
	now := time.Now().UTC().Format(time.RFC3339)
	rec := ArtifactResponse{
		ID:        newID("artifact-"),
		Type:      artifactType,
		Title:     title,
		Content:   content,
		Summary:   TrimStringPtr(payload.Summary),
		SourceURL: TrimStringPtr(payload.SourceURL),
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
	if patch.Title != nil && strings.TrimSpace(*patch.Title) == "" {
		return ArtifactResponse{}, BadRequest("title cannot be empty")
	}
	if patch.Content != nil && strings.TrimSpace(*patch.Content) == "" {
		return ArtifactResponse{}, BadRequest("content cannot be empty")
	}
	if patch.Type != nil && strings.TrimSpace(*patch.Type) == "" {
		return ArtifactResponse{}, BadRequest("type cannot be empty")
	}
	patch.Type = TrimStringPtr(patch.Type)
	patch.Title = TrimStringPtr(patch.Title)
	patch.Content = TrimStringPtr(patch.Content)
	patch.Summary = TrimStringPtr(patch.Summary)
	patch.SourceURL = TrimStringPtr(patch.SourceURL)
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

func (s *DefaultArtifactService) CreateNote(ctx context.Context, label string, content string) (NoteRecord, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return NoteRecord{}, BadRequest("content is required")
	}
	label = strings.TrimSpace(label)
	if label == "" {
		label = content
		if len(label) > 60 {
			label = label[:60] + "..."
		}
		label = strings.ReplaceAll(label, "\n", " ")
	}
	now := time.Now().UTC()
	rec := NoteRecord{
		ID:        newID("note-"),
		Type:      "note",
		Label:     label,
		Content:   content,
		Metadata:  map[string]any{},
		Status:    "pending",
		CreatedAt: now.Format(time.RFC3339),
	}
	return rec, s.repo.CreateNote(ctx, rec)
}

func (s *DefaultArtifactService) UpdateNote(ctx context.Context, id string, label *string, content *string) (NoteRecord, error) {
	if strings.TrimSpace(id) == "" {
		return NoteRecord{}, ErrNotFound
	}
	label = TrimStringPtr(label)
	content = TrimStringPtr(content)
	if err := s.repo.UpdateNote(ctx, id, label, content); err != nil {
		return NoteRecord{}, err
	}
	return s.repo.FetchNote(ctx, id)
}

func (s *DefaultArtifactService) DeleteNote(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrNotFound
	}
	return s.repo.DeleteNote(ctx, id)
}

func (s *DefaultArtifactService) RelatedNotes(ctx context.Context, id string, limit int) (RelatedNotesResponse, error) {
	if strings.TrimSpace(id) == "" {
		return RelatedNotesResponse{}, ErrNotFound
	}
	exists, err := s.repo.NoteExists(ctx, id)
	if err != nil {
		return RelatedNotesResponse{}, err
	}
	if !exists {
		return RelatedNotesResponse{}, ErrNotFound
	}
	hasEmbedding, err := s.repo.HasNoteEmbedding(ctx, id)
	if err != nil {
		return RelatedNotesResponse{}, err
	}
	if !hasEmbedding {
		return RelatedNotesResponse{
			Results: []RelatedRecord{},
			Message: "Note not yet embedded",
		}, nil
	}
	results, err := s.repo.RelatedNotes(ctx, id, limit)
	if err != nil {
		return RelatedNotesResponse{}, err
	}
	return RelatedNotesResponse{Results: results}, nil
}

func (s *DefaultArtifactService) UploadDocument(ctx context.Context, filename string, content []byte) (DocumentRecord, error) {
	sum := sha256.Sum256(content)
	docID := "sha256:" + hex.EncodeToString(sum[:])

	existing, found, err := s.repo.FindDocument(ctx, docID)
	if err != nil {
		return DocumentRecord{}, err
	}
	if found {
		existing.Status = "already_exists"
		return existing, nil
	}

	if err := s.files.SaveDocument(docID, content); err != nil {
		return DocumentRecord{}, err
	}

	size := int64(len(content))
	now := time.Now().UTC()
	rec := DocumentRecord{
		ID:         docID,
		Filename:   filename,
		Title:      nil,
		Size:       &size,
		PageCount:  nil,
		UploadedAt: now.Format(time.RFC3339),
		Ingested:   true,
		Status:     "ingested",
	}
	if err := s.repo.CreateDocument(ctx, rec, now.Unix()); err != nil {
		return DocumentRecord{}, err
	}
	return rec, nil
}

func (s *DefaultArtifactService) ListDocuments(ctx context.Context) ([]DocumentRecord, error) {
	return s.repo.ListDocuments(ctx)
}

func (s *DefaultArtifactService) DocumentFilePath(id string) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "", ErrNotFound
	}
	return s.files.DocumentPath(id)
}

func (s *DefaultArtifactService) UploadAudio(ctx context.Context, filename string, body io.Reader) (AudioUploadRecord, error) {
	_ = ctx
	storedFilename, contentType, err := s.files.SaveAudio(filename, body)
	if err != nil {
		return AudioUploadRecord{}, err
	}
	return AudioUploadRecord{
		Filename:    storedFilename,
		URL:         "/api/audio/" + storedFilename,
		ContentType: contentType,
	}, nil
}

func (s *DefaultArtifactService) AudioFilePath(filename string) (string, error) {
	return s.files.AudioPath(filename)
}
