package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresGraphPaneRepository struct{ pool *pgxpool.Pool }

func NewGraphPanePostgres(pool *pgxpool.Pool) *PostgresGraphPaneRepository {
	return &PostgresGraphPaneRepository{pool: pool}
}

// ForThesis assembles the "On the graph" pane for a thesis claim: its entities (with
// mention counts), the claims about those entities (each grounded in N sources or not),
// and the cited source records behind them. Empty pane (nil thesis) if the claim is gone.
func (r *PostgresGraphPaneRepository) ForThesis(ctx context.Context, thesisClaimID string) (*coreservice.GraphPane, error) {
	pane := &coreservice.GraphPane{
		Claims:          []coreservice.GraphClaim{},
		Cites:           []coreservice.GraphCite{},
		Entities:        []coreservice.GraphEntity{},
		LinkedArtifacts: []coreservice.GraphLinkedArtifact{},
	}

	var th coreservice.GraphThesis
	var updated time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, canonical_text, polarity, trust_tier, updated_at
		  FROM claims WHERE id = $1::uuid
	`, thesisClaimID).Scan(&th.ID, &th.Text, &th.Polarity, &th.TrustTier, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return pane, nil
	}
	if err != nil {
		return nil, fmt.Errorf("graph pane thesis: %w", err)
	}
	th.UpdatedAt = updated.Format(time.RFC3339)
	pane.Thesis = &th

	// Entities the thesis is about, with how many times each is mentioned anywhere.
	entRows, err := r.pool.Query(ctx, `
		SELECT e.id::text, e.canonical_name, e.kind, count(em.id) AS mentions
		  FROM claim_subjects cs
		  JOIN entities e ON e.id = cs.entity_id
		  LEFT JOIN entity_mentions em ON em.entity_id = e.id
		 WHERE cs.claim_id = $1::uuid
		 GROUP BY e.id, e.canonical_name, e.kind
		 ORDER BY mentions DESC
	`, thesisClaimID)
	if err != nil {
		return nil, fmt.Errorf("graph pane entities: %w", err)
	}
	defer entRows.Close()
	for entRows.Next() {
		var e coreservice.GraphEntity
		if err := entRows.Scan(&e.ID, &e.Name, &e.Kind, &e.Mentions); err != nil {
			return nil, fmt.Errorf("graph pane entity scan: %w", err)
		}
		e.Origin = "claim"
		pane.Entities = append(pane.Entities, e)
	}
	if err := entRows.Err(); err != nil {
		return nil, err
	}

	// Claims about any of the thesis's entities (excluding the thesis itself), with the
	// count of distinct sources backing each. Grounded = backed by >= 1 source.
	claimRows, err := r.pool.Query(ctx, `
		SELECT c.id::text, c.canonical_text, count(DISTINCT cm.source_id) AS sources
		  FROM claims c
		  JOIN claim_subjects cs ON cs.claim_id = c.id
		  LEFT JOIN claim_mentions cm ON cm.claim_id = c.id
		 WHERE c.id <> $1::uuid
		   AND cs.entity_id IN (SELECT entity_id FROM claim_subjects WHERE claim_id = $1::uuid)
		 GROUP BY c.id, c.canonical_text
		 ORDER BY sources DESC, c.created_at DESC
		 LIMIT 25
	`, thesisClaimID)
	if err != nil {
		return nil, fmt.Errorf("graph pane claims: %w", err)
	}
	defer claimRows.Close()
	for claimRows.Next() {
		var c coreservice.GraphClaim
		if err := claimRows.Scan(&c.ID, &c.Text, &c.Sources); err != nil {
			return nil, fmt.Errorf("graph pane claim scan: %w", err)
		}
		c.Grounded = c.Sources > 0
		pane.Claims = append(pane.Claims, c)
	}
	if err := claimRows.Err(); err != nil {
		return nil, err
	}

	// Cited sources: the records mentioned by any claim about the thesis's entities.
	citeRows, err := r.pool.Query(ctx, `
		SELECT DISTINCT r.id, r.label, COALESCE(r.source_url, ''), COALESCE(r.provider, ''), r.created_at
		  FROM claim_mentions cm
		  JOIN records r ON r.id = cm.source_id
		 WHERE cm.source_kind = 'record'
		   AND cm.claim_id IN (
		       SELECT cs2.claim_id FROM claim_subjects cs2
		        WHERE cs2.entity_id IN (SELECT entity_id FROM claim_subjects WHERE claim_id = $1::uuid)
		   )
		 ORDER BY r.created_at DESC
		 LIMIT 25
	`, thesisClaimID)
	if err != nil {
		return nil, fmt.Errorf("graph pane cites: %w", err)
	}
	defer citeRows.Close()
	for citeRows.Next() {
		var c coreservice.GraphCite
		var created time.Time
		if err := citeRows.Scan(&c.ID, &c.Title, &c.SourceURL, &c.Provider, &created); err != nil {
			return nil, fmt.Errorf("graph pane cite scan: %w", err)
		}
		c.CreatedAt = created.Format(time.RFC3339)
		pane.Cites = append(pane.Cites, c)
	}
	return pane, citeRows.Err()
}

// ForArtifact assembles the "On the graph" pane for the artifact you're viewing: the
// claims grounded in it (authored extraction), the entities those claims are about
// (with mention counts), and the source records backing them. Thesis is nil — an
// artifact pane is rooted in the page itself, not a single thesis. An empty pane (no
// claims/entities/cites) means the page hasn't been analyzed into the graph yet.
func (r *PostgresGraphPaneRepository) ForArtifact(ctx context.Context, artifactID string) (*coreservice.GraphPane, error) {
	pane := &coreservice.GraphPane{
		Claims:          []coreservice.GraphClaim{},
		Cites:           []coreservice.GraphCite{},
		Entities:        []coreservice.GraphEntity{},
		LinkedArtifacts: []coreservice.GraphLinkedArtifact{},
	}

	// Claims grounded in this artifact, with the count of distinct record sources also
	// backing each. Grounded = backed by >= 1 record source.
	claimRows, err := r.pool.Query(ctx, `
		SELECT c.id::text, c.canonical_text,
		       count(DISTINCT cm2.source_id) FILTER (WHERE cm2.source_kind = 'record') AS sources
		  FROM claims c
		  JOIN claim_mentions cm ON cm.claim_id = c.id
		   AND cm.source_kind = 'artifact' AND cm.source_id = $1
		  LEFT JOIN claim_mentions cm2 ON cm2.claim_id = c.id
		 GROUP BY c.id, c.canonical_text
		 ORDER BY sources DESC, c.created_at DESC
		 LIMIT 25
	`, artifactID)
	if err != nil {
		return nil, fmt.Errorf("graph pane artifact claims: %w", err)
	}
	defer claimRows.Close()
	for claimRows.Next() {
		var c coreservice.GraphClaim
		if err := claimRows.Scan(&c.ID, &c.Text, &c.Sources); err != nil {
			return nil, fmt.Errorf("graph pane artifact claim scan: %w", err)
		}
		c.Grounded = c.Sources > 0
		pane.Claims = append(pane.Claims, c)
	}
	if err := claimRows.Err(); err != nil {
		return nil, err
	}

	// Entities connected to this artifact, unioned across origins: subjects of the
	// artifact's claims ('claim'), plus tags and projected @mentions from
	// artifact_entities ('tag'/'mention'). Origin precedence tag > mention > claim, with
	// how many times each entity is mentioned anywhere.
	entRows, err := r.pool.Query(ctx, `
		WITH rel AS (
		    SELECT cs.entity_id AS eid, 'claim'::text AS origin
		      FROM claim_subjects cs
		     WHERE cs.claim_id IN (
		         SELECT claim_id FROM claim_mentions
		          WHERE source_kind = 'artifact' AND source_id = $1)
		    UNION ALL
		    SELECT ae.entity_id, ae.origin
		      FROM artifact_entities ae
		     WHERE ae.artifact_id = $1
		),
		agg AS (
		    SELECT eid,
		           CASE WHEN bool_or(origin = 'tag')     THEN 'tag'
		                WHEN bool_or(origin = 'mention') THEN 'mention'
		                ELSE 'claim' END AS origin
		      FROM rel GROUP BY eid
		)
		SELECT e.id::text, e.canonical_name, e.kind, agg.origin, count(em.id) AS mentions
		  FROM agg
		  JOIN entities e ON e.id = agg.eid
		  LEFT JOIN entity_mentions em ON em.entity_id = e.id
		 GROUP BY e.id, e.canonical_name, e.kind, agg.origin
		 ORDER BY mentions DESC, e.canonical_name ASC
	`, artifactID)
	if err != nil {
		return nil, fmt.Errorf("graph pane artifact entities: %w", err)
	}
	defer entRows.Close()
	for entRows.Next() {
		var e coreservice.GraphEntity
		if err := entRows.Scan(&e.ID, &e.Name, &e.Kind, &e.Origin, &e.Mentions); err != nil {
			return nil, fmt.Errorf("graph pane artifact entity scan: %w", err)
		}
		pane.Entities = append(pane.Entities, e)
	}
	if err := entRows.Err(); err != nil {
		return nil, err
	}

	// Cited sources: the records backing this artifact's claims.
	citeRows, err := r.pool.Query(ctx, `
		SELECT DISTINCT r.id, r.label, COALESCE(r.source_url, ''), COALESCE(r.provider, ''), r.created_at
		  FROM claim_mentions cm
		  JOIN records r ON r.id = cm.source_id
		 WHERE cm.source_kind = 'record'
		   AND cm.claim_id IN (
		       SELECT claim_id FROM claim_mentions
		        WHERE source_kind = 'artifact' AND source_id = $1
		   )
		 ORDER BY r.created_at DESC
		 LIMIT 25
	`, artifactID)
	if err != nil {
		return nil, fmt.Errorf("graph pane artifact cites: %w", err)
	}
	defer citeRows.Close()
	for citeRows.Next() {
		var c coreservice.GraphCite
		var created time.Time
		if err := citeRows.Scan(&c.ID, &c.Title, &c.SourceURL, &c.Provider, &created); err != nil {
			return nil, fmt.Errorf("graph pane artifact cite scan: %w", err)
		}
		c.CreatedAt = created.Format(time.RFC3339)
		pane.Cites = append(pane.Cites, c)
	}
	if err := citeRows.Err(); err != nil {
		return nil, err
	}

	// Other artifacts connected to this one: # cross-references in either direction
	// (referenced_by / references) and artifacts that share an entity with it
	// (shared_entity). Direct refs win over shared-entity when both hold.
	linkRows, err := r.pool.Query(ctx, `
		WITH links AS (
		    SELECT ar.artifact_id AS aid, 'referenced_by'::text AS relation, 1 AS pr
		      FROM artifact_refs ar
		     WHERE ar.target_kind IN ('page','shard') AND ar.target_id = $1
		    UNION ALL
		    SELECT ar.target_id, 'references', 1
		      FROM artifact_refs ar
		     WHERE ar.artifact_id = $1 AND ar.target_kind IN ('page','shard')
		    UNION ALL
		    SELECT DISTINCT ae2.artifact_id, 'shared_entity', 2
		      FROM artifact_entities ae1
		      JOIN artifact_entities ae2
		        ON ae2.entity_id = ae1.entity_id AND ae2.artifact_id <> ae1.artifact_id
		     WHERE ae1.artifact_id = $1
		),
		ranked AS (
		    SELECT aid, relation,
		           row_number() OVER (PARTITION BY aid ORDER BY pr, relation) AS rn
		      FROM links
		     WHERE aid <> $1
		)
		SELECT a.id, a.title, a.type, ranked.relation
		  FROM ranked
		  JOIN artifacts a ON a.id = ranked.aid
		 WHERE ranked.rn = 1
		 ORDER BY ranked.relation, a.title
		 LIMIT 25
	`, artifactID)
	if err != nil {
		return nil, fmt.Errorf("graph pane linked artifacts: %w", err)
	}
	defer linkRows.Close()
	for linkRows.Next() {
		var la coreservice.GraphLinkedArtifact
		if err := linkRows.Scan(&la.ID, &la.Title, &la.Kind, &la.Relation); err != nil {
			return nil, fmt.Errorf("graph pane linked artifact scan: %w", err)
		}
		pane.LinkedArtifacts = append(pane.LinkedArtifacts, la)
	}
	return pane, linkRows.Err()
}
