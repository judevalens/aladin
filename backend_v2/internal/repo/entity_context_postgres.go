package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresEntityContextRepository backs the Entity Context surface. Nothing here is a
// new store: identity comes from `entities`, edges from `relationships` (widened to
// entity endpoints in 00021), and Context is DERIVED from the material already linked to
// the entity — records that mention it (quotes) and pages that @mention it (your notes).
// Bodies are selected as-is; this layer never rewrites user or source material.
type PostgresEntityContextRepository struct{ pool *pgxpool.Pool }

func NewEntityContextPostgres(pool *pgxpool.Pool) *PostgresEntityContextRepository {
	return &PostgresEntityContextRepository{pool: pool}
}

// providerPlatform maps a record's provider to the surface's platform chip token.
// Unknown providers → "" (chip hidden) rather than a wrong label.
func providerPlatform(provider string) string {
	switch provider {
	case "hackernews":
		return "hn"
	case "reddit":
		return "reddit"
	case "bluesky":
		return "bluesky"
	case "rss":
		return "rss"
	default:
		return ""
	}
}

func (r *PostgresEntityContextRepository) ResolveRoot(ctx context.Context, entityID string) (string, error) {
	var root string
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(canonical_root_id, id)::text FROM entities WHERE id = $1::uuid
	`, entityID).Scan(&root)
	if err != nil {
		return "", fmt.Errorf("entity context resolve root: %w", err)
	}
	return root, nil
}

func (r *PostgresEntityContextRepository) Exists(ctx context.Context, entityID string) (bool, error) {
	var ok bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM entities WHERE id = $1::uuid)`, entityID).Scan(&ok); err != nil {
		return false, fmt.Errorf("entity context exists: %w", err)
	}
	return ok, nil
}

