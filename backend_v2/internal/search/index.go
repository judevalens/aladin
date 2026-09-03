package search

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aladin/backend_v2/internal/board"
)

// Content index — READABLE_WORKSPACE R1: readability is a per-kind contract, like
// syncability. Each artifact kind ships a PROJECTOR — a deterministic content →
// text-with-locators function, no LLM in the path — and the index is a rebuildable
// projection of those projections. Freshness is a staleness sweep against three source
// clocks (see repo.StaleArtifacts), because the sidecar writes boards and page bodies
// with direct SQL and no outbox frame ever fires for them.
//
// ADDING A READABLE KIND (the recipe — miss a step and the kind is silently invisible
// to every agent surface):
//  1. Write a projector func in this file; rows carry opaque locators a client can
//     reopen ("page:N", "block:<id>", "shape:<id>", "t:S").
//  2. Register it in newProjectors() under the artifacts.type string.
//  3. Extend TestContentProjectors with the new kind's shape.
//  4. If the kind's text lives outside artifacts/page_documents/artifact_documents,
//     add its clock to repo.StaleArtifacts' GREATEST — or its edits never re-project.

// ContentRow is one addressable span of an artifact's projection.
type ContentRow struct {
	Locator string
	Text    string
}

// StaleArtifact is an artifact whose source clock beats its last projection.
type StaleArtifact struct {
	ArtifactID  string
	UserID      string
	Kind        string
	SourceStamp time.Time
}

// ContentHit is one retrieval result — always citable (artifact + locator).
type ContentHit struct {
	ArtifactID string
	Kind       string
	Locator    string
	Title      string
	Snippet    string
	Score      float64
}

// NumberedPage is one page of an ingested document (projector source).
type NumberedPage struct {
	Page int
	Text string
}

// ContentIndexRepo is the storage the service drives (implemented in repo).
type ContentIndexRepo interface {
	StaleArtifacts(ctx context.Context, limit int) ([]StaleArtifact, error)
	ReplaceRows(ctx context.Context, target StaleArtifact, rows []ContentRow) error
	Search(ctx context.Context, userID, query string, limit int) ([]ContentHit, error)
	PageBlocks(ctx context.Context, artifactID string) (json.RawMessage, error)
	FilePages(ctx context.Context, artifactID string) ([]NumberedPage, error)
	ArtifactBody(ctx context.Context, artifactID string) (title, content string, err error)
}

type ContentIndexService interface {
	// Sweep projects up to limit stale artifacts; returns how many it re-indexed.
	// Driven by a worker ticker — cheap enough to run every ~30s for one user.
	Sweep(ctx context.Context, limit int) (int, error)
	// Search is lexical FTS over the whole index, best row per artifact.
	Search(ctx context.Context, userID, query string, limit int) ([]ContentHit, error)
}

type projector func(ctx context.Context, repo ContentIndexRepo, artifactID string) ([]ContentRow, error)

type defaultContentIndexService struct {
	repo       ContentIndexRepo
	projectors map[string]projector
}

func NewContentIndexService(repo ContentIndexRepo) ContentIndexService {
	return &defaultContentIndexService{repo: repo, projectors: newProjectors()}
}

func newProjectors() map[string]projector {
	return map[string]projector{
		"page":  projectPage,
		"file":  projectFile,
		"board": projectBoard,
		"link":  projectLink,
		// "voice" lands at R2 with transcription; "app" once the shard catalog merges.
	}
}

