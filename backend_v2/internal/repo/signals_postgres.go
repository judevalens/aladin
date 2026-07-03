package repo

import (
	"context"
	"encoding/json"
	"time"

	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresSignalRepository serves the Signals surface: shared claims as ranked signal cards,
// each assembled with its subject entities, assert/deny evidence tally, and argument-edge counts.
type PostgresSignalRepository struct{ pool *pgxpool.Pool }

func NewSignalPostgres(pool *pgxpool.Pool) *PostgresSignalRepository {
	return &PostgresSignalRepository{pool: pool}
}

// bookMinCosine mirrors claimResolveMinCosine (claims.Service): how similar a discovered
// claim must be to a thesis to count as the same proposition for the mark-to-market tally.
const bookMinCosine = 0.8

func (r *PostgresSignalRepository) List(ctx context.Context, params coreservice.SignalListParams) (map[string]any, error) {
	if params.Lens == "book" {
		return r.listBook(ctx, params)
	}
	// signal_score (digg-like salience): corroboration (total mentions) + contention (a bonus
	// when a claim is both asserted AND denied) + argument connectivity (edges touching it).
	order := "c.created_at DESC"
	if params.Sort == "top" {
		order = "signal_score DESC, c.created_at DESC"
	}
	query := `
		WITH mention_counts AS (
			SELECT claim_id,
			       count(*) FILTER (WHERE stance = 'assert') AS assert_count,
			       count(*) FILTER (WHERE stance = 'deny')   AS deny_count
			  FROM claim_mentions GROUP BY claim_id
		),
		edge_counts AS (
			SELECT cid AS claim_id,
			       count(*) FILTER (WHERE type = 'supports')    AS supports,
			       count(*) FILTER (WHERE type = 'contradicts') AS contradicts,
			       count(*) FILTER (WHERE type = 'qualifies')   AS qualifies
			  FROM (
				SELECT from_claim_id AS cid, type FROM claim_edges WHERE status <> 'rejected'
				UNION ALL
				SELECT to_claim_id   AS cid, type FROM claim_edges WHERE status <> 'rejected'
			  ) e GROUP BY cid
		)
		SELECT c.id::text, c.canonical_text, c.polarity, c.trust_tier, c.created_at,
		       COALESCE(mc.assert_count, 0), COALESCE(mc.deny_count, 0),
		       COALESCE(ec.supports, 0), COALESCE(ec.contradicts, 0), COALESCE(ec.qualifies, 0),
		       COALESCE((
				SELECT json_agg(json_build_object('id', en.id::text, 'name', en.canonical_name) ORDER BY en.canonical_name)
				  FROM claim_subjects cs JOIN entities en ON en.id = cs.entity_id
				 WHERE cs.claim_id = c.id
		       ), '[]'::json) AS subjects,
		       (COALESCE(mc.assert_count,0) + COALESCE(mc.deny_count,0)
		        + 2 * LEAST(COALESCE(mc.assert_count,0), COALESCE(mc.deny_count,0))
		        + COALESCE(ec.supports,0) + COALESCE(ec.contradicts,0) + COALESCE(ec.qualifies,0))::double precision AS signal_score
		  FROM claims c
		  LEFT JOIN mention_counts mc ON mc.claim_id = c.id
		  LEFT JOIN edge_counts ec ON ec.claim_id = c.id
		 WHERE c.scope = 'shared'
		 ORDER BY ` + order + `
		 LIMIT $1 OFFSET $2
	`
	rows, err := r.pool.Query(ctx, query, params.Limit, params.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	signals := []coreservice.ClaimSignal{}
	for rows.Next() {
		var (
			s         coreservice.ClaimSignal
			createdAt time.Time
			subjects  []byte
		)
		if err := rows.Scan(
			&s.ID, &s.Text, &s.Polarity, &s.TrustTier, &createdAt,
			&s.AssertCount, &s.DenyCount,
			&s.Supports, &s.Contradicts, &s.Qualifies,
			&subjects, &s.SignalScore,
		); err != nil {
			return nil, err
		}
		if len(subjects) > 0 {
			if err := json.Unmarshal(subjects, &s.Subjects); err != nil {
				return nil, err
			}
		}
		s.CreatedAt = createdAt.Format(time.RFC3339)
		signals = append(signals, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{"signals": signals}, nil
}

// listBook serves the "your book" lens: the caller's authored theses (scope='tenant'),
// each marked to market by how many DISTINCT discovered sources support vs contradict it.
// The per-thesis tally mirrors db.pgClaimRepo.ContradictionSurface — a discovered (shared)
// claim counts as the same proposition when it shares a subject entity and its embedding is
// within bookMinCosine cosine of the thesis; C1 resolution folds negations into deny-mentions
// on one canonical claim, so asserting sources = support and denying sources = contradict.
// support/contradict ride on AssertCount/DenyCount so the card's for/against bar works unchanged.
func (r *PostgresSignalRepository) listBook(ctx context.Context, params coreservice.SignalListParams) (map[string]any, error) {
	if params.OwnerUserID == "" {
		return map[string]any{"signals": []coreservice.ClaimSignal{}}, nil
	}
	order := "c.created_at DESC"
	if params.Sort == "top" {
		order = "signal_score DESC, c.created_at DESC"
	}
	query := `
		SELECT c.id::text, c.canonical_text, c.polarity, c.trust_tier, c.created_at,
		       COALESCE(t.support, 0)    AS support,
		       COALESCE(t.contradict, 0) AS contradict,
		       COALESCE((
				SELECT json_agg(json_build_object('id', en.id::text, 'name', en.canonical_name) ORDER BY en.canonical_name)
				  FROM claim_subjects cs JOIN entities en ON en.id = cs.entity_id
				 WHERE cs.claim_id = c.id
		       ), '[]'::json) AS subjects,
		       (COALESCE(t.support,0) + COALESCE(t.contradict,0)
		        + 2 * LEAST(COALESCE(t.support,0), COALESCE(t.contradict,0)))::double precision AS signal_score
		  FROM claims c
		  LEFT JOIN LATERAL (
			SELECT count(DISTINCT m.source_id) FILTER (WHERE m.stance = 'assert') AS support,
			       count(DISTINCT m.source_id) FILTER (WHERE m.stance = 'deny')   AS contradict
			  FROM claim_mentions m
			 WHERE m.claim_id IN (
				SELECT sc.id FROM claims sc
				 WHERE sc.scope = 'shared' AND sc.embedding IS NOT NULL AND c.embedding IS NOT NULL
				   AND sc.id IN (SELECT claim_id FROM claim_subjects
				                  WHERE entity_id = ANY(SELECT entity_id FROM claim_subjects WHERE claim_id = c.id))
				   AND 1 - (sc.embedding <=> c.embedding) >= $4
			 )
		  ) t ON true
		 WHERE c.scope = 'tenant' AND c.owner_user_id = $1::uuid
		 ORDER BY ` + order + `
		 LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, query, params.OwnerUserID, params.Limit, params.Offset, bookMinCosine)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	signals := []coreservice.ClaimSignal{}
	for rows.Next() {
		var (
			s         coreservice.ClaimSignal
			createdAt time.Time
			subjects  []byte
		)
		if err := rows.Scan(
			&s.ID, &s.Text, &s.Polarity, &s.TrustTier, &createdAt,
			&s.AssertCount, &s.DenyCount, &subjects, &s.SignalScore,
		); err != nil {
			return nil, err
		}
		if len(subjects) > 0 {
			if err := json.Unmarshal(subjects, &s.Subjects); err != nil {
				return nil, err
			}
		}
		s.CreatedAt = createdAt.Format(time.RFC3339)
		signals = append(signals, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{"signals": signals}, nil
}
