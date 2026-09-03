package page

import (
	"aladin/backend_v2/internal/artifact"
	"context"
	"encoding/json"
	"strings"

	"aladin/backend_v2/internal/apperror"
	"aladin/backend_v2/internal/auth"
)

type Service interface {
	Get(context.Context, string) (PageDocument, error)
	Save(context.Context, string, PageSaveInput) (PageDocument, error)
	// Attribution returns the page's block-level agent-attribution map
	// ({blockId: {by, at}}) as raw JSON ("{}" when none).
	Attribution(context.Context, string) (json.RawMessage, error)
	// History returns the page's edit history (humans + agents), newest first.
	History(context.Context, string) ([]PageEditEntry, error)
	// Diff returns the before/after markdown for one history entry (the entry's
	// snapshot vs the previous entry's), for a view-time text diff.
	Diff(context.Context, string) (PageDiff, error)
}

type Repository interface {
	GetArtifact(context.Context, string) (artifact.ArtifactResponse, error)
	// SavePageBlocks persists the page's blocks + derived searchText with
	// optimistic concurrency. expectedRev=0 disables the check.
	SavePageBlocks(ctx context.Context, artifactID string, blocks json.RawMessage, searchText string, expectedRev int64) (newRev int64, err error)
	// PageBlockAttribution returns the raw block_attribution JSON for a page.
	PageBlockAttribution(context.Context, string) (json.RawMessage, error)
	// PageEditHistory returns the page's coalesced edit sessions, newest first.
	PageEditHistory(context.Context, string) ([]PageEditEntry, error)
	// PageEditDiff returns the before/after markdown for a history entry id.
	PageEditDiff(context.Context, string) (PageDiff, error)
}

// PageEditEntry is one coalesced edit session on a page (the collab server
// records these; see docs/page-edit-history.md).
type PageEditEntry struct {
	ID         string `json:"id"`
	EditorKind string `json:"editorKind"` // "human" | "agent"
	EditorName string `json:"editorName"`
	OccurredAt string `json:"occurredAt"`
	EndedAt    string `json:"endedAt"`
	Edits      int    `json:"edits"`
}

// PageDiff is the before/after markdown around one history entry — the client
// renders a text diff from it.
type PageDiff struct {
	Before string `json:"before"`
	After  string `json:"after"`
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

type DefaultService struct {
	repo Repository
}

func NewService(repo Repository) *DefaultService {
	return &DefaultService{repo: repo}
}

func (s *DefaultService) Get(ctx context.Context, id string) (PageDocument, error) {
	if err := auth.RequireScope(ctx, auth.ScopeArtifactsRead); err != nil {
		return PageDocument{}, err
	}
	rec, err := s.pageArtifact(ctx, id)
	if err != nil {
		return PageDocument{}, err
	}
	return toPageDocument(rec), nil
}

func (s *DefaultService) Attribution(ctx context.Context, id string) (json.RawMessage, error) {
	if err := auth.RequireScope(ctx, auth.ScopeArtifactsRead); err != nil {
		return nil, err
	}
	// Verify it's a page the caller owns before exposing its attribution.
	if _, err := s.pageArtifact(ctx, id); err != nil {
		return nil, err
	}
	return s.repo.PageBlockAttribution(ctx, id)
}

func (s *DefaultService) History(ctx context.Context, id string) ([]PageEditEntry, error) {
	if err := auth.RequireScope(ctx, auth.ScopeArtifactsRead); err != nil {
		return nil, err
	}
	if _, err := s.pageArtifact(ctx, id); err != nil {
		return nil, err
	}
	return s.repo.PageEditHistory(ctx, id)
}

func (s *DefaultService) Diff(ctx context.Context, entryID string) (PageDiff, error) {
	if err := auth.RequireScope(ctx, auth.ScopeArtifactsRead); err != nil {
		return PageDiff{}, err
	}
	// Ownership is enforced in the repo (the entry → page → artifact.user_id join).
	return s.repo.PageEditDiff(ctx, entryID)
}

// Save is an M8c seam guard. Page content is owned by the collaborative Y.Doc
// (Hocuspocus), so a direct block write through PATCH /api/pages would diverge
// from — or be clobbered by — the live doc + its projection. The editor edits
// via the Y.Doc and agents via the MCP collab bridge. This path is dormant
// (the editor no longer calls it; usePageState is orphaned) and is removed
// wholesale in M8d; it is refused here meanwhile so it can never silently
// clobber collab state.
func (s *DefaultService) Save(ctx context.Context, id string, _ PageSaveInput) (PageDocument, error) {
	if err := auth.RequireScope(ctx, auth.ScopeArtifactsWrite); err != nil {
		return PageDocument{}, err
	}
	if _, err := s.pageArtifact(ctx, id); err != nil {
		return PageDocument{}, err
	}
	return PageDocument{}, apperror.BadRequest("page blocks are edited collaboratively, not via PATCH /api/pages")
}

func (s *DefaultService) pageArtifact(ctx context.Context, id string) (artifact.ArtifactResponse, error) {
	if strings.TrimSpace(id) == "" {
		return artifact.ArtifactResponse{}, apperror.ErrNotFound
	}
	rec, err := s.repo.GetArtifact(ctx, id)
	if err != nil {
		return artifact.ArtifactResponse{}, err
	}
	if rec.Type != "page" {
		return artifact.ArtifactResponse{}, apperror.ErrNotFound
	}
	return rec, nil
}

func toPageDocument(rec artifact.ArtifactResponse) PageDocument {
	return PageDocument{
		ID:        rec.ID,
		Title:     rec.Title,
		Blocks:    rec.Blocks,
		Revision:  rec.Revision,
		UpdatedAt: rec.UpdatedAt,
	}
}
