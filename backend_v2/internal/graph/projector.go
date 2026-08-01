// Package graph projects Aladin's entity layer (Postgres) into Neo4j as the connection
// lens. Nodes are Entity only; records/artifacts are NOT nodes — they stay in Postgres as
// provenance. All writes are idempotent MERGEs, so a projection can run repeatedly (backfill
// or incremental) without duplicating.
//
// The claim layer was removed, so Claim nodes and the ABOUT / SUPPORTS / CONTRADICTS /
// QUALIFIES / DIVERGES_FROM edges are gone with it. What remains is entities, MERGED_INTO,
// and RELATED_TO co-occurrence.
package graph

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Projector struct{ driver neo4j.DriverWithContext }

func NewProjector(uri, user, pass string) (*Projector, error) {
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(user, pass, ""))
	if err != nil {
		return nil, fmt.Errorf("neo4j driver: %w", err)
	}
	return &Projector{driver: driver}, nil
}

func (p *Projector) Close(ctx context.Context) error { return p.driver.Close(ctx) }

func (p *Projector) Ping(ctx context.Context) error { return p.driver.VerifyConnectivity(ctx) }

// --- projection data (read from Postgres, shaped for UNWIND) ---------------

type EntityNode struct {
	ID   string
	Name string
	Kind string
}
type MergeEdge struct{ From, Into string }
type RelatedEdge struct {
	A, B   string
	Weight int
}

// GraphData is one projection batch (a record's slice, or the whole corpus for backfill).
type GraphData struct {
	Entities []EntityNode
	Merges   []MergeEdge
	Related  []RelatedEdge
}

// Project MERGEs the batch into Neo4j.
func (p *Projector) Project(ctx context.Context, d GraphData) error {
	session := p.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		if err := run(ctx, tx, `
			UNWIND $rows AS r
			MERGE (e:Entity {id: r.id})
			SET e.name = r.name, e.kind = r.kind
		`, map[string]any{"rows": entityRows(d.Entities)}); err != nil {
			return nil, err
		}
		if err := run(ctx, tx, `
			UNWIND $rows AS r
			MATCH (a:Entity {id: r.from}), (b:Entity {id: r.into})
			MERGE (a)-[:MERGED_INTO]->(b)
		`, map[string]any{"rows": mergeRows(d.Merges)}); err != nil {
			return nil, err
		}
		return nil, run(ctx, tx, `
			UNWIND $rows AS r
			MATCH (a:Entity {id: r.a}), (b:Entity {id: r.b})
			MERGE (a)-[x:RELATED_TO]->(b)
			SET x.weight = r.weight
		`, map[string]any{"rows": relatedRows(d.Related)})
	})
	if err != nil {
		return fmt.Errorf("graph project: %w", err)
	}
	return nil
}

func run(ctx context.Context, tx neo4j.ManagedTransaction, cypher string, params map[string]any) error {
	_, err := tx.Run(ctx, cypher, params)
	return err
}

func entityRows(es []EntityNode) []map[string]any {
	out := make([]map[string]any, 0, len(es))
	for _, e := range es {
		out = append(out, map[string]any{"id": e.ID, "name": e.Name, "kind": e.Kind})
	}
	return out
}
func mergeRows(ms []MergeEdge) []map[string]any {
	out := make([]map[string]any, 0, len(ms))
	for _, m := range ms {
		out = append(out, map[string]any{"from": m.From, "into": m.Into})
	}
	return out
}
func relatedRows(rs []RelatedEdge) []map[string]any {
	out := make([]map[string]any, 0, len(rs))
	for _, r := range rs {
		out = append(out, map[string]any{"a": r.A, "b": r.B, "weight": r.Weight})
	}
	return out
}
