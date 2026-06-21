package graph

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// TestProjector_Project projects a small entity+claim graph into Neo4j and reads it back,
// asserting the nodes, an ABOUT edge, a CONTRADICTS edge, and the DERIVED DIVERGES_FROM.
// Skips unless a Neo4j is reachable (sandbox bolt :7688 by default).
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
	eA, eB := "e-a-"+tag, "e-b-"+tag
	cA, cB := "c-a-"+tag, "c-b-"+tag

	data := GraphData{
		Entities: []EntityNode{{ID: eA, Name: "OpenAI " + tag, Kind: "org"}, {ID: eB, Name: "Anthropic " + tag, Kind: "org"}},
		Claims: []ClaimNode{
			{ID: cA, Text: "A will win", Polarity: "assert", AssertSources: 5, DenySources: 1},
			{ID: cB, Text: "A will not win", Polarity: "assert", AssertSources: 2, DenySources: 4},
		},
		About:      []AboutEdge{{ClaimID: cA, EntityID: eA}, {ClaimID: cB, EntityID: eB}},
		ClaimEdges: []ClaimEdge{{From: cA, To: cB, Type: "contradicts", Confidence: 0.9}},
		Related:    []RelatedEdge{{A: eA, B: eB, Weight: 3}},
	}

	if err := p.Project(ctx, data); err != nil {
		t.Fatalf("Project: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		s := p.driver.NewSession(bg, neo4j.SessionConfig{})
		defer s.Close(bg)
		_, _ = s.Run(bg, `MATCH (n) WHERE n.id IN $ids DETACH DELETE n`,
			map[string]any{"ids": []string{eA, eB, cA, cB}})
	})

	// Read back: the CONTRADICTS edge and the DERIVED DIVERGES_FROM with basis.
	sess := p.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer sess.Close(ctx)
	res, err := sess.Run(ctx, `
		MATCH (a:Claim {id:$ca})-[:CONTRADICTS]->(b:Claim {id:$cb})
		MATCH (a)-[d:DIVERGES_FROM]->(b)
		MATCH (a)-[:ABOUT]->(:Entity {id:$ea})
		RETURN d.basis AS basis, d.strength AS strength
	`, map[string]any{"ca": cA, "cb": cB, "ea": eA})
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	rec, err := res.Single(ctx)
	if err != nil {
		t.Fatalf("expected one DIVERGES_FROM derived from CONTRADICTS: %v", err)
	}
	if basis, _ := rec.Get("basis"); basis != "contradiction" {
		t.Fatalf("DIVERGES_FROM basis = %v, want contradiction", basis)
	}
	if strength, _ := rec.Get("strength"); strength != 0.9 {
		t.Fatalf("DIVERGES_FROM strength = %v, want 0.9", strength)
	}
}

// TestProjector_ClaimCandidates projects A--RELATED_TO--B with a claim about each, then asks for
// candidates around A: it must return the same-subject claim AND the neighbour-entity claim
// (the graph reach pgvector can't do), ranked same-subject first.
func TestProjector_ClaimCandidates(t *testing.T) {
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
	eA, eB := "e-a-"+tag, "e-b-"+tag
	cA, cB := "c-a-"+tag, "c-b-"+tag

	data := GraphData{
		Entities: []EntityNode{{ID: eA, Name: "A " + tag, Kind: "org"}, {ID: eB, Name: "B " + tag, Kind: "org"}},
		Claims: []ClaimNode{
			{ID: cA, Text: "claim about A", Polarity: "assert"},
			{ID: cB, Text: "claim about B", Polarity: "assert"},
		},
		About:   []AboutEdge{{ClaimID: cA, EntityID: eA}, {ClaimID: cB, EntityID: eB}},
		Related: []RelatedEdge{{A: eA, B: eB, Weight: 2}},
	}
	if err := p.Project(ctx, data); err != nil {
		t.Fatalf("Project: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		s := p.driver.NewSession(bg, neo4j.SessionConfig{})
		defer s.Close(bg)
		_, _ = s.Run(bg, `MATCH (n) WHERE n.id IN $ids DETACH DELETE n`,
			map[string]any{"ids": []string{eA, eB, cA, cB}})
	})

	cands, err := p.ClaimCandidates(ctx, []string{eA}, 10)
	if err != nil {
		t.Fatalf("ClaimCandidates: %v", err)
	}
	got := map[string]int{}
	for i, c := range cands {
		got[c.ID] = i
	}
	posA, okA := got[cA]
	posB, okB := got[cB]
	if !okA || !okB {
		t.Fatalf("expected both the same-subject (%s) and the neighbour (%s) claim, got %+v", cA, cB, cands)
	}
	if posA > posB {
		t.Fatalf("same-subject claim must rank before the neighbour claim, got A@%d B@%d", posA, posB)
	}
}
