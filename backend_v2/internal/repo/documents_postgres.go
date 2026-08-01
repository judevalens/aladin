package repo

import (
	"context"
	"errors"
	"fmt"

	artifactservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// documents_postgres.go — persistence for ingested documents (design/INGESTION_PRD.md).

// ResourceResolver turns an artifact's stored storageKey into a path on disk.
// Satisfied by *FilesystemArtifactStore.
type ResourceResolver interface {
	ResourcePath(storageKey string) (string, error)
}

type PostgresDocumentRepository struct {
	pool  *pgxpool.Pool
	files ResourceResolver
}

func NewDocumentPostgres(pool *pgxpool.Pool, files ResourceResolver) *PostgresDocumentRepository {
	return &PostgresDocumentRepository{pool: pool, files: files}
}

func (r *PostgresDocumentRepository) GetDocument(ctx context.Context, artifactID string, withPages bool) (artifactservice.Document, error) {
	principal, err := artifactservice.RequirePrincipal(ctx)
	if err != nil {
		return artifactservice.Document{}, err
	}

	doc := artifactservice.Document{ArtifactID: artifactID, Sections: []artifactservice.DocumentSection{}}
	var errText *string
	err = r.pool.QueryRow(ctx, `
		SELECT status, error, page_count, extractor
		  FROM artifact_documents
		 WHERE artifact_id = $1 AND user_id = $2::uuid
	`, artifactID, principal.UserID).Scan(&doc.Status, &errText, &doc.PageCount, &doc.Extractor)
	if errors.Is(err, pgx.ErrNoRows) {
		return artifactservice.Document{}, artifactservice.ErrNotFound
	}
	if err != nil {
		return artifactservice.Document{}, fmt.Errorf("document %s: %w", artifactID, err)
	}
	if errText != nil {
		doc.Error = *errText
	}

	if withPages {
		if doc.Pages, err = r.GetPages(ctx, artifactID, 0, 0); err != nil {
			return artifactservice.Document{}, err
		}
	}

	rows, err := r.pool.Query(ctx, `
		SELECT title, level, page FROM artifact_sections
		 WHERE artifact_id = $1 ORDER BY position ASC
	`, artifactID)
	if err != nil {
		return artifactservice.Document{}, fmt.Errorf("document %s sections: %w", artifactID, err)
	}
	defer rows.Close()
	for rows.Next() {
		var section artifactservice.DocumentSection
		if err := rows.Scan(&section.Title, &section.Level, &section.Page); err != nil {
			return artifactservice.Document{}, err
		}
		doc.Sections = append(doc.Sections, section)
	}
	return doc, rows.Err()
}

// GetPages reads a page RANGE. from/to are 1-based and inclusive; 0 means unbounded.
//
// This exists because the previous shape (one jsonb blob) meant reading page 42 of a
// 400-page book deserialized the whole book.
func (r *PostgresDocumentRepository) GetPages(ctx context.Context, artifactID string, from, to int) ([]artifactservice.DocumentPage, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT page, text FROM artifact_pages
		 WHERE artifact_id = $1
		   AND ($2 = 0 OR page >= $2)
		   AND ($3 = 0 OR page <= $3)
		 ORDER BY page ASC
	`, artifactID, from, to)
	if err != nil {
		return nil, fmt.Errorf("document %s pages: %w", artifactID, err)
	}
	defer rows.Close()

	out := []artifactservice.DocumentPage{}
	for rows.Next() {
		var page artifactservice.DocumentPage
		if err := rows.Scan(&page.Page, &page.Text); err != nil {
			return nil, err
		}
		out = append(out, page)
	}
	return out, rows.Err()
}

// SearchDocument finds passages inside one document, newest Postgres full-text ranking,
// returning a highlighted snippet per page.
//
// This is the access pattern that makes a long document usable to a reader who has a
// QUESTION rather than a page number — which is every reader that isn't already holding
// the outline. ts_headline gives back a few hundred characters around the match instead
// of the page, so the answer stays proportional to the question.
func (r *PostgresDocumentRepository) SearchDocument(ctx context.Context, artifactID, query string, limit int) ([]artifactservice.DocumentHit, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT page,
		       ts_headline('english', text, q,
		           'MaxFragments=2, MaxWords=40, MinWords=15, StartSel=<<, StopSel=>>'),
		       ts_rank(tsv, q) AS score
		  FROM artifact_pages, websearch_to_tsquery('english', $2) AS q
		 WHERE artifact_id = $1 AND tsv @@ q
		 ORDER BY score DESC, page ASC
		 LIMIT $3
	`, artifactID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search document %s: %w", artifactID, err)
	}
	defer rows.Close()

	out := []artifactservice.DocumentHit{}
	for rows.Next() {
		var hit artifactservice.DocumentHit
		if err := rows.Scan(&hit.Page, &hit.Snippet, &hit.Score); err != nil {
			return nil, err
		}
		out = append(out, hit)
	}
	return out, rows.Err()
}

