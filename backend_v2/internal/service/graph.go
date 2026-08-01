package service

import (
	"context"

	"aladin/backend_v2/internal/graph"
)

// GraphReader reads the Neo4j connection lens (an entity's neighbourhood of co-occurring
// entities). Nil in deps when Neo4j isn't configured — handlers must check.
type GraphReader interface {
	Neighbors(ctx context.Context, entityID string, limit int) (*graph.Neighborhood, error)
}
