package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateClaimParams creates a canonical claim. Discovered claims are scope='shared'
// trust='believed'; authored theses are scope='tenant' (owner) trust='verified'.
type CreateClaimParams struct {
	Scope         string
	OwnerUserID   string // "" → NULL (shared)
	CanonicalText string
	Polarity      string // assert | deny | neutral
	TrustTier     string // "" → believed
	FirstSourceID string
}

// ClaimStanceCounts is the contradiction-surface tally for a thesis: how many distinct
// sources support vs contradict it.
type ClaimStanceCounts struct {
	Support    int
	Contradict int
	Hedge      int
}

// ClaimMentionParams records that a source asserts/denies/hedges a claim (the evidence layer).
type ClaimMentionParams struct {
	ClaimID    string
	SourceKind string // record | artifact
	SourceID   string
	Stance     string // assert | deny | hedge
	Resolver   string
}

// ScoredClaim is a candidate claim from resolution (embedding cosine over claims sharing
// a subject entity).
type ScoredClaim struct {
	ID            string
	CanonicalText string
	Polarity      string
	Similarity    float64
}

// ClaimEdgeParams proposes a typed argument edge between two distinct claims.
type ClaimEdgeParams struct {
	FromClaimID string
	ToClaimID   string
	Type        string // supports | contradicts | qualifies
	Confidence  float64
	Method      string
}

type pgClaimRepo struct{ pool *pgxpool.Pool }

func NewClaimRepository(pool *pgxpool.Pool) ClaimRepository {
	return &pgClaimRepo{pool}
}

// FindClaimByText looks up a claim by exact canonical text within a scope/owner. C0's
// thin dedup (paraphrase-aware resolution is C1) — enough to make a same-text re-extract
// idempotent.
func (r *pgClaimRepo) FindClaimByText(ctx context.Context, scope, ownerUserID, canonicalText string) (string, bool, error) {
	var owner any
	if ownerUserID != "" {
		owner = ownerUserID
	}
	var id string
	err := r.pool.QueryRow(ctx, `
		SELECT id::text FROM claims
		 WHERE scope = $1 AND canonical_text = $2 AND owner_user_id IS NOT DISTINCT FROM $3::uuid
		 LIMIT 1
	`, scope, canonicalText, owner).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("claim FindClaimByText: %w", err)
	}
	return id, true, nil
}

func (r *pgClaimRepo) CreateClaim(ctx context.Context, p CreateClaimParams) (string, error) {
	prov, err := json.Marshal(map[string]string{"first_source_id": p.FirstSourceID})
	if err != nil {
		return "", fmt.Errorf("claim CreateClaim marshal provenance: %w", err)
	}
	var owner any
	if p.OwnerUserID != "" {
		owner = p.OwnerUserID
	}
	scope := p.Scope
	if scope == "" {
		scope = "shared"
	}
	trust := p.TrustTier
	if trust == "" {
		trust = "believed"
	}
	var id string
	err = r.pool.QueryRow(ctx, `
		INSERT INTO claims (scope, owner_user_id, canonical_text, polarity, trust_tier, provenance)
		VALUES ($1, $2::uuid, $3, $4, $5, $6::jsonb)
		RETURNING id::text
	`, scope, owner, p.CanonicalText, p.Polarity, trust, prov).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("claim CreateClaim: %w", err)
	}
	return id, nil
}

func (r *pgClaimRepo) AddClaimSubject(ctx context.Context, claimID, entityID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO claim_subjects (claim_id, entity_id)
		VALUES ($1::uuid, $2::uuid)
		ON CONFLICT (claim_id, entity_id) DO NOTHING
	`, claimID, entityID)
	if err != nil {
		return fmt.Errorf("claim AddClaimSubject: %w", err)
	}
	return nil
}

// SetClaimEmbedding stores a claim's proposition embedding (C1, for resolution).
func (r *pgClaimRepo) SetClaimEmbedding(ctx context.Context, claimID string, vec []float32) error {
	if len(vec) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE claims SET embedding = $2::vector, updated_at = now() WHERE id = $1::uuid
	`, claimID, vectorToString(vec))
	if err != nil {
		return fmt.Errorf("claim SetClaimEmbedding: %w", err)
	}
	return nil
}

