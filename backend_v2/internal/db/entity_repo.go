package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EntityRef is a minimal entity reference (id + display name), e.g. the entities a
// record mentions — used to ground claims (claim_subjects).
type EntityRef struct {
	ID   string
	Name string
}

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

// SetEntityEmbedding stores an entity's context-embedding (R2).
func (r *pgEntityRepo) SetEntityEmbedding(ctx context.Context, entityID string, vec []float32) error {
	if len(vec) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE entities SET embedding = $2::vector, updated_at = now() WHERE id = $1::uuid
	`, entityID, vectorToString(vec))
	if err != nil {
		return fmt.Errorf("entity SetEntityEmbedding: %w", err)
	}
	return nil
}

// GetEntityEmbedding returns an entity's stored embedding, or has=false if unset.
func (r *pgEntityRepo) GetEntityEmbedding(ctx context.Context, entityID string) ([]float32, bool, error) {
	var raw *string
	if err := r.pool.QueryRow(ctx, `SELECT embedding::text FROM entities WHERE id = $1::uuid`, entityID).Scan(&raw); err != nil {
		return nil, false, fmt.Errorf("entity GetEntityEmbedding: %w", err)
	}
	if raw == nil || *raw == "" {
		return nil, false, nil
	}
	var vec []float32
	if err := json.Unmarshal([]byte(*raw), &vec); err != nil {
		return nil, false, fmt.Errorf("entity GetEntityEmbedding parse: %w", err)
	}
	return vec, true, nil
}

// FindSharedCandidatesByVector returns shared-tier entities of the same kind with an
// embedding cosine-similar to vec (excluding the exact normalized key), best first.
// Uses pgvector's <=> cosine-distance operator. A semantic near-match signal that
// trigram blocking misses (R2).
func (r *pgEntityRepo) FindSharedCandidatesByVector(ctx context.Context, kind, excludeKey string, vec []float32, minCosine float64, limit int) ([]ScoredCandidate, error) {
	if len(vec) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, kind, canonical_name, normalized_key, 1 - (embedding <=> $2::vector) AS cosine
		  FROM entities
		 WHERE scope = 'shared' AND kind = $1 AND embedding IS NOT NULL AND normalized_key <> $3
		   AND 1 - (embedding <=> $2::vector) >= $4
		 ORDER BY embedding <=> $2::vector ASC
		 LIMIT $5
	`, kind, vectorToString(vec), excludeKey, minCosine, limit)
	if err != nil {
		return nil, fmt.Errorf("entity FindSharedCandidatesByVector: %w", err)
	}
	defer rows.Close()
	var out []ScoredCandidate
	for rows.Next() {
		var c ScoredCandidate
		if err := rows.Scan(&c.ID, &c.Kind, &c.CanonicalName, &c.NormalizedKey, &c.Similarity); err != nil {
			return nil, fmt.Errorf("entity FindSharedCandidatesByVector scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// EntitiesForRecord returns the distinct entities a record mentions (via entity_mentions),
// the grounding set for claim extraction.
func (r *pgEntityRepo) EntitiesForRecord(ctx context.Context, recordID string) ([]EntityRef, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT e.id::text, e.canonical_name
		  FROM entity_mentions m JOIN entities e ON e.id = m.entity_id
		 WHERE m.record_id = $1
	`, recordID)
	if err != nil {
		return nil, fmt.Errorf("entity EntitiesForRecord: %w", err)
	}
	defer rows.Close()
	var out []EntityRef
	for rows.Next() {
		var ref EntityRef
		if err := rows.Scan(&ref.ID, &ref.Name); err != nil {
			return nil, fmt.Errorf("entity EntitiesForRecord scan: %w", err)
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

// EntitiesForArtifact returns the distinct entities a page is linked to (its tags +
// projected @mentions, via artifact_entities) — the grounding set for authored claim
// extraction (P3, the manual counterpart to EntitiesForRecord).
func (r *pgEntityRepo) EntitiesForArtifact(ctx context.Context, artifactID string) ([]EntityRef, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT e.id::text, e.canonical_name
		  FROM artifact_entities ae JOIN entities e ON e.id = ae.entity_id
		 WHERE ae.artifact_id = $1
	`, artifactID)
	if err != nil {
		return nil, fmt.Errorf("entity EntitiesForArtifact: %w", err)
	}
	defer rows.Close()
	var out []EntityRef
	for rows.Next() {
		var ref EntityRef
		if err := rows.Scan(&ref.ID, &ref.Name); err != nil {
			return nil, fmt.Errorf("entity EntitiesForArtifact scan: %w", err)
		}
		out = append(out, ref)
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
//
// Returns pgx.ErrNoRows when the row isn't pending (already decided, or unknown id) —
// deliberately symmetric with AcceptMerge, which RETURNINGs. Callers exposing these to a
// user need one predictable outcome for a double-click; previously this used Exec and
// silently "succeeded" on an already-decided row while Accept errored.
func (r *pgEntityRepo) RejectMerge(ctx context.Context, mergeID string) error {
	var id string
	err := r.pool.QueryRow(ctx, `
		UPDATE entity_merges
		   SET status = 'rejected', decided_by = 'user', decided_at = now()
		 WHERE id = $1::uuid AND status = 'proposed'
		 RETURNING id::text
	`, mergeID).Scan(&id)
	if err != nil {
		return fmt.Errorf("entity RejectMerge: %w", err)
	}
	return nil
}

// RevertMerge undoes an applied merge: the from-entity's canonical_root_id is reset to
// self (NULL), so it pops back out of the cluster with all its mentions intact (those
// were never rewritten — the reversibility guarantee). Marks the row reverted.
func (r *pgEntityRepo) RevertMerge(ctx context.Context, mergeID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("entity RevertMerge begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var fromID string
	err = tx.QueryRow(ctx, `
		UPDATE entity_merges
		   SET status = 'reverted', decided_at = now()
		 WHERE id = $1::uuid AND status = 'applied'
		 RETURNING from_entity_id::text
	`, mergeID).Scan(&fromID)
	if err != nil {
		return fmt.Errorf("entity RevertMerge update merge: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE entities SET canonical_root_id = NULL, updated_at = now() WHERE id = $1::uuid
	`, fromID); err != nil {
		return fmt.Errorf("entity RevertMerge reset root: %w", err)
	}
	return tx.Commit(ctx)
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

// ── P1.3: the judge sweep (entity-hardening plan) ────────────────────────────────────

// PlaceholderEntity is an unresolved entity minted by the editor's @ create path,
// awaiting background resolution (merge into an existing entity, or promotion).
type PlaceholderEntity struct {
	ID            string
	Kind          string
	CanonicalName string
	NormalizedKey string
	CreatedAt     time.Time
}

// JudgeableMerge is a proposed merge enriched for the judge sweep: kinds decide the
// deterministic tier (cross-typed-kind pairs never auto-merge — sense splits), and the
// judged filter (no 'judge' evidence key) guarantees one LLM judgment per pair, ever.
type JudgeableMerge struct {
	ID         string
	FromID     string
	FromName   string
	FromKind   string
	IntoID     string
	IntoName   string
	IntoKind   string
	Confidence float64
	Method     string
}

// ListPlaceholders returns unresolved placeholder entities (oldest first) that have not
// already been merged away.
func (r *pgEntityRepo) ListPlaceholders(ctx context.Context, limit int) ([]PlaceholderEntity, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, kind, canonical_name, normalized_key, created_at
		  FROM entities
		 WHERE trust_tier = 'placeholder' AND canonical_root_id IS NULL AND scope = 'shared'
		 ORDER BY created_at ASC
		 LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("entity ListPlaceholders: %w", err)
	}
	defer rows.Close()
	var out []PlaceholderEntity
	for rows.Next() {
		var p PlaceholderEntity
		if err := rows.Scan(&p.ID, &p.Kind, &p.CanonicalName, &p.NormalizedKey, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("entity ListPlaceholders scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// FindMergeCandidatesAnyKind finds merge candidates for a placeholder across ALL kinds,
// alias-aware, INCLUDING exact-key matches (for a placeholder, the same key under a
// typed kind is the prime candidate — the opposite of FindSharedCandidates, which
// excludes the exact key because its caller just created it). Excludes the entity
// itself, merged-away rows, and tenant-scope rows.
func (r *pgEntityRepo) FindMergeCandidatesAnyKind(ctx context.Context, entityID, normalizedKey string, minSim float64, limit int) ([]ScoredCandidate, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT e.id::text, e.kind, e.canonical_name, e.normalized_key,
		       MAX(GREATEST(similarity(e.normalized_key, $2),
		                    COALESCE(similarity(a.normalized, $2), 0))) AS sim
		  FROM entities e
		  LEFT JOIN entity_aliases a ON a.entity_id = e.id
		 WHERE e.scope = 'shared' AND e.id <> $1::uuid AND e.canonical_root_id IS NULL
		 GROUP BY e.id, e.kind, e.canonical_name, e.normalized_key
		HAVING MAX(GREATEST(similarity(e.normalized_key, $2),
		                    COALESCE(similarity(a.normalized, $2), 0))) >= $3
		 ORDER BY sim DESC, e.created_at ASC
		 LIMIT $4
	`, entityID, normalizedKey, minSim, limit)
	if err != nil {
		return nil, fmt.Errorf("entity FindMergeCandidatesAnyKind: %w", err)
	}
	defer rows.Close()
	var out []ScoredCandidate
	for rows.Next() {
		var c ScoredCandidate
		if err := rows.Scan(&c.ID, &c.Kind, &c.CanonicalName, &c.NormalizedKey, &c.Similarity); err != nil {
			return nil, fmt.Errorf("entity FindMergeCandidatesAnyKind scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListJudgeableMerges returns proposed merges that have not been judged yet (no 'judge'
// key in evidence), enriched with both sides' kinds. Oldest first.
func (r *pgEntityRepo) ListJudgeableMerges(ctx context.Context, limit int) ([]JudgeableMerge, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.id::text, m.from_entity_id::text, ef.canonical_name, ef.kind,
		       m.into_entity_id::text, ei.canonical_name, ei.kind,
		       COALESCE(m.confidence, 0), COALESCE(m.method, '')
		  FROM entity_merges m
		  JOIN entities ef ON ef.id = m.from_entity_id
		  JOIN entities ei ON ei.id = m.into_entity_id
		 WHERE m.status = 'proposed' AND NOT (m.evidence ? 'judge')
		 ORDER BY m.created_at ASC
		 LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("entity ListJudgeableMerges: %w", err)
	}
	defer rows.Close()
	var out []JudgeableMerge
	for rows.Next() {
		var m JudgeableMerge
		if err := rows.Scan(&m.ID, &m.FromID, &m.FromName, &m.FromKind,
			&m.IntoID, &m.IntoName, &m.IntoKind, &m.Confidence, &m.Method); err != nil {
			return nil, fmt.Errorf("entity ListJudgeableMerges scan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// DecideMerge records the sweep's verdict on a proposed merge.
//   - "applied": the pair is the same thing — status→applied, the from-entity's root is
//     repointed at the into-entity's root, and any entities that pointed at from are
//     re-pointed too (depth-1 invariant: canonical_root_id always names a true root, so
//     single-hop reads stay exact).
//   - "rejected": different things — negative evidence, the pair is never re-proposed.
//   - "unsure": stays proposed for manual review; the evidence marks it judged so the
//     sweep never re-spends an LLM call on it (one judgment per pair, ever).
// The evidence map is merged into the row's evidence jsonb.
func (r *pgEntityRepo) DecideMerge(ctx context.Context, mergeID, outcome, decidedBy string, evidence map[string]any) error {
	ev, err := json.Marshal(evidence)
	if err != nil {
		return fmt.Errorf("entity DecideMerge marshal evidence: %w", err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("entity DecideMerge begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string
	switch outcome {
	case "applied", "rejected":
		status = outcome
	case "unsure":
		status = "proposed"
	default:
		return fmt.Errorf("entity DecideMerge: unknown outcome %q", outcome)
	}

	var fromID, intoID string
	err = tx.QueryRow(ctx, `
		UPDATE entity_merges
		   SET status = $2, decided_by = $3,
		       decided_at = CASE WHEN $2 = 'proposed' THEN decided_at ELSE now() END,
		       evidence = evidence || $4::jsonb
		 WHERE id = $1::uuid AND status = 'proposed'
		 RETURNING from_entity_id::text, into_entity_id::text
	`, mergeID, status, decidedBy, ev).Scan(&fromID, &intoID)
	if err != nil {
		return fmt.Errorf("entity DecideMerge update: %w", err)
	}

	if outcome == "applied" {
		if _, err := tx.Exec(ctx, `
			WITH root AS (
				SELECT COALESCE(canonical_root_id, id) AS id FROM entities WHERE id = $2::uuid
			)
			UPDATE entities e
			   SET canonical_root_id = (SELECT id FROM root), updated_at = now()
			 WHERE e.id = $1::uuid OR e.canonical_root_id = $1::uuid
		`, fromID, intoID); err != nil {
			return fmt.Errorf("entity DecideMerge set root: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// HasProposedMergeFor reports whether any proposed merge involves the entity (used to
// hold a placeholder's promotion while its candidate pairs await judgment/review).
func (r *pgEntityRepo) HasProposedMergeFor(ctx context.Context, entityID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM entity_merges
			 WHERE status = 'proposed'
			   AND (from_entity_id = $1::uuid OR into_entity_id = $1::uuid)
		)
	`, entityID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("entity HasProposedMergeFor: %w", err)
	}
	return exists, nil
}

// PromoteEntity resolves a placeholder in place: it is a real, distinct entity.
func (r *pgEntityRepo) PromoteEntity(ctx context.Context, entityID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE entities SET trust_tier = 'believed', updated_at = now()
		 WHERE id = $1::uuid AND trust_tier = 'placeholder'
	`, entityID)
	if err != nil {
		return fmt.Errorf("entity PromoteEntity: %w", err)
	}
	return nil
}

// ListEntityAliases returns an entity's known surfaces (judge input).
func (r *pgEntityRepo) ListEntityAliases(ctx context.Context, entityID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT surface FROM entity_aliases WHERE entity_id = $1::uuid ORDER BY created_at ASC
	`, entityID)
	if err != nil {
		return nil, fmt.Errorf("entity ListEntityAliases: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("entity ListEntityAliases scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
