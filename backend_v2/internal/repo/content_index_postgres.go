package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Content index — READABLE_WORKSPACE R1. A derived projection of every readable artifact
// kind into addressable text rows (content_index) plus per-artifact staleness bookkeeping
// (content_index_state). Rebuildable, never canonical.

type ContentIndexPostgres struct {
	pool *pgxpool.Pool
}

func NewContentIndexPostgres(pool *pgxpool.Pool) *ContentIndexPostgres {
	return &ContentIndexPostgres{pool: pool}
}

// StaleArtifacts finds artifacts whose newest source clock beats what was last projected.
// Three clocks, because two writers bypass the Go layer entirely (the sidecar UPDATEs
// artifacts.content for boards and upserts page_documents for pages with direct SQL):
//   - artifacts.updated_at        title renames, board projections, link/voice edits
//   - page_documents.updated_at   collaborative page bodies
//   - artifact_documents.updated_at   ingestion finishing (pages/regions/chunks landing)
func (r *ContentIndexPostgres) StaleArtifacts(ctx context.Context, limit int) ([]service.StaleArtifact, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT a.id, a.user_id, a.type,
		       GREATEST(a.updated_at,
		                COALESCE(pd.updated_at, 'epoch'::timestamptz),
		                COALESCE(ad.updated_at, 'epoch'::timestamptz)) AS source_stamp
		  FROM artifacts a
		  LEFT JOIN page_documents pd ON pd.artifact_id = a.id
		  LEFT JOIN artifact_documents ad ON ad.artifact_id = a.id
		  LEFT JOIN content_index_state st ON st.artifact_id = a.id
		 WHERE GREATEST(a.updated_at,
		                COALESCE(pd.updated_at, 'epoch'::timestamptz),
		                COALESCE(ad.updated_at, 'epoch'::timestamptz))
		       > COALESCE(st.source_stamp, 'epoch'::timestamptz)
		 ORDER BY 4 ASC
		 LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("content index: stale scan: %w", err)
	}
	defer rows.Close()
	var out []service.StaleArtifact
	for rows.Next() {
		var s service.StaleArtifact
		if err := rows.Scan(&s.ArtifactID, &s.UserID, &s.Kind, &s.SourceStamp); err != nil {
			return nil, fmt.Errorf("content index: stale scan row: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ReplaceRows swaps an artifact's index rows atomically and records the source stamp that
// was observed BEFORE projection — a write racing the projection leaves its newer stamp
// in the source tables, so the next sweep re-projects.
func (r *ContentIndexPostgres) ReplaceRows(ctx context.Context, target service.StaleArtifact, contentRows []service.ContentRow) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("content index: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM content_index WHERE artifact_id = $1`, target.ArtifactID); err != nil {
		return fmt.Errorf("content index: clear rows: %w", err)
	}
	for i, row := range contentRows {
		if _, err := tx.Exec(ctx, `
			INSERT INTO content_index (user_id, artifact_id, kind, locator, ordinal, text)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			target.UserID, target.ArtifactID, target.Kind, row.Locator, i, row.Text,
		); err != nil {
			return fmt.Errorf("content index: insert row: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO content_index_state (artifact_id, source_stamp, indexed_at)
		VALUES ($1, $2, now())
		ON CONFLICT (artifact_id) DO UPDATE
		  SET source_stamp = EXCLUDED.source_stamp, indexed_at = now()`,
		target.ArtifactID, target.SourceStamp,
	); err != nil {
		return fmt.Errorf("content index: state upsert: %w", err)
	}
	return tx.Commit(ctx)
}

// Search runs lexical FTS over the index, best row per artifact (an artifact with ten
// matching blocks is one hit with its best locator, not ten rows crowding the section).
func (r *ContentIndexPostgres) Search(ctx context.Context, userID, query string, limit int) ([]service.ContentHit, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (ci.artifact_id)
		       ci.artifact_id, ci.kind, ci.locator, a.title,
		       ts_headline('english', ci.text, q, 'MaxWords=12, MinWords=4') AS snippet,
		       ts_rank(ci.tsv, q) AS score
		  FROM content_index ci
		  JOIN artifacts a ON a.id = ci.artifact_id,
		       websearch_to_tsquery('english', $2) AS q
		 WHERE ci.user_id = $1::uuid AND ci.tsv @@ q
		 ORDER BY ci.artifact_id, ts_rank(ci.tsv, q) DESC
		 LIMIT $3`, userID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("content index: search: %w", err)
	}
	defer rows.Close()
	var out []service.ContentHit
	for rows.Next() {
		var h service.ContentHit
		if err := rows.Scan(&h.ArtifactID, &h.Kind, &h.Locator, &h.Title, &h.Snippet, &h.Score); err != nil {
			return nil, fmt.Errorf("content index: search row: %w", err)
		}
		// ts_headline wraps matches in <b>…</b>; the command box renders plain text.
		h.Snippet = strings.NewReplacer("<b>", "", "</b>", "").Replace(h.Snippet)
		out = append(out, h)
	}
	return out, rows.Err()
}

// --- projector sources ------------------------------------------------------

// PageBlocks reads the page's projected block tree (page_documents.blocks).
func (r *ContentIndexPostgres) PageBlocks(ctx context.Context, artifactID string) (json.RawMessage, error) {
	var blocks json.RawMessage
	err := r.pool.QueryRow(ctx,
		`SELECT blocks FROM page_documents WHERE artifact_id = $1`, artifactID,
	).Scan(&blocks)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("content index: page blocks: %w", err)
	}
	return blocks, nil
}

// FilePages reads an ingested document's per-page text in order.
func (r *ContentIndexPostgres) FilePages(ctx context.Context, artifactID string) ([]service.NumberedPage, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT page, text FROM artifact_pages WHERE artifact_id = $1 ORDER BY page`, artifactID)
	if err != nil {
		return nil, fmt.Errorf("content index: file pages: %w", err)
	}
	defer rows.Close()
	var out []service.NumberedPage
	for rows.Next() {
		var p service.NumberedPage
		if err := rows.Scan(&p.Page, &p.Text); err != nil {
			return nil, fmt.Errorf("content index: file page row: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ArtifactBody reads the fields projectors need from the artifact row itself.
func (r *ContentIndexPostgres) ArtifactBody(ctx context.Context, artifactID string) (title, content string, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT title, content FROM artifacts WHERE id = $1`, artifactID,
	).Scan(&title, &content)
	if err == pgx.ErrNoRows {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("content index: artifact body: %w", err)
	}
	return title, content, nil
}
