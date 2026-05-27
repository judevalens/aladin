package service

import (
	"context"
	"encoding/json"
	"strings"

	"aladin/backend_v2/internal/blocknote"
)

type PageService interface {
	Get(context.Context, string) (PageDocument, error)
	Save(context.Context, string, PageSaveInput) (PageDocument, error)
}

type PageRepository interface {
	GetArtifact(context.Context, string) (ArtifactResponse, error)
	// SavePageBlocks persists the page's blocks + derived searchText with
	// optimistic concurrency. expectedRev=0 disables the check.
	SavePageBlocks(ctx context.Context, artifactID string, blocks json.RawMessage, searchText string, expectedRev int64) (newRev int64, err error)
}

type PageDocument struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Blocks    json.RawMessage `json:"blocks"`
	Revision  int64           `json:"revision"`
	UpdatedAt string          `json:"updatedAt"`
}

type PageSaveInput struct {
	Blocks   json.RawMessage `json:"blocks"`
	Revision int64           `json:"revision"`
}

type DefaultPageService struct {
	repo     PageRepository
	realtime RealtimeEventService
}

func NewPageService(repo PageRepository, realtime ...RealtimeEventService) *DefaultPageService {
	var rt RealtimeEventService
	if len(realtime) > 0 {
		rt = realtime[0]
	}
	return &DefaultPageService{repo: repo, realtime: rt}
}

func (s *DefaultPageService) Get(ctx context.Context, id string) (PageDocument, error) {
	if err := RequireScope(ctx, ScopeArtifactsRead); err != nil {
		return PageDocument{}, err
	}
	rec, err := s.pageArtifact(ctx, id)
	if err != nil {
		return PageDocument{}, err
	}
	return toPageDocument(rec), nil
}

func (s *DefaultPageService) Save(ctx context.Context, id string, input PageSaveInput) (PageDocument, error) {
	if err := RequireScope(ctx, ScopeArtifactsWrite); err != nil {
		return PageDocument{}, err
	}
	if _, err := s.pageArtifact(ctx, id); err != nil {
		return PageDocument{}, err
	}
	if !looksLikeJSONArray(input.Blocks) {
		return PageDocument{}, BadRequest("blocks must be a JSON array")
	}
	searchText, err := blocknote.ExtractText(input.Blocks)
	if err != nil {
		return PageDocument{}, BadRequest("blocks: " + err.Error())
	}
	if _, err := s.repo.SavePageBlocks(ctx, id, input.Blocks, searchText, input.Revision); err != nil {
		return PageDocument{}, err
	}

	updated, err := s.pageArtifact(ctx, id)
	if err != nil {
		return PageDocument{}, err
	}
	page := toPageDocument(updated)
	s.publishWorkspaceEvent(ctx, "page", page.ID, "updated", page)
	return page, nil
}

func (s *DefaultPageService) pageArtifact(ctx context.Context, id string) (ArtifactResponse, error) {
	if strings.TrimSpace(id) == "" {
		return ArtifactResponse{}, ErrNotFound
	}
	rec, err := s.repo.GetArtifact(ctx, id)
	if err != nil {
		return ArtifactResponse{}, err
	}
	if rec.Type != "page" {
		return ArtifactResponse{}, ErrNotFound
	}
	return rec, nil
}

func (s *DefaultPageService) publishWorkspaceEvent(ctx context.Context, resourceKind string, resourceID string, operation string, payload any) {
	if s.realtime == nil {
		return
	}
	_ = s.realtime.Publish(ctx, PublishTarget{
		Stream:       WorkspaceStream,
		ResourceKind: resourceKind,
		ResourceID:   resourceID,
		Operation:    operation,
	}, payload)
}

func toPageDocument(rec ArtifactResponse) PageDocument {
	return PageDocument{
		ID:        rec.ID,
		Title:     rec.Title,
		Blocks:    rec.Blocks,
		Revision:  rec.Revision,
		UpdatedAt: rec.UpdatedAt,
	}
}
