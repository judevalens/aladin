package graph

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neighborhood is an entity's local graph: its related entities, the claims about it, and
// the divergences (contradictions) among claims about it and its neighbours. The multi-hop
// shape is why this lives in Neo4j rather than a SQL join.
type Neighborhood struct {
	Entity      NeighborEntity   `json:"entity"`
	Related     []NeighborEntity `json:"relatedEntities"`
	Claims      []NeighborClaim  `json:"claims"`
	Divergences []Divergence     `json:"divergences"`
}

type NeighborEntity struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Weight int64  `json:"weight,omitempty"` // RELATED_TO weight (0 for the focus entity)
}

type NeighborClaim struct {
	ID            string `json:"id"`
	Text          string `json:"text"`
	Polarity      string `json:"polarity"`
	AssertSources int64  `json:"assertSources"`
	DenySources   int64  `json:"denySources"`
}

type Divergence struct {
	FromText string  `json:"fromText"`
	ToText   string  `json:"toText"`
	Basis    string  `json:"basis"`
	Strength float64 `json:"strength"`
}

// Neighbors reads an entity's neighbourhood from Neo4j. limit bounds the related-entity set.
func (p *Projector) Neighbors(ctx context.Context, entityID string, limit int) (*Neighborhood, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	session := p.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	out, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		nb := &Neighborhood{Related: []NeighborEntity{}, Claims: []NeighborClaim{}, Divergences: []Divergence{}}

		// Focus entity.
		res, err := tx.Run(ctx, `MATCH (e:Entity {id:$id}) RETURN e.name AS name, e.kind AS kind`, map[string]any{"id": entityID})
		if err != nil {
			return nil, err
		}
		rec, err := res.Single(ctx)
		if err != nil {
			return nil, fmt.Errorf("entity not in graph: %w", err)
		}
		nb.Entity = NeighborEntity{ID: entityID, Name: str(rec, "name"), Kind: str(rec, "kind")}

		// Related entities (by co-occurrence weight).
		res, err = tx.Run(ctx, `
			MATCH (e:Entity {id:$id})-[r:RELATED_TO]-(o:Entity)
			RETURN o.id AS id, o.name AS name, o.kind AS kind, r.weight AS weight
			ORDER BY r.weight DESC LIMIT $lim
		`, map[string]any{"id": entityID, "lim": limit})
		if err != nil {
			return nil, err
		}
		for res.Next(ctx) {
			r := res.Record()
			nb.Related = append(nb.Related, NeighborEntity{ID: str(r, "id"), Name: str(r, "name"), Kind: str(r, "kind"), Weight: i64(r, "weight")})
		}

		// Claims about the entity.
		res, err = tx.Run(ctx, `
			MATCH (c:Claim)-[:ABOUT]->(:Entity {id:$id})
			RETURN c.id AS id, c.text AS text, c.polarity AS polarity,
			       c.assertSources AS assert, c.denySources AS deny
		`, map[string]any{"id": entityID})
		if err != nil {
			return nil, err
		}
		for res.Next(ctx) {
			r := res.Record()
			nb.Claims = append(nb.Claims, NeighborClaim{
				ID: str(r, "id"), Text: str(r, "text"), Polarity: str(r, "polarity"),
				AssertSources: i64(r, "assert"), DenySources: i64(r, "deny"),
			})
		}

		// Divergences involving claims about this entity.
		res, err = tx.Run(ctx, `
			MATCH (ca:Claim)-[:ABOUT]->(:Entity {id:$id})
			MATCH (ca)-[d:DIVERGES_FROM]-(cb:Claim)
			RETURN ca.text AS fromText, cb.text AS toText, d.basis AS basis, d.strength AS strength
		`, map[string]any{"id": entityID})
		if err != nil {
			return nil, err
		}
		for res.Next(ctx) {
			r := res.Record()
			nb.Divergences = append(nb.Divergences, Divergence{
				FromText: str(r, "fromText"), ToText: str(r, "toText"),
				Basis: str(r, "basis"), Strength: f64(r, "strength"),
			})
		}
		return nb, nil
	})
	if err != nil {
		return nil, fmt.Errorf("graph neighbors: %w", err)
	}
	return out.(*Neighborhood), nil
}

// ClaimCandidate is a claim retrieved by graph proximity to a set of subject entities — a
// candidate for claim-resolution adjudication (the graph channel). Text/Polarity come straight
// off the projected Claim node, so the caller needn't round-trip to Postgres to adjudicate.
type ClaimCandidate struct {
	ID       string
	Text     string
	Polarity string
}

// ClaimCandidates returns claims connected to the given subject entities through the graph:
// claims ABOUT a subject entity, or ABOUT an entity one hop away (a RELATED_TO co-occurrence
// neighbour or a MERGED_INTO alias). Ranked nearer-first (same-subject before neighbour). This
// is the structural half of hybrid claim retrieval — it reaches claims the pgvector channel
// can't (a related entity expressing the same idea in very different words).
func (p *Projector) ClaimCandidates(ctx context.Context, subjectEntityIDs []string, limit int) ([]ClaimCandidate, error) {
	if len(subjectEntityIDs) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	session := p.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	out, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, `
			MATCH (sub:Entity) WHERE sub.id IN $ids
			MATCH path = (sub)-[:RELATED_TO|MERGED_INTO*0..1]-(:Entity)<-[:ABOUT]-(c:Claim)
			WITH c, min(length(path)) AS dist
			RETURN c.id AS id, c.text AS text, c.polarity AS polarity, dist
			ORDER BY dist ASC
			LIMIT $limit
		`, map[string]any{"ids": subjectEntityIDs, "limit": limit})
		if err != nil {
			return nil, err
		}
		cands := []ClaimCandidate{}
		for res.Next(ctx) {
			r := res.Record()
			cands = append(cands, ClaimCandidate{ID: str(r, "id"), Text: str(r, "text"), Polarity: str(r, "polarity")})
		}
		return cands, res.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("graph claim candidates: %w", err)
	}
	return out.([]ClaimCandidate), nil
}

func str(r *neo4j.Record, k string) string {
	if v, ok := r.Get(k); ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
func i64(r *neo4j.Record, k string) int64 {
	if v, ok := r.Get(k); ok && v != nil {
		if n, ok := v.(int64); ok {
			return n
		}
	}
	return 0
}
func f64(r *neo4j.Record, k string) float64 {
	if v, ok := r.Get(k); ok && v != nil {
		if n, ok := v.(float64); ok {
			return n
		}
	}
	return 0
}
