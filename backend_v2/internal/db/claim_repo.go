package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateClaimParams creates a canonical claim. C0 uses scope='shared' (discovered side).
type CreateClaimParams struct {
	Scope         string
	OwnerUserID   string // "" → NULL (shared)
	CanonicalText string
	Polarity      string // assert | deny | neutral
	FirstSourceID string
}

// ClaimMentionParams records that a source asserts/denies/hedges a claim (the evidence layer).
type ClaimMentionParams struct {
	ClaimID    string
	SourceKind string // record | artifact
	SourceID   string
	Stance     string // assert | deny | hedge
	Resolver   string
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
	var id string
	err = r.pool.QueryRow(ctx, `
		INSERT INTO claims (scope, owner_user_id, canonical_text, polarity, trust_tier, provenance)
		VALUES ($1, $2::uuid, $3, $4, 'believed', $5::jsonb)
		RETURNING id::text
	`, scope, owner, p.CanonicalText, p.Polarity, prov).Scan(&id)
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
