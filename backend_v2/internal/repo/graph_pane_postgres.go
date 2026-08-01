package repo

import (
	"context"
	"fmt"

	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresGraphPaneRepository struct{ pool *pgxpool.Pool }

func NewGraphPanePostgres(pool *pgxpool.Pool) *PostgresGraphPaneRepository {
	return &PostgresGraphPaneRepository{pool: pool}
}

// ForArtifact assembles the "On the graph" pane for the artifact you're viewing: the
// entities connected to it (tagged or @mentioned, with mention counts) and the other
// artifacts it connects to. An empty pane means nothing is linked to the page yet.
func (r *PostgresGraphPaneRepository) ForArtifact(ctx context.Context, artifactID string) (*coreservice.GraphPane, error) {
	pane := &coreservice.GraphPane{
		Entities:        []coreservice.GraphEntity{},
		LinkedArtifacts: []coreservice.GraphLinkedArtifact{},
	}

	// Entities connected to this artifact: tags and projected @mentions from
	// artifact_entities, with origin precedence tag > mention, plus how many times each
	// entity is mentioned anywhere.
	entRows, err := r.pool.Query(ctx, `
		WITH agg AS (
		    SELECT ae.entity_id AS eid,
		           CASE WHEN bool_or(ae.origin = 'tag') THEN 'tag' ELSE 'mention' END AS origin
		      FROM artifact_entities ae
		     WHERE ae.artifact_id = $1
		     GROUP BY ae.entity_id
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
