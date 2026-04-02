package graph

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"aladin/backend_v2/internal/db"
)

// skipTypes are containers that don't need graph nodes.
var skipTypes = map[string]bool{"feed": true}

// Promoter writes enriched artifacts into Neo4j.
// Uses MERGE everywhere — safe to call multiple times for the same artifact.
type Promoter struct {
	driver neo4j.DriverWithContext
}

func NewPromoter(uri, user, pass string) (*Promoter, error) {
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(user, pass, ""))
	if err != nil {
		return nil, fmt.Errorf("neo4j driver: %w", err)
	}
	return &Promoter{driver: driver}, nil
}

func (p *Promoter) Close(ctx context.Context) {
	p.driver.Close(ctx)
}

func (p *Promoter) Promote(ctx context.Context, a *db.EmbeddedArtifact) error {
	if skipTypes[a.Type] {
		return nil
	}

	session := p.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// Upsert artifact node
		if _, err := tx.Run(ctx, `
			MERGE (a:Artifact {id: $id})
			SET a.label    = $label,
			    a.type     = $type,
			    a.sourceUrl = $sourceUrl,
			    a.summary  = $summary
		`, map[string]any{
			"id":        a.ID,
			"label":     a.Label,
			"type":      a.Type,
			"sourceUrl": a.SourceURL,
			"summary":   stringFromEnrichment(a.Enrichment, "summary"),
		}); err != nil {
			return nil, err
		}

		// Entity nodes + MENTIONS edges
		for _, entity := range stringsFromEnrichment(a.Enrichment, "entities") {
			if entity == "" {
				continue
			}
			if _, err := tx.Run(ctx, `
				MERGE (e:Entity {name: $name})
				WITH e
				MATCH (a:Artifact {id: $aid})
				MERGE (a)-[:MENTIONS]->(e)
			`, map[string]any{"name": entity, "aid": a.ID}); err != nil {
				return nil, err
			}
		}

		// Topic nodes + TAGGED_WITH edges
		for _, topic := range stringsFromEnrichment(a.Enrichment, "topics") {
			if topic == "" {
				continue
			}
			if _, err := tx.Run(ctx, `
				MERGE (t:Topic {name: $name})
				WITH t
				MATCH (a:Artifact {id: $aid})
				MERGE (a)-[:TAGGED_WITH]->(t)
			`, map[string]any{"name": topic, "aid": a.ID}); err != nil {
				return nil, err
			}
		}

		return nil, nil
	})
	if err != nil {
		slog.Error("graph: promote failed",
			"component", "graph",
			"artifact_id", a.ID,
			"err", err,
		)
		return fmt.Errorf("neo4j promote: %w", err)
	}

	slog.Debug("graph: promoted artifact",
		"component", "graph",
		"artifact_id", a.ID,
		"entity_count", len(stringsFromEnrichment(a.Enrichment, "entities")),
		"topic_count", len(stringsFromEnrichment(a.Enrichment, "topics")),
	)
	return nil
}

func stringFromEnrichment(e map[string]any, key string) string {
	v, _ := e[key].(string)
	return v
}

func stringsFromEnrichment(e map[string]any, key string) []string {
	raw, ok := e[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
