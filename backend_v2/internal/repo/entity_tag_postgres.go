package repo

import (
	"context"
	"fmt"
	"strings"

	"aladin/backend_v2/internal/entities"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresEntityTagRepository backs the entity tag / @mention picker: entity search for
// the typeahead, create-new, and the artifact↔entity links in artifact_entities.
type PostgresEntityTagRepository struct{ pool *pgxpool.Pool }

func NewEntityTagPostgres(pool *pgxpool.Pool) *PostgresEntityTagRepository {
	return &PostgresEntityTagRepository{pool: pool}
}

// SearchEntities matches entities for the typeahead: a tenant's own (Tier 1) entities and
// shared (Tier 0) entities whose name contains the query or whose normalized key is
// trigram-similar. Tenant matches and prefix matches rank first. ownerUserID "" → shared
// only.
func (r *PostgresEntityTagRepository) SearchEntities(ctx context.Context, ownerUserID, query string, limit int) ([]coreservice.EntityHit, error) {
	key := entities.Normalize(query)
	like := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
	prefix := key + "%"

	var owner any
	if strings.TrimSpace(ownerUserID) != "" {
		owner = ownerUserID
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id::text, canonical_name, kind, scope, trust_tier,
		       GREATEST(similarity(normalized_key, $1), 0) AS sim
		  FROM entities
		 WHERE (scope = 'shared' OR (scope = 'tenant' AND owner_user_id = $2::uuid))
		   AND (lower(canonical_name) LIKE $3 OR normalized_key LIKE $4 OR similarity(normalized_key, $1) >= 0.2)
		 ORDER BY (scope = 'tenant') DESC,
		          (normalized_key LIKE $4) DESC,
		          sim DESC,
		          canonical_name ASC
		 LIMIT $5
	`, key, owner, like, prefix, limit)
	if err != nil {
		return nil, fmt.Errorf("entity tag search: %w", err)
	}
	defer rows.Close()

	out := []coreservice.EntityHit{}
	for rows.Next() {
		var h coreservice.EntityHit
		var sim float64
		if err := rows.Scan(&h.ID, &h.Name, &h.Kind, &h.Scope, &h.TrustTier, &sim); err != nil {
			return nil, fmt.Errorf("entity tag search scan: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// CreateEntity is find-or-create for the "create new" typeahead path: if a shared (Tier 0)
// entity already exists with the same normalized key (e.g. one the resolver minted from the
// live stream), reuse it instead of minting a duplicate; otherwise create it. A typed
// existing entity is preferred over an 'unknown' one. (The picker often creates with
// kind='unknown', so deduping must ignore kind.) Best-effort: there is no unique constraint,
// so two truly-concurrent creates of a brand-new key can still race — acceptable for this
// manual path.
func (r *PostgresEntityTagRepository) CreateEntity(ctx context.Context, kind, canonicalName, normalizedKey string) (coreservice.EntityHit, error) {
	var h coreservice.EntityHit
	err := r.pool.QueryRow(ctx, `
		WITH existing AS (
			SELECT id, canonical_name, kind, scope, trust_tier
			  FROM entities
			 WHERE scope = 'shared' AND normalized_key = $3
			 ORDER BY (kind <> 'unknown') DESC, created_at ASC
			 LIMIT 1
		), inserted AS (
			INSERT INTO entities (scope, kind, canonical_name, normalized_key, trust_tier, provenance)
			SELECT 'shared', $1, $2, $3, 'believed', '{"source":"manual_tag"}'::jsonb
			 WHERE NOT EXISTS (SELECT 1 FROM existing)
			RETURNING id, canonical_name, kind, scope, trust_tier
		)
		SELECT id::text, canonical_name, kind, scope, trust_tier FROM inserted
		UNION ALL
		SELECT id::text, canonical_name, kind, scope, trust_tier FROM existing
		LIMIT 1
	`, kind, canonicalName, normalizedKey).Scan(&h.ID, &h.Name, &h.Kind, &h.Scope, &h.TrustTier)
	if err != nil {
		return coreservice.EntityHit{}, fmt.Errorf("entity tag create: %w", err)
	}
	return h, nil
}

// AttachTag links an entity to an artifact as a page-level tag (idempotent).
func (r *PostgresEntityTagRepository) AttachTag(ctx context.Context, artifactID, entityID, addedBy string) error {
	var added any
	if strings.TrimSpace(addedBy) != "" {
		added = addedBy
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO artifact_entities (artifact_id, entity_id, origin, added_by)
		VALUES ($1, $2::uuid, 'tag', $3::uuid)
		ON CONFLICT (artifact_id, entity_id, origin, COALESCE(block_id, ''), COALESCE(surface, ''))
		DO NOTHING
	`, artifactID, entityID, added)
	if err != nil {
		return fmt.Errorf("entity tag attach: %w", err)
	}
	return nil
}

// DetachTag removes a page-level tag (does not touch projected @mention rows).
func (r *PostgresEntityTagRepository) DetachTag(ctx context.Context, artifactID, entityID string) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM artifact_entities
		 WHERE artifact_id = $1 AND entity_id = $2::uuid AND origin = 'tag'
	`, artifactID, entityID)
	if err != nil {
		return fmt.Errorf("entity tag detach: %w", err)
	}
	return nil
}

// ReplaceMentions reconciles the projected @entity mentions for an artifact: in one
// transaction it drops all existing origin='mention' rows for the page and inserts the
// given set (tags, origin='tag', are untouched). The set is the source of truth, derived
// from the page's ydoc on save.
func (r *PostgresEntityTagRepository) ReplaceMentions(ctx context.Context, artifactID string, mentions []coreservice.MentionRef) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("entity mention sync begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		DELETE FROM artifact_entities WHERE artifact_id = $1 AND origin = 'mention'
	`, artifactID); err != nil {
		return fmt.Errorf("entity mention sync clear: %w", err)
	}

	for _, m := range mentions {
		var surface any
		if strings.TrimSpace(m.Surface) != "" {
			surface = m.Surface
		}
		var block any
		if strings.TrimSpace(m.BlockID) != "" {
			block = m.BlockID
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO artifact_entities (artifact_id, entity_id, origin, block_id, surface)
			VALUES ($1, $2::uuid, 'mention', $3, $4)
			ON CONFLICT (artifact_id, entity_id, origin, COALESCE(block_id, ''), COALESCE(surface, ''))
			DO NOTHING
		`, artifactID, m.EntityID, block, surface); err != nil {
			return fmt.Errorf("entity mention sync insert: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// ListForArtifact returns the entities linked to an artifact (tags + projected mentions),
// one row per (entity, origin), name/kind resolved from the entity registry.
func (r *PostgresEntityTagRepository) ListForArtifact(ctx context.Context, artifactID string) ([]coreservice.AttachedEntity, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT e.id::text, e.canonical_name, e.kind, ae.origin, COALESCE(ae.block_id, '')
		  FROM artifact_entities ae
		  JOIN entities e ON e.id = ae.entity_id
		 WHERE ae.artifact_id = $1
		 ORDER BY ae.origin, e.canonical_name
	`, artifactID)
	if err != nil {
		return nil, fmt.Errorf("entity tag list: %w", err)
	}
	defer rows.Close()

	out := []coreservice.AttachedEntity{}
	for rows.Next() {
		var a coreservice.AttachedEntity
		if err := rows.Scan(&a.ID, &a.Name, &a.Kind, &a.Origin, &a.BlockID); err != nil {
			return nil, fmt.Errorf("entity tag list scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
