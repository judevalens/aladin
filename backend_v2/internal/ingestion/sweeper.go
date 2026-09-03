package ingestion

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"aladin/backend_v2/internal/document"
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
	ClaimPending(ctx context.Context, limit int) ([]document.PendingDocument, error)
	SaveResult(ctx context.Context, artifactID, userID string, result document.DocumentResult) error
}

// Sweeper claims un-ingested documents and extracts them.
type Sweeper struct {
	store     Store
	segmenter Segmenter
	log       *slog.Logger
	batch     int
}

func NewSweeper(store Store, segmenter Segmenter, log *slog.Logger) *Sweeper {
	return &Sweeper{store: store, segmenter: segmenter, log: log, batch: 5}
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
		result := s.extract(ctx, item)
		if err := s.store.SaveResult(ctx, item.ArtifactID, item.UserID, result); err != nil {
			s.log.Error("ingestion: save failed",
				"component", "ingestion", "artifact_id", item.ArtifactID, "err", err)
			continue
		}
		s.log.Info("ingestion: document processed",
			"component", "ingestion", "artifact_id", item.ArtifactID,
			"status", result.Status, "pages", result.PageCount,
			"sections", len(result.Sections), "regions", len(result.Regions),
			"chunks", len(result.Chunks))
		done++
	}
	return done, nil
}

// extract dispatches on format. Today there is one; the point of the seam is that a
// second is a new branch here plus a new extractor, not a new pipeline.
//
// Text, outline AND regions come from ONE pass (INGESTION_PRD §13). Splitting them across
// two extractors would mean resolving a region's box against text from a different
// coordinate model, turning an exact page anchor into an inference — and §13d spends the
// entire error budget on boundaries, none on anchors.
func (s *Sweeper) extract(ctx context.Context, item document.PendingDocument) document.DocumentResult {
	if !isPDF(item) {
		return document.DocumentResult{
			Status: string(StatusUnsupported),
			Error:  fmt.Sprintf("no extractor for %q", item.MIMEType),
		}
	}

	layout, err := s.segmenter.Segment(ctx, item.Path)
	if err != nil {
		// A missing venv, a crashed script, a timeout, unreadable output — each already
		// carries a message a human can act on, so surface it verbatim rather than
		// flattening it to "ingestion failed".
		return document.DocumentResult{Status: string(StatusFailed), Error: err.Error()}
	}
	doc := layout.ToDocument()

	sections := make([]document.DocumentSection, 0, len(doc.Sections))
	for _, section := range doc.Sections {
		sections = append(sections, document.DocumentSection(section))
	}
	pages := make([]document.DocumentPage, 0, len(doc.Pages))
	for _, page := range doc.Pages {
		pages = append(pages, document.DocumentPage(page))
	}
	regions := make([]document.DocumentRegion, 0)
	for _, page := range layout.Pages {
		for ordinal, region := range page.Regions {
			regions = append(regions, document.DocumentRegion{
				Page:       page.Page,
				Ordinal:    ordinal,
				Class:      region.Class,
				Confidence: region.Confidence,
				Bbox:       region.Bbox,
				Text:       region.Text,
			})
		}
	}

	// Regions become a navigable tree (§11). Flattened depth-first, with ParentID
	// carrying the parent's INDEX in this slice — the repo swaps each index for the row
	// id it just inserted, which works precisely because parents come first.
	chunks := make([]document.DocumentChunk, 0)
	FlattenChunks(BuildChunks(regions), func(chunk Chunk, parentIndex int) int {
		entry := document.DocumentChunk{
			Ordinal: chunk.Ordinal, Depth: chunk.Depth, Kind: string(chunk.Kind),
			Title: chunk.Title, PageFrom: chunk.PageFrom, PageTo: chunk.PageTo, Text: chunk.Text,
		}
		if parentIndex >= 0 {
			index := int64(parentIndex)
			entry.ParentID = &index
		}
		chunks = append(chunks, entry)
		return len(chunks) - 1
	})

	return document.DocumentResult{
		Status:    string(doc.Status),
		Error:     doc.Error,
		PageCount: doc.PageCount,
		Pages:     pages,
		Sections:  sections,
		Regions:   regions,
		Chunks:    chunks,
		Extractor: doc.Extractor,
	}
}

func isPDF(item document.PendingDocument) bool {
	if strings.EqualFold(item.MIMEType, "application/pdf") {
		return true
	}
	return strings.HasSuffix(strings.ToLower(item.Filename), ".pdf")
}
