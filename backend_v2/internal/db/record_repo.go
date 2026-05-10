package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type pgRecordRepo struct{ pool *pgxpool.Pool }

func NewRecordRepository(pool *pgxpool.Pool) RecordRepository {
	return &pgRecordRepo{pool}
}

// SaveComplete writes a fully-processed tenant record to PG.
// Tenant/source matching context lives in tenant_item_matches, not records.
func (r *pgRecordRepo) SaveComplete(ctx context.Context, a *CompletedRecord) error {
	meta, _ := json.Marshal(a.Metadata)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO records
			(id, external_id, type, label, content, source_url,
			 metadata, enrichment, embedding, status, source_revision, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb, $9::vector, 'in_graph', $10, NOW())
		ON CONFLICT (id) DO UPDATE
		SET type = EXCLUDED.type,
		    label = EXCLUDED.label,
		    content = EXCLUDED.content,
		    source_url = EXCLUDED.source_url,
		    metadata = EXCLUDED.metadata,
		    enrichment = EXCLUDED.enrichment,
		    embedding = EXCLUDED.embedding,
		    status = EXCLUDED.status,
		    source_revision = EXCLUDED.source_revision
		WHERE records.source_revision < EXCLUDED.source_revision
	`,
		a.ID, a.ExternalID,
		a.Type, a.Label, a.Content, a.SourceURL,
		string(meta), string(a.Enrichment), vectorToString(a.Embedding), a.SourceRevision,
	)
	if err != nil {
		return fmt.Errorf("SaveComplete: %w", err)
	}
	return nil
}

func vectorToString(v []float32) string {
	b, _ := json.Marshal(v)
	return string(b)
}