// Identity returns the focused node. `Since` carries only the AGE phrase here (e.g.
// "3 weeks"); the service composes the full provenance line once it knows the context
// count.
func (r *PostgresEntityContextRepository) Identity(ctx context.Context, entityID string) (coreservice.EntityIdentity, error) {
	var out coreservice.EntityIdentity
	var gist *string
	var trustTier string
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, canonical_name, kind, gist, trust_tier,
		       CASE
		         WHEN now() - created_at < interval '1 day'  THEN 'today'
		         WHEN now() - created_at < interval '14 days'
		           THEN floor(extract(epoch FROM now() - created_at) / 86400)::int || ' days'
		         WHEN now() - created_at < interval '60 days'
		           THEN floor(extract(epoch FROM now() - created_at) / 604800)::int || ' weeks'
		         ELSE floor(extract(epoch FROM now() - created_at) / 2592000)::int || ' months'
		       END AS age
		  FROM entities WHERE id = $1::uuid
	`, entityID).Scan(&out.ID, &out.Name, &out.Kind, &gist, &trustTier, &out.Since)
	if err != nil {
		return coreservice.EntityIdentity{}, fmt.Errorf("entity context identity: %w", err)
	}
	if gist != nil {
		out.Gist = *gist
	}
	out.Placeholder = trustTier == "placeholder"
	out.Watchers = []string{} // per-entity watchers don't exist; the surface hides the row
	return out, nil
}

// EdgesFor returns the entity's typed edges in BOTH directions, always oriented FROM the
// focused entity: an inbound `A enables B` is read from B as `B enabled_by A`, so the
// page always says how the *focused* entity relates. Only inverses with a named opposite
// flip; symmetric relations (competes) and ones without an inverse are shown as-is.
func (r *PostgresEntityContextRepository) EdgesFor(ctx context.Context, ownerUserID, entityID string) ([]coreservice.EntityEdge, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT rel.rel_type, other.id::text, other.canonical_name, other.kind,
		       COALESCE(rel.metadata->>'why', ''), COALESCE(rel.metadata->>'origin', 'you'),
		       (rel.src_id = $2) AS outbound
		  FROM relationships rel
		  JOIN entities other
		    ON other.id = (CASE WHEN rel.src_id = $2 THEN rel.dst_id ELSE rel.src_id END)::uuid
		 WHERE rel.user_id = $1::uuid
		   AND rel.src_kind = 'entity' AND rel.dst_kind = 'entity'
		   AND (rel.src_id = $2 OR rel.dst_id = $2)
		 ORDER BY rel.created_at ASC
	`, ownerUserID, entityID)
	if err != nil {
		return nil, fmt.Errorf("entity context edges: %w", err)
	}
	defer rows.Close()

	out := []coreservice.EntityEdge{}
	for rows.Next() {
		var e coreservice.EntityEdge
		var outbound bool
		if err := rows.Scan(&e.Rel, &e.ToID, &e.To, &e.Kind, &e.Why, &e.Origin, &outbound); err != nil {
			return nil, fmt.Errorf("entity context edges scan: %w", err)
		}
		if !outbound {
			e.Rel = inverseRel(e.Rel)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// inverseRel names the reading of a relation from the far end. Relations without a named
// inverse (supports/contradicts/competes read the same both ways in this vocabulary) are
// returned unchanged.
func inverseRel(rel string) string {
	switch rel {
	case coreservice.RelEnables:
		return coreservice.RelEnabledBy
	case coreservice.RelEnabledBy:
		return coreservice.RelEnables
	case coreservice.RelPartOf:
		return coreservice.RelInstance
	case coreservice.RelInstance:
		return coreservice.RelPartOf
	default:
		return rel
	}
}

// ContextFor derives the accreted material for an entity, newest first:
//   - quotes: records that mention it, body = the record's own text, attributed to the
//     source's handle / subreddit / title.
//   - notes: the user's pages that @mention it, body = the block's snippet — what they
//     actually wrote. Rows without a snippet (tags, or mentions projected before 00021)
//     are skipped rather than shown as empty bodies.
//
// Both arms select stored text unchanged (the verbatim guarantee). Questions have no
// store yet (Phase C).
func (r *PostgresEntityContextRepository) ContextFor(ctx context.Context, ownerUserID, entityID string) ([]coreservice.EntityContextItem, error) {
	var owner any
	if strings.TrimSpace(ownerUserID) != "" {
		owner = ownerUserID
	}
	rows, err := r.pool.Query(ctx, `
		WITH quotes AS (
			SELECT 'quote' AS type,
			       rec.content AS body,
			       COALESCE(
			         NULLIF('@' || (rec.metadata->>'author_handle'), '@'),
			         NULLIF(rec.metadata->>'by', ''),
			         NULLIF('r/' || (rec.metadata->>'subreddit'), 'r/'),
			         NULLIF(rec.label, ''),
			         COALESCE(rec.provider, 'source')
			       ) AS who,
			       COALESCE(rec.provider, '') AS provider,
			       '' AS source_id,
			       rec.created_at AS at
			  FROM entity_mentions em
			  JOIN records rec ON rec.id = em.record_id
			 WHERE em.entity_id = $1::uuid
			   AND rec.content <> ''
			   AND (rec.owner_user_id IS NULL OR rec.owner_user_id = $2::uuid)
		), notes AS (
			SELECT 'note' AS type,
			       ae.snippet AS body,
			       'me' AS who,
			       '' AS provider,
			       ae.artifact_id AS source_id,
			       ae.created_at AS at
			  FROM artifact_entities ae
			  JOIN artifacts a ON a.id = ae.artifact_id
			 WHERE ae.entity_id = $1::uuid
			   AND ae.origin = 'mention'
			   AND COALESCE(ae.snippet, '') <> ''
			   AND a.user_id = $2::uuid
		)
		SELECT type, body, who, provider, source_id,
		       CASE
		         WHEN now() - at < interval '1 hour'
		           THEN GREATEST(floor(extract(epoch FROM now() - at) / 60)::int, 1) || 'm'
		         WHEN now() - at < interval '1 day'
		           THEN floor(extract(epoch FROM now() - at) / 3600)::int || 'h'
		         WHEN now() - at < interval '30 days'
		           THEN floor(extract(epoch FROM now() - at) / 86400)::int || 'd'
		         ELSE floor(extract(epoch FROM now() - at) / 2592000)::int || 'mo'
		       END AS rel_time
		  FROM (SELECT * FROM quotes UNION ALL SELECT * FROM notes) items
		 ORDER BY at DESC
		 LIMIT 100
	`, entityID, owner)
	if err != nil {
		return nil, fmt.Errorf("entity context items: %w", err)
	}
	defer rows.Close()

	out := []coreservice.EntityContextItem{}
	for rows.Next() {
		var it coreservice.EntityContextItem
		var provider string
		if err := rows.Scan(&it.Type, &it.Body, &it.Who, &provider, &it.SourceID, &it.Time); err != nil {
			return nil, fmt.Errorf("entity context items scan: %w", err)
		}
		it.Platform = providerPlatform(provider)
		out = append(out, it)
	}
	return out, rows.Err()
}

// PendingMergesFor returns the judge's unanswered questions about this entity's identity
// — the "Same, or just similar?" panel. Both directions: a proposal names two entities
// and is a question about each, so it must surface from whichever one you're looking at.
//
// A dedicated query rather than db.ListProposedMerges, which is global (no entity filter)
// and drops `evidence` — and evidence is the whole point here: it carries the judge's
// verdict and reasoning.
func (r *PostgresEntityContextRepository) PendingMergesFor(ctx context.Context, entityID string) ([]coreservice.PendingMerge, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.id::text,
		       other.id::text, other.canonical_name, other.kind,
		       COALESCE(m.confidence, 0),
		       COALESCE(m.evidence->'judge'->>'verdict', ''),
		       COALESCE(m.evidence->'judge'->>'reason', ''),
		       COALESCE(m.method, '')
		  FROM entity_merges m
		  JOIN entities other
		    ON other.id = (CASE WHEN m.from_entity_id = $1::uuid
		                        THEN m.into_entity_id ELSE m.from_entity_id END)
		 WHERE m.status = 'proposed'
		   AND (m.from_entity_id = $1::uuid OR m.into_entity_id = $1::uuid)
		 ORDER BY m.confidence DESC NULLS LAST, m.created_at ASC
	`, entityID)
	if err != nil {
		return nil, fmt.Errorf("entity context pending merges: %w", err)
	}
	defer rows.Close()

	out := []coreservice.PendingMerge{}
	for rows.Next() {
		var p coreservice.PendingMerge
		var verdict, method string
		if err := rows.Scan(&p.MergeID, &p.OtherID, &p.OtherName, &p.OtherKind,
			&p.Confidence, &verdict, &p.Reason, &method); err != nil {
			return nil, fmt.Errorf("entity context pending merges scan: %w", err)
		}
		p.Suggestion = mergeSuggestion(verdict)
		p.Why = mergeWhy(p.Confidence, method)
		out = append(out, p)
	}
	return out, rows.Err()
}

// MergeQueue is the global inbox: every proposed merge, both sides named, newest-confident
// first. Excludes pairs where either side was already merged away (canonical_root_id set),
// since those are stale questions the overlay already answered.
func (r *PostgresEntityContextRepository) MergeQueue(ctx context.Context, limit int) ([]coreservice.MergeQueueItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.id::text,
		       ef.id::text, ef.canonical_name, ef.kind,
		       ei.id::text, ei.canonical_name, ei.kind,
		       COALESCE(m.confidence, 0),
		       COALESCE(m.evidence->'judge'->>'verdict', ''),
		       COALESCE(m.method, '')
		  FROM entity_merges m
		  JOIN entities ef ON ef.id = m.from_entity_id
		  JOIN entities ei ON ei.id = m.into_entity_id
		 WHERE m.status = 'proposed'
		   AND ef.canonical_root_id IS NULL
		   AND ei.canonical_root_id IS NULL
		 ORDER BY m.confidence DESC NULLS LAST, m.created_at ASC
		 LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("entity merge queue: %w", err)
	}
	defer rows.Close()

	out := []coreservice.MergeQueueItem{}
	for rows.Next() {
		var it coreservice.MergeQueueItem
		var verdict, method string
		if err := rows.Scan(&it.MergeID, &it.FromID, &it.FromName, &it.FromKind,
			&it.IntoID, &it.IntoName, &it.IntoKind, &it.Confidence, &verdict, &method); err != nil {
			return nil, fmt.Errorf("entity merge queue scan: %w", err)
		}
		it.Suggestion = mergeSuggestion(verdict)
		it.Why = mergeWhy(it.Confidence, method)
		out = append(out, it)
	}
	return out, rows.Err()
}

// mergeSuggestion reads the judge's verdict. An un-judged pair (no key, or the judge
// hasn't swept yet) is honestly "unsure" — never a guess.
func mergeSuggestion(verdict string) string {
	switch verdict {
	case "same":
		return coreservice.MergeSuggestSynonym
	case "different":
		return coreservice.MergeSuggestDistinct
	default:
		return coreservice.MergeSuggestUnsure
	}
}

// mergeWhy renders the evidence that surfaced the pair, in plain language.
func mergeWhy(confidence float64, method string) string {
	parts := []string{}
	if confidence > 0 {
		parts = append(parts, fmt.Sprintf("name overlap %d%%", int(confidence*100+0.5)))
	}
	if method == "placeholder_sweep" {
		parts = append(parts, "from an unresolved @mention")
	}
	if len(parts) == 0 {
		return "surfaced for review"
	}
	return strings.Join(parts, " · ")
}

// InsertEdge writes a typed, provenance-carrying edge. Idempotent on
// (user, src, dst, rel_type) via uq_relationships_edge — redrawing the same edge updates
// its why rather than duplicating it.
func (r *PostgresEntityContextRepository) InsertEdge(ctx context.Context, in coreservice.DrawEdgeInput) error {
	metadata, err := json.Marshal(map[string]string{
		"why":    in.Why,
		"origin": coreservice.EdgeOriginYou,
	})
	if err != nil {
		return fmt.Errorf("entity context insert edge marshal: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO relationships (user_id, src_kind, src_id, dst_kind, dst_id, rel_type, metadata)
		VALUES ($1::uuid, 'entity', $2, 'entity', $3, $4, $5::jsonb)
		ON CONFLICT (user_id, src_kind, src_id, dst_kind, dst_id, rel_type)
		DO UPDATE SET metadata = EXCLUDED.metadata
	`, in.OwnerUserID, in.FromID, in.ToID, in.Rel, metadata)
	if err != nil {
		return fmt.Errorf("entity context insert edge: %w", err)
	}
	return nil
}
