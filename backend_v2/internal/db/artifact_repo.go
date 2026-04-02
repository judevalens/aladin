package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type pgArtifactRepo struct{ pool *pgxpool.Pool }

func NewArtifactRepository(pool *pgxpool.Pool) ArtifactRepository {
	return &pgArtifactRepo{pool}
}

// SaveComplete writes a fully-processed artifact to PG in a single INSERT.
// Uses external_id + source_id for dedup — re-fetching the same post is a no-op.
func (r *pgArtifactRepo) SaveComplete(ctx context.Context, a *CompletedArtifact) error {
	meta, _ := json.Marshal(a.Metadata)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO artifacts
			(id, source_id, external_id, type, label, content, source_url,
			 metadata, enrichment, embedding, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb, $10::vector, 'in_graph', NOW())
		ON CONFLICT (source_id, external_id) WHERE external_id IS NOT NULL DO NOTHING
	`,
		a.ID, a.SourceID, a.ExternalID,
		a.Type, a.Label, a.Content, a.SourceURL,
		string(meta), string(a.Enrichment), vectorToString(a.Embedding),
	)
	if err != nil {
		return fmt.Errorf("SaveComplete: %w", err)
	}
	return nil
}

// ExistsExternal returns a set of which externalIDs already exist for the given source.
func (r *pgArtifactRepo) ExistsExternal(ctx context.Context, sourceID string, externalIDs []string) (map[string]bool, error) {
	if len(externalIDs) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT external_id FROM artifacts
		WHERE source_id = $1 AND external_id = ANY($2)
	`, sourceID, externalIDs)
	if err != nil {
		return nil, fmt.Errorf("ExistsExternal: %w", err)
	}
	defer rows.Close()

	known := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("ExistsExternal scan: %w", err)
		}
		known[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ExistsExternal rows: %w", err)
	}
	return known, nil
}

func vectorToString(v []float32) string {
	b, _ := json.Marshal(v)
	return string(b)
}
