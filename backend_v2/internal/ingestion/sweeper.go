package ingestion

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	coreservice "aladin/backend_v2/internal/service"
)

// sweeper.go — what drives extraction.
//
// There is no queue message. The sweeper asks the database for artifacts that look
// ingestible and have no document row yet, and claims them by inserting one. That choice
// buys three things a queue wouldn't:
//
//   - the upload path needs NO changes, so ingestion can't be forgotten at a new call site
//   - a lost enqueue can't strand a file — the work list is derived from state, not events
//   - files uploaded before ingestion existed get picked up on the next sweep
//
// The cost is latency: up to one sweep interval before a new upload starts extracting.
// For documents a human just dropped somewhere, seconds are fine.

// Store is the persistence the sweeper needs (satisfied by *repo.PostgresDocumentRepository).
type Store interface {
	ClaimPending(ctx context.Context, limit int) ([]coreservice.PendingDocument, error)
	SaveResult(ctx context.Context, artifactID, userID string, result coreservice.DocumentResult) error
}

// Sweeper claims un-ingested documents and extracts them.
type Sweeper struct {
	store Store
	log   *slog.Logger
	batch int
}

func NewSweeper(store Store, log *slog.Logger) *Sweeper {
	return &Sweeper{store: store, log: log, batch: 5}
}

// WithBatch bounds how many documents one sweep will extract. Extraction is CPU-bound
// and synchronous, so this is the knob that keeps a 20-file drop from monopolising the
// worker for a minute.
func (s *Sweeper) WithBatch(n int) *Sweeper {
	if n > 0 {
		s.batch = n
	}
	return s
}

// Sweep claims a batch and extracts each one. Returns how many were processed.
//
// A single document can never fail the sweep: extraction returns a Status rather than an
// error, and the panic-prone parsing is contained further down. Whatever happens, the
// row moves off 'ingesting' — a row stuck there is indistinguishable from a hang, which
// is the one outcome §4 exists to prevent.
func (s *Sweeper) Sweep(ctx context.Context) (int, error) {
	pending, err := s.store.ClaimPending(ctx, s.batch)
	if err != nil {
		return 0, fmt.Errorf("claim pending: %w", err)
	}

	done := 0
	for _, item := range pending {
		if ctx.Err() != nil {
			return done, ctx.Err()
		}
		result := s.extract(item)
		if err := s.store.SaveResult(ctx, item.ArtifactID, item.UserID, result); err != nil {
			s.log.Error("ingestion: save failed",
				"component", "ingestion", "artifact_id", item.ArtifactID, "err", err)
			continue
		}
		s.log.Info("ingestion: document processed",
			"component", "ingestion", "artifact_id", item.ArtifactID,
			"status", result.Status, "pages", result.PageCount, "sections", len(result.Sections))
		done++
	}
	return done, nil
}

// extract dispatches on format. Today there is one; the point of the seam is that a
// second is a new branch here plus a new Extract function, not a new pipeline.
func (s *Sweeper) extract(item coreservice.PendingDocument) coreservice.DocumentResult {
	if !isPDF(item) {
		return coreservice.DocumentResult{
			Status: string(StatusUnsupported),
			Error:  fmt.Sprintf("no extractor for %q", item.MIMEType),
		}
	}

	doc := ExtractPDF(item.Path)

	sections := make([]coreservice.DocumentSection, 0, len(doc.Sections))
	for _, section := range doc.Sections {
		sections = append(sections, coreservice.DocumentSection(section))
	}
	pages := make([]coreservice.DocumentPage, 0, len(doc.Pages))
	for _, page := range doc.Pages {
		pages = append(pages, coreservice.DocumentPage(page))
	}
	return coreservice.DocumentResult{
		Status:    string(doc.Status),
		Error:     doc.Error,
		PageCount: doc.PageCount,
		Pages:     pages,
		Sections:  sections,
		Extractor: doc.Extractor,
	}
}

func isPDF(item coreservice.PendingDocument) bool {
	if strings.EqualFold(item.MIMEType, "application/pdf") {
		return true
	}
	return strings.HasSuffix(strings.ToLower(item.Filename), ".pdf")
}
