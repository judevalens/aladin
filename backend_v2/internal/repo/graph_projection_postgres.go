package repo

import (
	"context"
	"fmt"

	"aladin/backend_v2/internal/graph"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresGraphProjectionRepository reads the entity + claim layer into the shape the Neo4j
// projector consumes. FullGraph backs `make ops-backfill-graph`; RecordGraph backs the
// incremental projection after resolve_claims.
type PostgresGraphProjectionRepository struct{ pool *pgxpool.Pool }

func NewGraphProjectionPostgres(pool *pgxpool.Pool) *PostgresGraphProjectionRepository {
	return &PostgresGraphProjectionRepository{pool: pool}
}

// FullGraph reads the whole entity/claim layer — nodes, ABOUT, claim edges, merges, and
// entity co-occurrence (RELATED_TO). Used for backfill.
func (r *PostgresGraphProjectionRepository) FullGraph(ctx context.Context) (graph.GraphData, error) {
	var d graph.GraphData

	if err := r.scanEntities(ctx, &d, `SELECT id::text, canonical_name, COALESCE(kind, '') FROM entities`); err != nil {
		return d, err
	}
	if err := r.scanClaims(ctx, &d, `
		SELECT c.id::text, c.canonical_text, c.polarity,
		       COALESCE(m.assert, 0), COALESCE(m.deny, 0)
		  FROM claims c
		  LEFT JOIN (
		      SELECT claim_id,
		             count(*) FILTER (WHERE stance = 'assert') AS assert,
		             count(*) FILTER (WHERE stance = 'deny')   AS deny
		        FROM claim_mentions GROUP BY claim_id
		  ) m ON m.claim_id = c.id
	`); err != nil {
		return d, err
	}
	if err := r.scanAbout(ctx, &d, `SELECT claim_id::text, entity_id::text FROM claim_subjects`); err != nil {
		return d, err
	}
	if err := r.scanClaimEdges(ctx, &d, `
		SELECT from_claim_id::text, to_claim_id::text, type, COALESCE(confidence, 0)
		  FROM claim_edges WHERE status <> 'rejected'
	`); err != nil {
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

// RecordGraph reads just the slice a single record contributes — the entities it mentions,
// the claims it asserts/denies, their ABOUT edges, and claim edges among those claims.
// (RELATED_TO + DIVERGES_FROM are global/derived — left to FullGraph.)
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
	if err := r.scanClaims(ctx, &d, `
		SELECT c.id::text, c.canonical_text, c.polarity,
		       COALESCE(m.assert, 0), COALESCE(m.deny, 0)
		  FROM claims c
		  JOIN claim_mentions cm ON cm.claim_id = c.id AND cm.source_kind = 'record' AND cm.source_id = $1
		  LEFT JOIN (
		      SELECT claim_id,
		             count(*) FILTER (WHERE stance = 'assert') AS assert,
		             count(*) FILTER (WHERE stance = 'deny')   AS deny
		        FROM claim_mentions GROUP BY claim_id
		  ) m ON m.claim_id = c.id
		 GROUP BY c.id, c.canonical_text, c.polarity, m.assert, m.deny
	`, recordID); err != nil {
		return d, err
	}
	if err := r.scanAbout(ctx, &d, `
		SELECT cs.claim_id::text, cs.entity_id::text
		  FROM claim_subjects cs
		 WHERE cs.claim_id IN (
		     SELECT claim_id FROM claim_mentions WHERE source_kind = 'record' AND source_id = $1
		 )
	`, recordID); err != nil {
		return d, err
	}
	if err := r.scanClaimEdges(ctx, &d, `
		SELECT from_claim_id::text, to_claim_id::text, type, COALESCE(confidence, 0)
		  FROM claim_edges
		 WHERE status <> 'rejected'
		   AND from_claim_id IN (SELECT claim_id FROM claim_mentions WHERE source_kind='record' AND source_id=$1)
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
func (r *PostgresGraphProjectionRepository) scanClaims(ctx context.Context, d *graph.GraphData, q string, args ...any) error {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("graph projection claims: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c graph.ClaimNode
		if err := rows.Scan(&c.ID, &c.Text, &c.Polarity, &c.AssertSources, &c.DenySources); err != nil {
			return err
		}
		d.Claims = append(d.Claims, c)
	}
	return rows.Err()
}
func (r *PostgresGraphProjectionRepository) scanAbout(ctx context.Context, d *graph.GraphData, q string, args ...any) error {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("graph projection about: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var a graph.AboutEdge
		if err := rows.Scan(&a.ClaimID, &a.EntityID); err != nil {
			return err
		}
		d.About = append(d.About, a)
	}
	return rows.Err()
}
func (r *PostgresGraphProjectionRepository) scanClaimEdges(ctx context.Context, d *graph.GraphData, q string, args ...any) error {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("graph projection claim_edges: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var e graph.ClaimEdge
		if err := rows.Scan(&e.From, &e.To, &e.Type, &e.Confidence); err != nil {
			return err
		}
		d.ClaimEdges = append(d.ClaimEdges, e)
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
