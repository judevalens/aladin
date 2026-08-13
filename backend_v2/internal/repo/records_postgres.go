package repo

import (
	"context"
	"errors"
	"fmt"

	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRecordRepository struct{ pool *pgxpool.Pool }

func NewRecordPostgres(pool *pgxpool.Pool) *PostgresRecordRepository {
	return &PostgresRecordRepository{pool: pool}
}

func (r *PostgresRecordRepository) List(ctx context.Context) ([]coreservice.RecordResponse, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
		    a.id, a.type, a.label, a.content, a.source_url,
		    a.parent_id, a.enrichment, a.metadata, a.created_at,
		    COUNT(c.id) AS child_count
		  FROM records a
		  LEFT JOIN records c ON c.parent_id = a.id AND c.status != 'superseded'
		 WHERE a.parent_id IS NULL
		   AND a.status != 'superseded'
		 GROUP BY a.id, a.type, a.label, a.content, a.source_url, a.parent_id,
		          a.enrichment, a.metadata, a.created_at
		 ORDER BY a.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []coreservice.RecordResponse
	for rows.Next() {
		rec, err := scanRecordRow(rows, true)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *PostgresRecordRepository) Children(ctx context.Context, id string, limit int, offset int) (map[string]any, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
		    a.id, a.type, a.label, a.content, a.source_url,
		    a.parent_id, a.enrichment, a.metadata, a.created_at,
		    COUNT(c.id) AS child_count
		  FROM records a
		  LEFT JOIN records c ON c.parent_id = a.id AND c.status != 'superseded'
		 WHERE a.parent_id = $1
		   AND a.status != 'superseded'
		 GROUP BY a.id, a.type, a.label, a.content, a.source_url, a.parent_id,
		          a.enrichment, a.metadata, a.created_at
		 ORDER BY COALESCE((a.metadata->>'score')::int, 0) DESC, a.created_at DESC
		 LIMIT $2 OFFSET $3
	`, id, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []coreservice.RecordResponse
	for rows.Next() {
		rec, err := scanRecordRow(rows, true)
		if err != nil {
			return nil, err
		}
		items = append(items, rec)
	}
	var total int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM records
		 WHERE parent_id = $1 AND status != 'superseded'
	`, id).Scan(&total); err != nil {
		return nil, err
	}
	return map[string]any{"items": items, "total": total, "limit": limit, "offset": offset}, nil
}

func (r *PostgresRecordRepository) Create(ctx context.Context, id string, kind string, label string, content string, sourceURL string, parentID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO records (id, type, label, content, source_url, parent_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		ON CONFLICT (id) DO UPDATE SET
		    type = EXCLUDED.type,
		    label = EXCLUDED.label,
		    content = EXCLUDED.content,
		    source_url = EXCLUDED.source_url,
		    parent_id = EXCLUDED.parent_id
	`, id, kind, label, content, nilIfEmpty(sourceURL), nilIfEmpty(parentID))
	return err
}

func (r *PostgresRecordRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM records WHERE id = $1`, id)
	return err
}

// SimilarRecords returns records whose embedding is cosine-closest to the given record's
// (the vector "similar sources" lens), excluding the record itself. Empty if the record
// has no embedding.
func (r *PostgresRecordRepository) SimilarRecords(ctx context.Context, id string, limit int) ([]coreservice.SimilarRecord, error) {
	rows, err := r.pool.Query(ctx, `
		WITH target AS (SELECT embedding FROM records WHERE id = $1)
		SELECT r.id, r.label, COALESCE(r.source_url, ''), COALESCE(r.provider, ''),
		       1 - (r.embedding <=> t.embedding) AS cosine
		  FROM records r, target t
		 WHERE r.id <> $1
		   AND r.embedding IS NOT NULL
		   AND t.embedding IS NOT NULL
		 ORDER BY r.embedding <=> t.embedding ASC
		 LIMIT $2
	`, id, limit)
	if err != nil {
		return nil, fmt.Errorf("SimilarRecords: %w", err)
	}
	defer rows.Close()

	out := []coreservice.SimilarRecord{}
	for rows.Next() {
		var s coreservice.SimilarRecord
		if err := rows.Scan(&s.ID, &s.Label, &s.SourceURL, &s.Provider, &s.Cosine); err != nil {
			return nil, fmt.Errorf("SimilarRecords scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ResetForRetry moves a 'failed' record back to 'captured' so it re-enters the pipeline
// (the worker's reaper re-drives it). Returns false if the record wasn't failed.
func (r *PostgresRecordRepository) ResetForRetry(ctx context.Context, id string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE records SET status = 'captured', updated_at = now()
		 WHERE id = $1 AND status = 'failed'
	`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// GetRecord reads one record for the shard bridge's entity registry. Records
// carry no owner column (see the baseline schema) — the same property the
// existing List/Children reads have — so the shard's manifest grant is the gate.
func (r *PostgresRecordRepository) GetRecord(ctx context.Context, id string) (coreservice.RecordResponse, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT
		    a.id, a.type, a.label, a.content, a.source_url,
		    a.parent_id, a.enrichment, a.metadata, a.created_at,
		    COUNT(c.id) AS child_count
		  FROM records a
		  LEFT JOIN records c ON c.parent_id = a.id AND c.status != 'superseded'
		 WHERE a.id = $1
		   AND a.status != 'superseded'
		 GROUP BY a.id, a.type, a.label, a.content, a.source_url, a.parent_id,
		          a.enrichment, a.metadata, a.created_at
	`, id)
	rec, err := scanRecordRow(row, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return coreservice.RecordResponse{}, false, nil
	}
	if err != nil {
		return coreservice.RecordResponse{}, false, err
	}
	return rec, true, nil
}
