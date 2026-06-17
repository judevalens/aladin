package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EntityCandidate is a shared-tier entity matched by normalized key (oldest first).
type EntityCandidate struct {
	ID            string
	Kind          string
	CanonicalName string
	NormalizedKey string
}

// CreateEntityParams creates a shared (Tier 0) entity.
type CreateEntityParams struct {
	Kind          string
	CanonicalName string
	NormalizedKey string
	FirstRecordID string
}

// AliasParams records a surface form for an entity.
type AliasParams struct {
	EntityID   string
	Surface    string
	Normalized string
	Kind       string
	Source     string
}

// MentionParams records that a record mentions an entity (resolution provenance).
type MentionParams struct {
	RecordID       string
	EntityID       string
	Surface        string
	Kind           string
	Resolver       string
	SourceRevision int64
}

type pgEntityRepo struct{ pool *pgxpool.Pool }

func NewEntityRepository(pool *pgxpool.Pool) EntityRepository {
	return &pgEntityRepo{pool}
}

func (r *pgEntityRepo) FindSharedByKey(ctx context.Context, kind, normalizedKey string) ([]EntityCandidate, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, kind, canonical_name, normalized_key
		  FROM entities
		 WHERE scope = 'shared' AND kind = $1 AND normalized_key = $2
		 ORDER BY created_at ASC
	`, kind, normalizedKey)
	if err != nil {
		return nil, fmt.Errorf("entity FindSharedByKey: %w", err)
	}
	defer rows.Close()
	var out []EntityCandidate
	for rows.Next() {
		var c EntityCandidate
		if err := rows.Scan(&c.ID, &c.Kind, &c.CanonicalName, &c.NormalizedKey); err != nil {
			return nil, fmt.Errorf("entity FindSharedByKey scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *pgEntityRepo) CreateSharedEntity(ctx context.Context, p CreateEntityParams) (string, error) {
	prov, err := json.Marshal(map[string]string{"first_record_id": p.FirstRecordID})
	if err != nil {
		return "", fmt.Errorf("entity CreateSharedEntity marshal provenance: %w", err)
	}
	var id string
	err = r.pool.QueryRow(ctx, `
		INSERT INTO entities (scope, kind, canonical_name, normalized_key, trust_tier, provenance)
		VALUES ('shared', $1, $2, $3, 'believed', $4::jsonb)
		RETURNING id::text
	`, p.Kind, p.CanonicalName, p.NormalizedKey, prov).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("entity CreateSharedEntity: %w", err)
	}
	return id, nil
}

func (r *pgEntityRepo) AddAlias(ctx context.Context, p AliasParams) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO entity_aliases (entity_id, surface, normalized, kind, source)
		VALUES ($1::uuid, $2, $3, $4, $5)
		ON CONFLICT (entity_id, normalized) DO NOTHING
	`, p.EntityID, p.Surface, p.Normalized, p.Kind, p.Source)
	if err != nil {
		return fmt.Errorf("entity AddAlias: %w", err)
	}
	return nil
}

func (r *pgEntityRepo) AddMention(ctx context.Context, p MentionParams) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO entity_mentions (record_id, entity_id, surface, kind, resolver, source_revision)
		VALUES ($1, $2::uuid, $3, $4, $5, $6)
		ON CONFLICT (record_id, entity_id, surface) DO NOTHING
	`, p.RecordID, p.EntityID, p.Surface, p.Kind, p.Resolver, p.SourceRevision)
	if err != nil {
		return fmt.Errorf("entity AddMention: %w", err)
	}
	return nil
}
