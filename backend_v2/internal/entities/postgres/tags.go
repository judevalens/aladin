package postgres

import (
	"context"
	"fmt"
	"strings"

	"aladin/backend_v2/internal/auth"
	entitydomain "aladin/backend_v2/internal/entities"
	"aladin/backend_v2/internal/outbox"
	"aladin/backend_v2/internal/treesync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresEntityTagRepository backs the entity tag / @mention picker: entity search for
// the typeahead, create-new, and the artifact↔entity links in artifact_entities.
type PostgresEntityTagRepository struct{ pool *pgxpool.Pool }

func NewEntityTagPostgres(pool *pgxpool.Pool) *PostgresEntityTagRepository {
	return &PostgresEntityTagRepository{pool: pool}
}

// SearchEntities matches entities for the typeahead, alias-aware (P1.1): the query is
// matched against every known surface in entity_aliases (exact > prefix > trigram) AND
// against the entities' own canonical fields (so rows without alias backfill still match).
// Every match is resolved through the merge overlay to its canonical root and deduped —
// a query hitting a merged-away synonym returns one row, the root. Each hit carries its
// alias list for synonym display. ownerUserID "" → shared only.
func (r *PostgresEntityTagRepository) SearchEntities(ctx context.Context, ownerUserID, query string, limit int) ([]entitydomain.EntityHit, error) {
	key := entitydomain.Normalize(query)
	like := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
	prefix := key + "%"

	var owner any
	if strings.TrimSpace(ownerUserID) != "" {
		owner = ownerUserID
	}

	rows, err := r.pool.Query(ctx, `
		WITH alias_matches AS (
			SELECT a.entity_id,
			       MAX(CASE WHEN a.normalized = $1 THEN 3
			                WHEN a.normalized LIKE $4 THEN 2
			                ELSE 1 END) AS tier,
			       MAX(similarity(a.normalized, $1)) AS sim
			  FROM entity_aliases a
			 WHERE a.normalized LIKE $4 OR similarity(a.normalized, $1) >= 0.25
			 GROUP BY a.entity_id
		), direct_matches AS (
			SELECT e.id AS entity_id,
			       CASE WHEN e.normalized_key = $1 THEN 3
			            WHEN e.normalized_key LIKE $4 THEN 2
			            ELSE 1 END AS tier,
			       GREATEST(similarity(e.normalized_key, $1), 0) AS sim
			  FROM entities e
			 WHERE lower(e.canonical_name) LIKE $3 OR e.normalized_key LIKE $4
			    OR similarity(e.normalized_key, $1) >= 0.25
		), all_matches AS (
			SELECT entity_id, MAX(tier) AS tier, MAX(sim) AS sim
			  FROM (SELECT * FROM alias_matches UNION ALL SELECT * FROM direct_matches) m
			 GROUP BY entity_id
		)
		SELECT root.id::text, root.canonical_name, root.kind, root.scope, root.trust_tier,
		       MAX(m.tier) AS tier, MAX(m.sim) AS sim,
		       COALESCE(array_agg(DISTINCT al.surface)
		                FILTER (WHERE al.surface IS NOT NULL
		                          AND lower(al.surface) <> lower(root.canonical_name)), '{}') AS aliases
		  FROM all_matches m
		  JOIN entities e ON e.id = m.entity_id
		  JOIN entities root ON root.id = COALESCE(e.canonical_root_id, e.id)
		  LEFT JOIN entity_aliases al ON al.entity_id = root.id
		 WHERE (e.scope = 'shared' OR (e.scope = 'tenant' AND e.owner_user_id = $2::uuid))
		   AND (root.scope = 'shared' OR (root.scope = 'tenant' AND root.owner_user_id = $2::uuid))
		 GROUP BY root.id, root.canonical_name, root.kind, root.scope, root.trust_tier
		 ORDER BY (root.scope = 'tenant') DESC, tier DESC, sim DESC, root.canonical_name ASC
		 LIMIT $5
	`, key, owner, like, prefix, limit)
	if err != nil {
		return nil, fmt.Errorf("entity tag search: %w", err)
	}
	defer rows.Close()

	out := []entitydomain.EntityHit{}
	for rows.Next() {
		var h entitydomain.EntityHit
		var tier int
		var sim float64
		if err := rows.Scan(&h.ID, &h.Name, &h.Kind, &h.Scope, &h.TrustTier, &tier, &sim, &h.Aliases); err != nil {
			return nil, fmt.Errorf("entity tag search scan: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// CreateEntity is the editor's "@ created something" path (P1.2): alias-aware EXACT
// find-or-create. If ANY known surface — an entity's canonical key OR any alias,
// including a placeholder minted by an earlier @ — already resolves to this key, reuse
// it (resolved through the merge overlay to its canonical root; resolved entities are
// preferred over placeholders, typed over untyped). Otherwise mint a PLACEHOLDER:
// trust_tier='placeholder', canonical alias seeded, no fuzzy work — this path must be
// instant and deterministic. The background judge resolves placeholders later (merge
// into an existing entity, or promote in place). Dedup ignores kind (dedup-by-key), and
// is best-effort: no unique constraint exists (sense-splitting precludes one), so two
// truly-concurrent creates of a brand-new key can still race — acceptable here.
func (r *PostgresEntityTagRepository) CreateEntity(ctx context.Context, kind, canonicalName, normalizedKey string) (entitydomain.EntityHit, error) {
	var h entitydomain.EntityHit
	err := r.pool.QueryRow(ctx, `
		WITH matched AS (
			SELECT e.id, e.canonical_root_id
			  FROM entities e
			 WHERE e.scope = 'shared' AND e.normalized_key = $3
			 UNION
			SELECT a.entity_id AS id, e2.canonical_root_id
			  FROM entity_aliases a
			  JOIN entities e2 ON e2.id = a.entity_id
			 WHERE a.normalized = $3 AND e2.scope = 'shared'
		), existing AS (
			SELECT root.id, root.canonical_name, root.kind, root.scope, root.trust_tier
			  FROM matched m
			  JOIN entities root ON root.id = COALESCE(m.canonical_root_id, m.id)
			 ORDER BY (root.trust_tier <> 'placeholder') DESC,
			          (root.kind NOT IN ('unknown', 'other')) DESC,
			          root.created_at ASC
			 LIMIT 1
		), inserted AS (
			INSERT INTO entities (scope, kind, canonical_name, normalized_key, trust_tier, provenance)
			SELECT 'shared', $1, $2, $3, 'placeholder', '{"source":"manual_tag"}'::jsonb
			 WHERE NOT EXISTS (SELECT 1 FROM existing)
			RETURNING id, canonical_name, kind, scope, trust_tier
		), seeded AS (
			-- canonical-as-alias invariant for freshly minted rows (00020 backfills old ones)
			INSERT INTO entity_aliases (entity_id, surface, normalized, kind, source)
			SELECT id, $2, $3, $1, 'canonical' FROM inserted
			ON CONFLICT (entity_id, normalized) DO NOTHING
		)
		SELECT id::text, canonical_name, kind, scope, trust_tier FROM inserted
		UNION ALL
		SELECT id::text, canonical_name, kind, scope, trust_tier FROM existing
		LIMIT 1
	`, kind, canonicalName, normalizedKey).Scan(&h.ID, &h.Name, &h.Kind, &h.Scope, &h.TrustTier)
	if err != nil {
		return entitydomain.EntityHit{}, fmt.Errorf("entity tag create: %w", err)
	}
	// Keep the wire contract uniform with search: aliases is always an array, never null.
	h.Aliases = []string{}
	return h, nil
}

// AttachTag links an entity to an artifact as a page-level tag (idempotent).
func (r *PostgresEntityTagRepository) AttachTag(ctx context.Context, artifactID, entityID, addedBy string) error {
	principal, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return err
	}
	userID := principal.UserID
	var added any
	if strings.TrimSpace(addedBy) != "" {
		added = addedBy
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("entity tag attach begin: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := outbox.LockUser(ctx, tx, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO artifact_entities (artifact_id, entity_id, origin, added_by)
		VALUES ($1, $2::uuid, 'tag', $3::uuid)
		ON CONFLICT (artifact_id, entity_id, origin, COALESCE(block_id, ''), COALESCE(surface, ''))
		DO NOTHING
	`, artifactID, entityID, added); err != nil {
		return fmt.Errorf("entity tag attach: %w", err)
	}
	if err := treesync.EmitNodeUpsert(ctx, tx, userID, artifactID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DetachTag removes a page-level tag (does not touch projected @mention rows).
func (r *PostgresEntityTagRepository) DetachTag(ctx context.Context, artifactID, entityID string) error {
	principal, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return err
	}
	userID := principal.UserID

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("entity tag detach begin: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := outbox.LockUser(ctx, tx, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM artifact_entities
		 WHERE artifact_id = $1 AND entity_id = $2::uuid AND origin = 'tag'
	`, artifactID, entityID); err != nil {
		return fmt.Errorf("entity tag detach: %w", err)
	}
	if err := treesync.EmitNodeUpsert(ctx, tx, userID, artifactID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ReplaceMentions reconciles the projected @entity mentions for an artifact: in one
// transaction it drops all existing origin='mention' rows for the page and inserts the
// given set (tags, origin='tag', are untouched). The set is the source of truth, derived
// from the page's ydoc on save.
func (r *PostgresEntityTagRepository) ReplaceMentions(ctx context.Context, artifactID string, mentions []entitydomain.MentionRef) error {
	principal, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return err
	}
	userID := principal.UserID

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("entity mention sync begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := outbox.LockUser(ctx, tx, userID); err != nil {
		return err
	}

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
		var snippet any
		if strings.TrimSpace(m.Snippet) != "" {
			snippet = m.Snippet
		}
		// The unique key excludes snippet: editing a block's prose around an existing
		// mention updates the snippet in place rather than duplicating the row.
		if _, err := tx.Exec(ctx, `
			INSERT INTO artifact_entities (artifact_id, entity_id, origin, block_id, surface, snippet)
			VALUES ($1, $2::uuid, 'mention', $3, $4, $5)
			ON CONFLICT (artifact_id, entity_id, origin, COALESCE(block_id, ''), COALESCE(surface, ''))
			DO UPDATE SET snippet = EXCLUDED.snippet
		`, artifactID, m.EntityID, block, surface, snippet); err != nil {
			return fmt.Errorf("entity mention sync insert: %w", err)
		}
	}
	// The page's entity set changed → emit a node frame so reactive views (graph pane) refetch.
	if err := treesync.EmitNodeUpsert(ctx, tx, userID, artifactID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ListForArtifact returns the entities linked to an artifact (tags + projected mentions),
// one row per (entity, origin), name/kind resolved from the entity registry.
func (r *PostgresEntityTagRepository) ListForArtifact(ctx context.Context, artifactID string) ([]entitydomain.AttachedEntity, error) {
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

	out := []entitydomain.AttachedEntity{}
	for rows.Next() {
		var a entitydomain.AttachedEntity
		if err := rows.Scan(&a.ID, &a.Name, &a.Kind, &a.Origin, &a.BlockID); err != nil {
			return nil, fmt.Errorf("entity tag list scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