// ClaimPending finds file artifacts that look ingestible and have no document row yet,
// and claims them by inserting one with status 'ingesting'.
//
// Deriving the work list from "artifact exists, document row doesn't" rather than from a
// queue message means the upload path needs no changes at all, a lost enqueue can't
// strand a file, and PDFs uploaded before ingestion existed get picked up on their own.
// The INSERT is the claim: the primary key makes a double-claim impossible, so two
// workers racing is safe.
func (r *PostgresDocumentRepository) ClaimPending(ctx context.Context, limit int) ([]artifactservice.PendingDocument, error) {
	rows, err := r.pool.Query(ctx, `
		INSERT INTO artifact_documents (artifact_id, user_id, status)
		SELECT a.id, a.user_id, 'ingesting'
		  FROM artifacts a
		 WHERE a.type = 'file'
		   AND (a.metadata->>'mimeType' = 'application/pdf'
		        OR lower(a.metadata->>'originalFilename') LIKE '%.pdf')
		   AND a.metadata->>'storageKey' IS NOT NULL
		   AND NOT EXISTS (SELECT 1 FROM artifact_documents d WHERE d.artifact_id = a.id)
		 ORDER BY a.created_at ASC
		 LIMIT $1
		ON CONFLICT (artifact_id) DO NOTHING
		RETURNING artifact_id, user_id,
		          (SELECT metadata->>'storageKey'       FROM artifacts WHERE id = artifact_id),
		          (SELECT metadata->>'mimeType'         FROM artifacts WHERE id = artifact_id),
		          (SELECT metadata->>'originalFilename' FROM artifacts WHERE id = artifact_id)
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("claim pending documents: %w", err)
	}
	defer rows.Close()

	out := []artifactservice.PendingDocument{}
	for rows.Next() {
		var (
			pending    artifactservice.PendingDocument
			storageKey *string
			mime       *string
			filename   *string
		)
		if err := rows.Scan(&pending.ArtifactID, &pending.UserID, &storageKey, &mime, &filename); err != nil {
			return nil, err
		}
		if storageKey == nil {
			continue
		}
		path, err := r.files.ResourcePath(*storageKey)
		if err != nil {
			// Claimed but unreadable — record it rather than leaving the row 'ingesting'
			// forever, which would look identical to "still working".
			_ = r.markFailed(ctx, pending.ArtifactID, fmt.Sprintf("cannot resolve stored file: %v", err))
			continue
		}
		pending.Path = path
		if mime != nil {
			pending.MIMEType = *mime
		}
		if filename != nil {
			pending.Filename = *filename
		}
		out = append(out, pending)
	}
	return out, rows.Err()
}

func (r *PostgresDocumentRepository) markFailed(ctx context.Context, artifactID, message string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE artifact_documents SET status = 'failed', error = $2, updated_at = now()
		 WHERE artifact_id = $1
	`, artifactID, message)
	return err
}

// SaveResult writes the extraction outcome and the outline in one tx, then emits the
// artifact's node frame so open surfaces refresh through the syncer (never a poll).
func (r *PostgresDocumentRepository) SaveResult(ctx context.Context, artifactID, userID string, result artifactservice.DocumentResult) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := LockUser(ctx, tx, userID); err != nil {
		return err
	}

	var errText *string
	if result.Error != "" {
		errText = &result.Error
	}
	if _, err := tx.Exec(ctx, `
		UPDATE artifact_documents
		   SET status = $2, error = $3, page_count = $4, extractor = $5, updated_at = now()
		 WHERE artifact_id = $1
	`, artifactID, result.Status, errText, result.PageCount, result.Extractor); err != nil {
		return fmt.Errorf("save document %s: %w", artifactID, err)
	}

	// Re-extraction replaces the pages wholesale, same reasoning as the outline below.
	if _, err := tx.Exec(ctx, `DELETE FROM artifact_pages WHERE artifact_id = $1`, artifactID); err != nil {
		return err
	}
	for _, page := range result.Pages {
		if _, err := tx.Exec(ctx, `
			INSERT INTO artifact_pages (artifact_id, page, text) VALUES ($1, $2, $3)
		`, artifactID, page.Page, page.Text); err != nil {
			return fmt.Errorf("save page %d: %w", page.Page, err)
		}
	}

	// Re-ingestion replaces the outline wholesale; a partial merge would leave stale
	// entries from a previous extractor.
	if _, err := tx.Exec(ctx, `DELETE FROM artifact_sections WHERE artifact_id = $1`, artifactID); err != nil {
		return err
	}
	for position, section := range result.Sections {
		if _, err := tx.Exec(ctx, `
			INSERT INTO artifact_sections (artifact_id, title, level, page, position)
			VALUES ($1, $2, $3, $4, $5)
		`, artifactID, section.Title, section.Level, section.Page, position); err != nil {
			return fmt.Errorf("save section %d: %w", position, err)
		}
	}

	// The tree row shows ingestion status, so the artifact's node frame has to go out
	// with the write — same transaction, same spine as every other change.
	if err := emitNodeUpsert(ctx, tx, userID, artifactID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
