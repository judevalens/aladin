package service

import (
	"context"
	"strings"
)

// documents.go — the read/write contract for ingested documents (design/INGESTION_PRD.md).
//
// Ingestion turns an uploaded file into text and structure. This is the layer the API and
// the worker talk to; the extraction itself lives in internal/ingestion, and the two are
// deliberately separate so a new format is a new extractor rather than a new pipeline.

// DocumentSection is one outline entry, read from the document's own bookmarks.
type DocumentSection struct {
	Title string `json:"title"`
	Level int    `json:"level"`
	Page  int    `json:"page"`
}

// DocumentPage is one page's extracted text.
type DocumentPage struct {
	Page int    `json:"page"`
	Text string `json:"text"`
}

// Document is an artifact's ingested content.
//
// Status is always present and always terminal-or-moving (§4) — a surface should never
// have to guess between "still working" and "quietly gave up".
type Document struct {
	ArtifactID string            `json:"artifactId"`
	Status     string            `json:"status"`
	Error      string            `json:"error,omitempty"`
	PageCount  int               `json:"pageCount"`
	Sections   []DocumentSection `json:"sections"`
	Pages      []DocumentPage    `json:"pages,omitempty"`
	Extractor  string            `json:"extractor,omitempty"`
}

// PendingDocument is an artifact the sweeper has claimed for ingestion. Path is resolved
// from the artifact's stored resource; the worker reads it and nothing else.
type PendingDocument struct {
	ArtifactID string
	UserID     string
	Path       string
	MIMEType   string
	Filename   string
}

// DocumentResult is what the worker writes back after extraction.
type DocumentResult struct {
	Status    string
	Error     string
	PageCount int
	Pages     []DocumentPage
	Sections  []DocumentSection
	Extractor string
}

type DocumentService interface {
	// Get returns an artifact's ingested document. WithPages controls whether the text
	// comes along — a tree row wants status, a viewer wants the words.
	Get(ctx context.Context, artifactID string, withPages bool) (Document, error)
}

type DocumentRepository interface {
	GetDocument(ctx context.Context, artifactID string, withPages bool) (Document, error)
	// ClaimPending atomically finds ingestible artifacts with no document row yet and
	// marks them 'ingesting', so two workers can't take the same file.
	ClaimPending(ctx context.Context, limit int) ([]PendingDocument, error)
	// SaveResult writes the outcome and emits the artifact's node frame, so open
	// surfaces refresh through the syncer rather than polling.
	SaveResult(ctx context.Context, artifactID, userID string, result DocumentResult) error
}

type documentService struct{ repo DocumentRepository }

// NewDocumentService returns the interface, never the concrete type.
func NewDocumentService(repo DocumentRepository) DocumentService {
	return &documentService{repo: repo}
}

func (s *documentService) Get(ctx context.Context, artifactID string, withPages bool) (Document, error) {
	if err := RequireScope(ctx, ScopeArtifactsRead); err != nil {
		return Document{}, err
	}
	if strings.TrimSpace(artifactID) == "" {
		return Document{}, ErrNotFound
	}
	return s.repo.GetDocument(ctx, artifactID, withPages)
}