func (s *defaultContentIndexService) Sweep(ctx context.Context, limit int) (int, error) {
	stale, err := s.repo.StaleArtifacts(ctx, limit)
	if err != nil {
		return 0, err
	}
	indexed := 0
	for _, target := range stale {
		project, ok := s.projectors[target.Kind]
		if !ok {
			// Unprojectable kinds still get a state row (empty rows) so the sweep
			// doesn't rediscover them forever.
			if err := s.repo.ReplaceRows(ctx, target, nil); err != nil {
				return indexed, err
			}
			continue
		}
		rows, err := project(ctx, s.repo, target.ArtifactID)
		if err != nil {
			// One broken artifact must not stall the sweep; it stays stale and is
			// retried next tick (and keeps failing visibly in logs, not silently).
			return indexed, fmt.Errorf("content index: project %s (%s): %w", target.ArtifactID, target.Kind, err)
		}
		if err := s.repo.ReplaceRows(ctx, target, rows); err != nil {
			return indexed, err
		}
		indexed++
	}
	return indexed, nil
}

func (s *defaultContentIndexService) Search(ctx context.Context, userID, query string, limit int) ([]ContentHit, error) {
	query = strings.TrimSpace(query)
	if query == "" || userID == "" {
		return nil, nil
	}
	return s.repo.Search(ctx, userID, query, limit)
}

// --- projectors -------------------------------------------------------------

// projectPage: page_documents.blocks → one row per block, locator "block:<id>".
// Children are blocks with their own ids, so nesting flattens naturally.
func projectPage(ctx context.Context, repo ContentIndexRepo, artifactID string) ([]ContentRow, error) {
	blocks, err := repo.PageBlocks(ctx, artifactID)
	if err != nil {
		return nil, err
	}
	if len(blocks) == 0 {
		return nil, nil
	}
	var parsed []pageBlock
	if err := json.Unmarshal(blocks, &parsed); err != nil {
		return nil, nil // an unreadable projection indexes as empty, never errors the sweep
	}
	var rows []ContentRow
	var walk func([]pageBlock)
	walk = func(nodes []pageBlock) {
		for _, block := range nodes {
			if text := board.FlattenRichText(block.Content); text != "" && block.ID != "" {
				rows = append(rows, ContentRow{Locator: "block:" + block.ID, Text: text})
			}
			walk(block.Children)
		}
	}
	walk(parsed)
	return rows, nil
}

type pageBlock struct {
	ID       string          `json:"id"`
	Content  json.RawMessage `json:"content"`
	Children []pageBlock     `json:"children"`
}

// projectFile: artifact_pages → one row per page, locator "page:N" — the same locator
// the reader's wormhole already opens.
func projectFile(ctx context.Context, repo ContentIndexRepo, artifactID string) ([]ContentRow, error) {
	pages, err := repo.FilePages(ctx, artifactID)
	if err != nil {
		return nil, err
	}
	rows := make([]ContentRow, 0, len(pages))
	for _, page := range pages {
		text := strings.TrimSpace(page.Text)
		if text == "" {
			continue
		}
		rows = append(rows, ContentRow{Locator: fmt.Sprintf("page:%d", page.Page), Text: text})
	}
	return rows, nil
}

// projectBoard: the projected snapshot → one row per legible shape, locator "shape:<id>",
// through the SAME parser the MCP board summary renders from (board_lines.go).
func projectBoard(ctx context.Context, repo ContentIndexRepo, artifactID string) ([]ContentRow, error) {
	_, content, err := repo.ArtifactBody(ctx, artifactID)
	if err != nil {
		return nil, err
	}
	parsed := board.ParseContent(content)
	rows := make([]ContentRow, 0, len(parsed.Lines))
	for _, line := range parsed.Lines {
		rows = append(rows, ContentRow{Locator: "shape:" + line.ShapeID, Text: line.Text})
	}
	return rows, nil
}

// projectLink: title + URL today; the fetched, readable body arrives with R2's URL
// ingestion (the link artifact runs through the ingest engine like a PDF).
func projectLink(ctx context.Context, repo ContentIndexRepo, artifactID string) ([]ContentRow, error) {
	title, content, err := repo.ArtifactBody(ctx, artifactID)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(strings.Join(strings.Fields(title+" "+content), " "))
	if text == "" {
		return nil, nil
	}
	return []ContentRow{{Locator: "", Text: text}}, nil
}
