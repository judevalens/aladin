package repo

import (
	"context"
	"fmt"

	"aladin/backend_v2/internal/graph"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresGraphProjectionRepository reads the entity layer into the shape the Neo4j projector
// consumes. FullGraph backs `make ops-backfill-graph`; RecordGraph backs the incremental
// projection after resolve_entities.
//
// The claim layer was removed, so the graph is entity-only: nodes, MERGED_INTO, and RELATED_TO
// co-occurrence. ABOUT/SUPPORTS/CONTRADICTS/DIVERGES_FROM were all claim-anchored and went with it.
type PostgresGraphProjectionRepository struct{ pool *pgxpool.Pool }

func NewGraphProjectionPostgres(pool *pgxpool.Pool) *PostgresGraphProjectionRepository {
	return &PostgresGraphProjectionRepository{pool: pool}
}

// FullGraph reads the whole entity layer — nodes, merges, and entity co-occurrence
// (RELATED_TO). Used for backfill.
func (r *PostgresGraphProjectionRepository) FullGraph(ctx context.Context) (graph.GraphData, error) {
	var d graph.GraphData

	if err := r.scanEntities(ctx, &d, `SELECT id::text, canonical_name, COALESCE(kind, '') FROM entities`); err != nil {
		return d, err
	}
	if err := r.scanMerges(ctx, &d, `
		SELECT from_entity_id::text, into_entity_id::text
		  FROM entity_merges WHERE status = 'applied'
	`); err != nil {
		return d, err
	}
	// Entity co-occurrence: pairs mentioned by the same record, weighted by shared count.
	if err := r.scanRelated(ctx, &d, `
		SELECT a.entity_id::text, b.entity_id::text, count(*)::int
		  FROM entity_mentions a
		  JOIN entity_mentions b ON a.record_id = b.record_id AND a.entity_id < b.entity_id
		 GROUP BY a.entity_id, b.entity_id
	`); err != nil {
		return d, err
	}
	return d, nil
}

// RecordGraph reads just the slice a single record contributes — the entities it mentions.
// (RELATED_TO is global/derived — left to FullGraph.)
func (r *PostgresGraphProjectionRepository) RecordGraph(ctx context.Context, recordID string) (graph.GraphData, error) {
	var d graph.GraphData

	if err := r.scanEntities(ctx, &d, `
		SELECT e.id::text, e.canonical_name, COALESCE(e.kind, '')
		  FROM entity_mentions em JOIN entities e ON e.id = em.entity_id
		 WHERE em.record_id = $1
		 GROUP BY e.id, e.canonical_name, e.kind
	`, recordID); err != nil {
		return d, err
	}
	return d, nil
}

func (r *PostgresGraphProjectionRepository) scanEntities(ctx context.Context, d *graph.GraphData, q string, args ...any) error {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("graph projection entities: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var e graph.EntityNode
		if err := rows.Scan(&e.ID, &e.Name, &e.Kind); err != nil {
			return err
		}
		d.Entities = append(d.Entities, e)
	}
	return rows.Err()
}
func (r *PostgresGraphProjectionRepository) scanMerges(ctx context.Context, d *graph.GraphData, q string, args ...any) error {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("graph projection merges: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var m graph.MergeEdge
		if err := rows.Scan(&m.From, &m.Into); err != nil {
			return err
		}
		d.Merges = append(d.Merges, m)
	}
	return rows.Err()
}
func (r *PostgresGraphProjectionRepository) scanRelated(ctx context.Context, d *graph.GraphData, q string, args ...any) error {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("graph projection related: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var e graph.RelatedEdge
		if err := rows.Scan(&e.A, &e.B, &e.Weight); err != nil {
			return err
		}
		d.Related = append(d.Related, e)
	}
	return rows.Err()
}