// FindClaimCandidates returns claims in the given scope/owner that share ≥1 subject
// entity with the mention and are embedding-similar — the candidates for polarity-aware
// resolution (C1). Subject overlap is the cheap blocker the entity layer gives for free;
// pgvector cosine ranks. Pass scope='shared', owner='' to match a tenant thesis against
// discovered claims (the contradiction surface, C3).
func (r *pgClaimRepo) FindClaimCandidates(ctx context.Context, scope, ownerUserID string, subjectEntityIDs []string, vec []float32, minCosine float64, limit int) ([]ScoredClaim, error) {
	if len(subjectEntityIDs) == 0 || len(vec) == 0 {
		return nil, nil
	}
	var owner any
	if ownerUserID != "" {
		owner = ownerUserID
	}
	rows, err := r.pool.Query(ctx, `
		SELECT c.id::text, c.canonical_text, c.polarity, 1 - (c.embedding <=> $4::vector) AS cosine
		  FROM claims c
		 WHERE c.scope = $1 AND c.owner_user_id IS NOT DISTINCT FROM $2::uuid
		   AND c.embedding IS NOT NULL
		   AND c.id IN (SELECT claim_id FROM claim_subjects WHERE entity_id = ANY($3::uuid[]))
		   AND 1 - (c.embedding <=> $4::vector) >= $5
		 ORDER BY c.embedding <=> $4::vector ASC
		 LIMIT $6
	`, scope, owner, subjectEntityIDs, vectorToString(vec), minCosine, limit)
	if err != nil {
		return nil, fmt.Errorf("claim FindClaimCandidates: %w", err)
	}
	defer rows.Close()
	var out []ScoredClaim
	for rows.Next() {
		var c ScoredClaim
		if err := rows.Scan(&c.ID, &c.CanonicalText, &c.Polarity, &c.Similarity); err != nil {
			return nil, fmt.Errorf("claim FindClaimCandidates scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ContradictionSurface computes, for a thesis claim, how many distinct discovered
// (shared) sources SUPPORT vs CONTRADICT it (C4 — the hero query). It matches the thesis
// to shared claims of the same proposition (shared subject entity + embedding cosine ≥
// minCosine) and tallies their mentions by stance — deduped by source. Because C1
// resolution folds negations into deny-mentions on one canonical proposition, support =
// asserting sources and contradict = denying sources. Returns zero counts if the thesis
// has no embedding or subjects.
func (r *pgClaimRepo) ContradictionSurface(ctx context.Context, thesisClaimID string, minCosine float64) (ClaimStanceCounts, error) {
	var out ClaimStanceCounts

	var embRaw *string
	if err := r.pool.QueryRow(ctx, `SELECT embedding::text FROM claims WHERE id = $1::uuid`, thesisClaimID).Scan(&embRaw); err != nil {
		return out, fmt.Errorf("claim ContradictionSurface load thesis: %w", err)
	}
	if embRaw == nil || *embRaw == "" {
		return out, nil
	}

	subjRows, err := r.pool.Query(ctx, `SELECT entity_id::text FROM claim_subjects WHERE claim_id = $1::uuid`, thesisClaimID)
	if err != nil {
		return out, fmt.Errorf("claim ContradictionSurface subjects: %w", err)
	}
	var subjects []string
	for subjRows.Next() {
		var s string
		if err := subjRows.Scan(&s); err != nil {
			subjRows.Close()
			return out, fmt.Errorf("claim ContradictionSurface subject scan: %w", err)
		}
		subjects = append(subjects, s)
	}
	subjRows.Close()
	if err := subjRows.Err(); err != nil {
		return out, err
	}
	if len(subjects) == 0 {
		return out, nil
	}

	rows, err := r.pool.Query(ctx, `
		WITH matched AS (
			SELECT c.id
			  FROM claims c
			 WHERE c.scope = 'shared' AND c.embedding IS NOT NULL
			   AND c.id IN (SELECT claim_id FROM claim_subjects WHERE entity_id = ANY($1::uuid[]))
			   AND 1 - (c.embedding <=> $2::vector) >= $3
		)
		SELECT m.stance, count(DISTINCT m.source_id)
		  FROM claim_mentions m JOIN matched ON matched.id = m.claim_id
		 GROUP BY m.stance
	`, subjects, *embRaw, minCosine)
	if err != nil {
		return out, fmt.Errorf("claim ContradictionSurface aggregate: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var stance string
		var n int
		if err := rows.Scan(&stance, &n); err != nil {
			return out, fmt.Errorf("claim ContradictionSurface scan: %w", err)
		}
		switch stance {
		case "assert":
			out.Support = n
		case "deny":
			out.Contradict = n
		case "hedge":
			out.Hedge = n
		}
	}
	return out, rows.Err()
}

// AddClaimEdge proposes a typed argument edge between two distinct claims, unless the
// (unordered) pair already has an edge of that type in any status (negative-evidence
// safe, like entity ProposeMerge). Returns whether a new edge was inserted.
func (r *pgClaimRepo) AddClaimEdge(ctx context.Context, p ClaimEdgeParams) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO claim_edges (from_claim_id, to_claim_id, type, status, confidence, method, decided_by)
		SELECT $1::uuid, $2::uuid, $3, 'proposed', $4, $5, 'auto'
		 WHERE NOT EXISTS (
			SELECT 1 FROM claim_edges
			 WHERE type = $3
			   AND ((from_claim_id = $1::uuid AND to_claim_id = $2::uuid)
			     OR (from_claim_id = $2::uuid AND to_claim_id = $1::uuid))
		 )
	`, p.FromClaimID, p.ToClaimID, p.Type, p.Confidence, p.Method)
	if err != nil {
		return false, fmt.Errorf("claim AddClaimEdge: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// SourceClaims returns the distinct canonical claims a source asserts/denies (via
// claim_mentions), newest first. For the Y3 Connect path: the claims a page just produced.
func (r *pgClaimRepo) SourceClaims(ctx context.Context, sourceKind, sourceID string) ([]SourceClaim, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT c.id::text, c.canonical_text, c.polarity, max(m.created_at) AS seen
		  FROM claim_mentions m JOIN claims c ON c.id = m.claim_id
		 WHERE m.source_kind = $1 AND m.source_id = $2
		 GROUP BY c.id, c.canonical_text, c.polarity
		 ORDER BY seen DESC
	`, sourceKind, sourceID)
	if err != nil {
		return nil, fmt.Errorf("claim SourceClaims: %w", err)
	}
	defer rows.Close()
	var out []SourceClaim
	for rows.Next() {
		var c SourceClaim
		var seen any
		if err := rows.Scan(&c.ID, &c.Text, &c.Polarity, &seen); err != nil {
			return nil, fmt.Errorf("claim SourceClaims scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *pgClaimRepo) AddClaimMention(ctx context.Context, p ClaimMentionParams) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO claim_mentions (claim_id, source_kind, source_id, stance, resolver)
		VALUES ($1::uuid, $2, $3, $4, $5)
		ON CONFLICT (source_kind, source_id, claim_id) DO NOTHING
	`, p.ClaimID, p.SourceKind, p.SourceID, p.Stance, p.Resolver)
	if err != nil {
		return fmt.Errorf("claim AddClaimMention: %w", err)
	}
	return nil
}
