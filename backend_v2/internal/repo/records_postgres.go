package repo

import (
	"context"

	coreservice "aladin/backend_v2/internal/service"

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
