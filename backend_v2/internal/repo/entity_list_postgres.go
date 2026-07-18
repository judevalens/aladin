package repo

import (
	"context"
	"fmt"
	"strings"

	"aladin/backend_v2/internal/entities"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresEntityListRepository backs the Entities index. No new store: it aggregates
// over the registry (entities/entity_aliases) plus the three things that make an entity
// interesting — what it's wired to (relationships), where it was seen (entity_mentions +
// artifact_entities), and what the judge still wants decided (entity_merges).
type PostgresEntityListRepository struct{ pool *pgxpool.Pool }

func NewEntityListPostgres(pool *pgxpool.Pool) *PostgresEntityListRepository {
	return &PostgresEntityListRepository{pool: pool}
}

// List returns the index cards.
//
// Three invariants worth stating, because getting any of them wrong is silently wrong:
//   - SCOPE: shared entities plus only THIS user's tenant entities. Same predicate as
//     SearchEntities — without it a tenant's private entities leak into another's index.
//   - ROOTS ONLY: `canonical_root_id IS NULL` — merged-away entities must never surface;
//     the judge already answered those.
//   - EMPTY QUERY LISTS ALL: the typeahead returns nothing for "" (correct for a
//     picker); an index must browse. The alias CTE is gated so it short-circuits.
//
// Counts come from LEFT JOIN LATERAL per row rather than N+1 round-trips.
func (r *PostgresEntityListRepository) List(ctx context.Context, q coreservice.EntityListQuery) ([]coreservice.EntityListItem, error) {
	key := entities.Normalize(q.Query)
	like := "%" + strings.ToLower(q.Query) + "%"
	prefix := key + "%"

	var owner any
	if strings.TrimSpace(q.OwnerUserID) != "" {
		owner = q.OwnerUserID
	}

	rows, err := r.pool.Query(ctx, `
		WITH matched AS (
			-- Alias-aware match, same shape as the picker's search: any known surface
			-- finds the entity. Gated on a non-empty query so the browse path skips it.
			SELECT a.entity_id
			  FROM entity_aliases a
			 WHERE NULLIF($2, '') IS NOT NULL
			   AND (a.normalized LIKE $4 OR similarity(a.normalized, $2) >= 0.25)
			 UNION
			SELECT e.id AS entity_id
			  FROM entities e
			 WHERE NULLIF($2, '') IS NOT NULL
			   AND (lower(e.canonical_name) LIKE $3 OR e.normalized_key LIKE $4
			        OR similarity(e.normalized_key, $2) >= 0.25)
		), base AS (
			SELECT e.id, e.canonical_name, e.kind, COALESCE(e.gist, '') AS gist,
			       e.trust_tier, e.updated_at
			  FROM entities e
			 WHERE e.canonical_root_id IS NULL
			   AND (e.scope = 'shared' OR (e.scope = 'tenant' AND e.owner_user_id = $1::uuid))
			   AND ($5 = '' OR e.kind = $5)
			   -- $8 status filter: 'unresolved' → placeholder tier (cheap, no aggregate)
			   AND ($8 <> 'unresolved' OR e.trust_tier = 'placeholder')
			   AND (NULLIF($2, '') IS NULL
			        OR e.id IN (SELECT entity_id FROM matched)
			        OR EXISTS (SELECT 1 FROM entities m
			                    WHERE m.canonical_root_id = e.id
			                      AND m.id IN (SELECT entity_id FROM matched)))
		)
		SELECT b.id::text, b.canonical_name, b.kind, b.gist, b.trust_tier,
		       to_char(b.updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS updated_at,
		       l.n AS links, s.n AS sources, a.n AS attention,
		       COALESCE(al.surfaces, '{}') AS aliases
		  FROM base b
		  LEFT JOIN LATERAL (
		      -- relationships.src_id/dst_id are TEXT (the table is polymorphic), so compare
		      -- as text to keep idx_relationships_src/_dst usable.
		      SELECT count(*) AS n FROM relationships r
		       WHERE r.user_id = $1::uuid
		         AND r.src_kind = 'entity' AND r.dst_kind = 'entity'
		         AND (r.src_id = b.id::text OR r.dst_id = b.id::text)
		  ) l ON true
		  LEFT JOIN LATERAL (
		      -- Same user-scoping as the detail page's ContextFor, so a card's count and
		      -- the page's material agree.
		      SELECT count(*) AS n FROM (
		          SELECT em.record_id FROM entity_mentions em
		            JOIN records rec ON rec.id = em.record_id
		           WHERE em.entity_id = b.id
		             AND (rec.owner_user_id IS NULL OR rec.owner_user_id = $1::uuid)
		          UNION ALL
		          SELECT ae.artifact_id FROM artifact_entities ae
		            JOIN artifacts art ON art.id = ae.artifact_id
		           WHERE ae.entity_id = b.id AND art.user_id = $1::uuid
		      ) src
		  ) s ON true
		  LEFT JOIN LATERAL (
		      -- BOTH directions: a proposal names two entities and is a question about each.
		      SELECT count(*) AS n FROM entity_merges m
		       WHERE m.status = 'proposed'
		         AND (m.from_entity_id = b.id OR m.into_entity_id = b.id)
		  ) a ON true
		  LEFT JOIN LATERAL (
		      SELECT array_agg(x.surface) AS surfaces FROM (
		          SELECT al2.surface FROM entity_aliases al2
		           WHERE al2.entity_id = b.id
		             AND lower(al2.surface) <> lower(b.canonical_name)
		           ORDER BY al2.created_at LIMIT 4
		      ) x
		  ) al ON true
		 -- $8 status filter: 'pending' → only entities with an open merge question.
		 WHERE ($8 <> 'pending' OR a.n > 0)
		 ORDER BY
		   CASE WHEN $6 = 'attention' THEN a.n END DESC NULLS LAST,
		   CASE WHEN $6 = 'links' THEN l.n END DESC NULLS LAST,
		   CASE WHEN $6 = 'name' THEN b.canonical_name END ASC NULLS LAST,
		   b.canonical_name ASC
		 LIMIT $7
	`, owner, key, like, prefix, q.Kind, q.Sort, q.Limit, q.Filter)
	if err != nil {
		return nil, fmt.Errorf("entity list: %w", err)
	}
	defer rows.Close()

	out := []coreservice.EntityListItem{}
	for rows.Next() {
		var it coreservice.EntityListItem
		if err := rows.Scan(&it.ID, &it.Name, &it.Kind, &it.Gist, &it.TrustTier, &it.UpdatedAt,
			&it.Links, &it.Sources, &it.Attention, &it.Aliases); err != nil {
			return nil, fmt.Errorf("entity list scan: %w", err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// Summary describes the whole visible registry (never the filtered subset — a search
// shouldn't make the layer look smaller than it is).
func (r *PostgresEntityListRepository) Summary(ctx context.Context, ownerUserID string) (coreservice.EntitySummary, error) {
	var owner any
	if strings.TrimSpace(ownerUserID) != "" {
		owner = ownerUserID
	}
	out := coreservice.EntitySummary{Tiers: map[string]int{}}

	rows, err := r.pool.Query(ctx, `
		SELECT trust_tier, count(*)
		  FROM entities
		 WHERE canonical_root_id IS NULL
		   AND (scope = 'shared' OR (scope = 'tenant' AND owner_user_id = $1::uuid))
		 GROUP BY trust_tier
	`, owner)
	if err != nil {
		return out, fmt.Errorf("entity list summary tiers: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var tier string
		var n int
		if err := rows.Scan(&tier, &n); err != nil {
			return out, fmt.Errorf("entity list summary scan: %w", err)
		}
		out.Tiers[tier] = n
		out.Total += n
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("entity list summary rows: %w", err)
	}

	if err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM entity_merges WHERE status = 'proposed'
	`).Scan(&out.PendingDecisions); err != nil {
		return out, fmt.Errorf("entity list summary pending: %w", err)
	}
	return out, nil
}
