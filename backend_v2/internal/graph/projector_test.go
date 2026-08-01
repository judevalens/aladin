package graph

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// TestProjector_Project projects a small entity graph into Neo4j and reads it back,
// asserting the nodes plus the RELATED_TO and MERGED_INTO edges. (The claim layer was
// removed, so Claim nodes and their edges are gone.) Skips unless a Neo4j is reachable
// (sandbox bolt :7688 by default).
func TestProjector_Project(t *testing.T) {
	uri := os.Getenv("NEO4J_TEST_URI")
	if uri == "" {
		uri = "bolt://localhost:7688"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	p, err := NewProjector(uri, "neo4j", "password")
	if err != nil {
		t.Skipf("no neo4j driver: %v", err)
	}
	defer p.Close(ctx)
	if err := p.Ping(ctx); err != nil {
		t.Skipf("neo4j unreachable at %s: %v", uri, err)
	}

	tag := uuid.NewString()[:8]
	eA, eB, eC := "e-a-"+tag, "e-b-"+tag, "e-c-"+tag

	data := GraphData{
		Entities: []EntityNode{
			{ID: eA, Name: "OpenAI " + tag, Kind: "org"},
			{ID: eB, Name: "Anthropic " + tag, Kind: "org"},
			{ID: eC, Name: "Anthropic PBC " + tag, Kind: "org"},
		},
		Merges:  []MergeEdge{{From: eC, Into: eB}},
		Related: []RelatedEdge{{A: eA, B: eB, Weight: 3}},
	}

	if err := p.Project(ctx, data); err != nil {
		t.Fatalf("Project: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		s := p.driver.NewSession(bg, neo4j.SessionConfig{})
		defer s.Close(bg)
		_, _ = s.Run(bg, `MATCH (n) WHERE n.id IN $ids DETACH DELETE n`,
			map[string]any{"ids": []string{eA, eB, eC}})
	})

	// Read back: the RELATED_TO weight and the MERGED_INTO edge.
	sess := p.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer sess.Close(ctx)
	res, err := sess.Run(ctx, `
		MATCH (a:Entity {id:$ea})-[r:RELATED_TO]-(b:Entity {id:$eb})
		MATCH (c:Entity {id:$ec})-[:MERGED_INTO]->(b)
		RETURN r.weight AS weight, b.name AS name
	`, map[string]any{"ea": eA, "eb": eB, "ec": eC})
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	rec, err := res.Single(ctx)
	if err != nil {
		t.Fatalf("expected RELATED_TO + MERGED_INTO: %v", err)
	}
	if weight, _ := rec.Get("weight"); weight != int64(3) {
		t.Fatalf("RELATED_TO weight = %v, want 3", weight)
	}
	if name, _ := rec.Get("name"); name != "Anthropic "+tag {
		t.Fatalf("merge target name = %v", name)
	}
}
