package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EntityCandidate is a shared-tier entity matched by normalized key (oldest first).
type EntityCandidate struct {
	ID            string
	Kind          string
	CanonicalName string
	NormalizedKey string
}

// ScoredCandidate is a fuzzy (trigram) near-match with its similarity score.
type ScoredCandidate struct {
	ID            string
	Kind          string
	CanonicalName string
	NormalizedKey string
	Similarity    float64
}

// CreateEntityParams creates a shared (Tier 0) entity.
type CreateEntityParams struct {
	Kind          string
	CanonicalName string
	NormalizedKey string
	FirstRecordID string
}

// CreateTenantEntityParams creates a tenant (Tier 1) entity, optionally bound to its
// shared canonical via SharedEntityID ("" = unbound / private).
type CreateTenantEntityParams struct {
	OwnerUserID    string
	SharedEntityID string
	Kind           string
	CanonicalName  string
	NormalizedKey  string
	FirstRecordID  string
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

// ProposeMergeParams proposes that two entities may be the same (for human review).
type ProposeMergeParams struct {
	FromEntityID string
	IntoEntityID string
	Confidence   float64
	Method       string
	Evidence     map[string]any
}

// ProposedMerge is a pending merge surfaced to the curation queue.
type ProposedMerge struct {
	ID           string
	FromEntityID string
	FromName     string
	IntoEntityID string
	IntoName     string
	Confidence   float64
	Method       string
	CreatedAt    time.Time
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

// FindSharedCandidates returns shared-tier entities of the same kind whose normalized
// key is *similar but not equal* (trigram), best first. Exact matches are excluded
// (those are handled by FindSharedByKey). Used for fuzzy near-match detection (R1).
// Note: uses pg_trgm similarity() over the (scope,kind)-narrowed set; switching to the
// `%` operator + a GIN trgm index on normalized_key is a later scale optimization.
func (r *pgEntityRepo) FindSharedCandidates(ctx context.Context, kind, normalizedKey string, minSim float64, limit int) ([]ScoredCandidate, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, kind, canonical_name, normalized_key, similarity(normalized_key, $2) AS sim
		  FROM entities
		 WHERE scope = 'shared' AND kind = $1 AND normalized_key <> $2
		   AND similarity(normalized_key, $2) >= $3
		 ORDER BY sim DESC, created_at ASC
		 LIMIT $4
	`, kind, normalizedKey, minSim, limit)
	if err != nil {
		return nil, fmt.Errorf("entity FindSharedCandidates: %w", err)
	}
	defer rows.Close()
	var out []ScoredCandidate
	for rows.Next() {
		var c ScoredCandidate
		if err := rows.Scan(&c.ID, &c.Kind, &c.CanonicalName, &c.NormalizedKey, &c.Similarity); err != nil {
			return nil, fmt.Errorf("entity FindSharedCandidates scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// FindTenantByKey looks up a tenant's own (Tier 1) entities by normalized key.
func (r *pgEntityRepo) FindTenantByKey(ctx context.Context, ownerUserID, kind, normalizedKey string) ([]EntityCandidate, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, kind, canonical_name, normalized_key
		  FROM entities
		 WHERE scope = 'tenant' AND owner_user_id = $1::uuid AND kind = $2 AND normalized_key = $3
		 ORDER BY created_at ASC
	`, ownerUserID, kind, normalizedKey)
	if err != nil {
		return nil, fmt.Errorf("entity FindTenantByKey: %w", err)
	}
	defer rows.Close()
	var out []EntityCandidate
	for rows.Next() {
		var c EntityCandidate
		if err := rows.Scan(&c.ID, &c.Kind, &c.CanonicalName, &c.NormalizedKey); err != nil {
			return nil, fmt.Errorf("entity FindTenantByKey scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateTenantEntity inserts a Tier 1 entity owned by a tenant, optionally bound to a
// shared (Tier 0) canonical.
func (r *pgEntityRepo) CreateTenantEntity(ctx context.Context, p CreateTenantEntityParams) (string, error) {
	prov, err := json.Marshal(map[string]string{"first_record_id": p.FirstRecordID})
	if err != nil {
		return "", fmt.Errorf("entity CreateTenantEntity marshal provenance: %w", err)
	}
	var shared any
	if p.SharedEntityID != "" {
		shared = p.SharedEntityID
	}
	var id string
	err = r.pool.QueryRow(ctx, `
		INSERT INTO entities (scope, owner_user_id, shared_entity_id, kind, canonical_name, normalized_key, trust_tier, provenance)
		VALUES ('tenant', $1::uuid, $2::uuid, $3, $4, $5, 'believed', $6::jsonb)
		RETURNING id::text
	`, p.OwnerUserID, shared, p.Kind, p.CanonicalName, p.NormalizedKey, prov).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("entity CreateTenantEntity: %w", err)
	}
	return id, nil
}

// ResolveCanonicalRoot returns an entity's effective canonical id, composing the
// reversible merge overlay across tiers: it follows canonical_root_id to the local
// (tenant) root first — so a tenant's own merges win (local override) — and only then,
// if that root is bound to a shared entity, crosses into the shared tier and follows
// its root. Shared entities never point up at a tenant (enforced), so this terminates.
func (r *pgEntityRepo) ResolveCanonicalRoot(ctx context.Context, entityID string) (string, error) {
	cur := entityID
	for i := 0; i < 32; i++ {
		var root, shared *string
		err := r.pool.QueryRow(ctx, `
			SELECT canonical_root_id::text, shared_entity_id::text
			  FROM entities WHERE id = $1::uuid
		`, cur).Scan(&root, &shared)
		if err != nil {
			return "", fmt.Errorf("entity ResolveCanonicalRoot: %w", err)
		}
		if root != nil && *root != cur {
			cur = *root // follow a merge within the current tier
			continue
		}
		if shared != nil { // at a local root that is bound to shared → cross tiers
			cur = *shared
			continue
		}
		return cur, nil
	}
	return cur, nil // depth cap (defensive; the overlay is acyclic)
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

// ProposeMerge records a proposed merge for human review, unless the (unordered) pair
// already has a row in any status — which crucially includes a 'rejected' row, the
// negative evidence that stops the pipeline from re-proposing a pair a human has marked
// distinct. Returns whether a new proposal was inserted.
func (r *pgEntityRepo) ProposeMerge(ctx context.Context, p ProposeMergeParams) (bool, error) {
	evidence, err := json.Marshal(p.Evidence)
	if err != nil {
		return false, fmt.Errorf("entity ProposeMerge marshal evidence: %w", err)
	}
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO entity_merges (from_entity_id, into_entity_id, status, confidence, method, evidence, decided_by)
		SELECT $1::uuid, $2::uuid, 'proposed', $3, $4, $5::jsonb, 'auto'
		 WHERE NOT EXISTS (
			SELECT 1 FROM entity_merges
			 WHERE (from_entity_id = $1::uuid AND into_entity_id = $2::uuid)
			    OR (from_entity_id = $2::uuid AND into_entity_id = $1::uuid)
		 )
	`, p.FromEntityID, p.IntoEntityID, p.Confidence, p.Method, evidence)
	if err != nil {
		return false, fmt.Errorf("entity ProposeMerge: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// RecordDistinct records that two entities are NOT the same (negative evidence),
// e.g. an adjudicator returned "different". It writes a rejected merge row so
// ProposeMerge never proposes the pair. No-op if the pair already has a row.
func (r *pgEntityRepo) RecordDistinct(ctx context.Context, fromEntityID, intoEntityID, method, reason string) error {
	evidence, err := json.Marshal(map[string]string{"reason": reason})
	if err != nil {
		return fmt.Errorf("entity RecordDistinct marshal evidence: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO entity_merges (from_entity_id, into_entity_id, status, method, evidence, decided_by, decided_at)
		SELECT $1::uuid, $2::uuid, 'rejected', $3, $4::jsonb, 'llm', now()
		 WHERE NOT EXISTS (
			SELECT 1 FROM entity_merges
			 WHERE (from_entity_id = $1::uuid AND into_entity_id = $2::uuid)
			    OR (from_entity_id = $2::uuid AND into_entity_id = $1::uuid)
		 )
	`, fromEntityID, intoEntityID, method, evidence)
	if err != nil {
		return fmt.Errorf("entity RecordDistinct: %w", err)
	}
	return nil
}

func (r *pgEntityRepo) ListProposedMerges(ctx context.Context, limit int) ([]ProposedMerge, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.id::text, m.from_entity_id::text, ef.canonical_name,
		       m.into_entity_id::text, ei.canonical_name,
		       COALESCE(m.confidence, 0), COALESCE(m.method, ''), m.created_at
		  FROM entity_merges m
		  JOIN entities ef ON ef.id = m.from_entity_id
		  JOIN entities ei ON ei.id = m.into_entity_id
		 WHERE m.status = 'proposed'
		 ORDER BY m.created_at ASC
		 LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("entity ListProposedMerges: %w", err)
	}
	defer rows.Close()
	var out []ProposedMerge
	for rows.Next() {
		var m ProposedMerge
		if err := rows.Scan(&m.ID, &m.FromEntityID, &m.FromName, &m.IntoEntityID, &m.IntoName, &m.Confidence, &m.Method, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("entity ListProposedMerges scan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// RejectMerge marks a proposed merge rejected. The row stays as negative evidence so
// ProposeMerge never re-proposes the pair.
func (r *pgEntityRepo) RejectMerge(ctx context.Context, mergeID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE entity_merges
		   SET status = 'rejected', decided_by = 'user', decided_at = now()
		 WHERE id = $1::uuid AND status = 'proposed'
	`, mergeID)
	if err != nil {
		return fmt.Errorf("entity RejectMerge: %w", err)
	}
	return nil
}

// AcceptMerge applies a proposed merge: it records the decision and points the
// from-entity's canonical_root_id at the into-entity's root (the reversible overlay).
// The read path that resolves mentions *through* canonical_root_id (union-find with
// path compression) is a later phase; this just writes the overlay link.
func (r *pgEntityRepo) AcceptMerge(ctx context.Context, mergeID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("entity AcceptMerge begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var fromID, intoID string
	err = tx.QueryRow(ctx, `
		UPDATE entity_merges
		   SET status = 'applied', decided_by = 'user', decided_at = now()
		 WHERE id = $1::uuid AND status = 'proposed'
		 RETURNING from_entity_id::text, into_entity_id::text
	`, mergeID).Scan(&fromID, &intoID)
	if err != nil {
		return fmt.Errorf("entity AcceptMerge update merge: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE entities e
		   SET canonical_root_id = COALESCE(
				 (SELECT canonical_root_id FROM entities WHERE id = $2::uuid),
				 $2::uuid),
		       updated_at = now()
		 WHERE e.id = $1::uuid
	`, fromID, intoID); err != nil {
		return fmt.Errorf("entity AcceptMerge set root: %w", err)
	}
	return tx.Commit(ctx)
}
