package graph

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neighborhood is an entity's local graph: the entities it co-occurs with. The multi-hop
// shape is why this lives in Neo4j rather than a SQL join.
type Neighborhood struct {
	Entity  NeighborEntity   `json:"entity"`
	Related []NeighborEntity `json:"relatedEntities"`
}

type NeighborEntity struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Weight int64  `json:"weight,omitempty"` // RELATED_TO weight (0 for the focus entity)
}

// Neighbors reads an entity's neighbourhood from Neo4j. limit bounds the related-entity set.
func (p *Projector) Neighbors(ctx context.Context, entityID string, limit int) (*Neighborhood, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	session := p.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	out, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		nb := &Neighborhood{Related: []NeighborEntity{}}

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

		return nb, nil
	})
	if err != nil {
		return nil, fmt.Errorf("graph neighbors: %w", err)
	}
	return out.(*Neighborhood), nil
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
