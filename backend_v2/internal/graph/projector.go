// Package graph projects Aladin's entity + claim layer (Postgres) into Neo4j as the
// connection lens. Nodes are Entity + Claim only (the meaning layer); records/artifacts are
// NOT nodes — they stay in Postgres as provenance. All writes are idempotent MERGEs, so a
// projection can run repeatedly (backfill or incremental) without duplicating.
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
type ClaimNode struct {
	ID            string
	Text          string
	Polarity      string
	AssertSources int
	DenySources   int
}
type AboutEdge struct{ ClaimID, EntityID string }
type ClaimEdge struct {
	From, To, Type string
	Confidence     float64
}
type MergeEdge struct{ From, Into string }
type RelatedEdge struct {
	A, B   string
	Weight int
}

// GraphData is one projection batch (a record's slice, or the whole corpus for backfill).
type GraphData struct {
	Entities   []EntityNode
	Claims     []ClaimNode
	About      []AboutEdge
	ClaimEdges []ClaimEdge
	Merges     []MergeEdge
	Related    []RelatedEdge
}

// claimEdgeRel whitelists the claim-edge type → Neo4j relationship name. The type comes
// from claim_edges.type (CHECK-constrained to these), so building it into the Cypher string
// is safe (no injection) and avoids needing apoc for dynamic relationship types.
var claimEdgeRel = map[string]string{
	"supports":    "SUPPORTS",
	"contradicts": "CONTRADICTS",
	"qualifies":   "QUALIFIES",
}

// Project MERGEs the batch into Neo4j and (re)derives DIVERGES_FROM for the touched claims.
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
			MERGE (c:Claim {id: r.id})
			SET c.text = r.text, c.polarity = r.polarity,
			    c.assertSources = r.assert, c.denySources = r.deny
		`, map[string]any{"rows": claimRows(d.Claims)}); err != nil {
			return nil, err
		}
		if err := run(ctx, tx, `
			UNWIND $rows AS r
			MATCH (c:Claim {id: r.claim}), (e:Entity {id: r.entity})
			MERGE (c)-[:ABOUT]->(e)
		`, map[string]any{"rows": aboutRows(d.About)}); err != nil {
			return nil, err
		}
		// One UNWIND per whitelisted claim-edge type (relationship type can't be parameterized).
		for typ, rel := range claimEdgeRel {
			rows := claimEdgeRows(d.ClaimEdges, typ)
			if len(rows) == 0 {
				continue
			}
			cy := fmt.Sprintf(`
				UNWIND $rows AS r
				MATCH (a:Claim {id: r.from}), (b:Claim {id: r.to})
				MERGE (a)-[e:%s]->(b)
				SET e.confidence = r.confidence
			`, rel)
			if err := run(ctx, tx, cy, map[string]any{"rows": rows}); err != nil {
				return nil, err
			}
		}
		if err := run(ctx, tx, `
			UNWIND $rows AS r
			MATCH (a:Entity {id: r.from}), (b:Entity {id: r.into})
			MERGE (a)-[:MERGED_INTO]->(b)
		`, map[string]any{"rows": mergeRows(d.Merges)}); err != nil {
			return nil, err
		}
		if err := run(ctx, tx, `
			UNWIND $rows AS r
			MATCH (a:Entity {id: r.a}), (b:Entity {id: r.b})
			MERGE (a)-[x:RELATED_TO]->(b)
			SET x.weight = r.weight
		`, map[string]any{"rows": relatedRows(d.Related)}); err != nil {
			return nil, err
		}
		// Derive divergence: a contradiction between two claims IS a divergence. Materialized
		// so reads are cheap and the edge can carry basis/strength; idempotent.
		return nil, run(ctx, tx, `
			MATCH (a:Claim)-[r:CONTRADICTS]->(b:Claim)
			MERGE (a)-[d:DIVERGES_FROM]->(b)
			SET d.basis = 'contradiction', d.strength = coalesce(r.confidence, 0.5)
		`, nil)
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
func claimRows(cs []ClaimNode) []map[string]any {
	out := make([]map[string]any, 0, len(cs))
	for _, c := range cs {
		out = append(out, map[string]any{"id": c.ID, "text": c.Text, "polarity": c.Polarity, "assert": c.AssertSources, "deny": c.DenySources})
	}
	return out
}
func aboutRows(as []AboutEdge) []map[string]any {
	out := make([]map[string]any, 0, len(as))
	for _, a := range as {
		out = append(out, map[string]any{"claim": a.ClaimID, "entity": a.EntityID})
	}
	return out
}
func claimEdgeRows(es []ClaimEdge, typ string) []map[string]any {
	out := make([]map[string]any, 0)
	for _, e := range es {
		if e.Type == typ {
			out = append(out, map[string]any{"from": e.From, "to": e.To, "confidence": e.Confidence})
		}
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
