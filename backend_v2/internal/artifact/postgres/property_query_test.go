package postgres

import (
	"context"
	"testing"
	"time"

	"aladin/backend_v2/internal/artifact"
)

// TestArtifactPropertyQuery covers the H1c read: artifacts filtered by a typed property
// (metadata->'properties', a JSON ARRAY of {key,type,value}) plus the facet list a filter UI needs.
// Properties are matched by jsonb containment, which is what the 00035 GIN index serves.
func TestArtifactPropertyQuery(t *testing.T) {
	ctx := adminContext(testAdminUserID)
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	pool := mustTestPool(ctxTO, t)
	defer pool.Close()
	cleanupSyncTables(ctxTO, t, pool)
	seedUser(ctxTO, t, pool, testAdminUserID)

	r := NewArtifactsPostgres(pool)

	// Three notes: two with Status (Live / Draft), one with only an unrelated property.
	pos := int64(0)
	mk := func(id, title string, props []map[string]string) {
		t.Helper()
		pos++
		anyProps := make([]any, 0, len(props))
		for _, p := range props {
			anyProps = append(anyProps, p)
		}
		now := time.Now().UTC().Format(time.RFC3339)
		rec := artifact.ArtifactResponse{
			ID: id, Type: "note", Title: title,
			Metadata:  map[string]any{"properties": anyProps},
			CreatedAt: now, UpdatedAt: now,
		}
		node := artifact.TreeNodeRecord{ID: id, Kind: "artifact", ArtifactID: &id, Position: pos}
		if err := r.CreateArtifactGraph(ctx, rec, node, nil, ""); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}
	live := tid("prop-live")
	draft := tid("prop-draft")
	other := tid("prop-other")
	mk(live, "Live note", []map[string]string{{"key": "Status", "type": "select", "value": "Live"}})
	mk(draft, "Draft note", []map[string]string{{"key": "Status", "type": "select", "value": "Draft"}})
	mk(other, "Other note", []map[string]string{{"key": "Ticker", "type": "text", "value": "NVDA"}})

	ids := func(recs []artifact.ArtifactResponse) map[string]bool {
		out := map[string]bool{}
		for _, a := range recs {
			out[a.ID] = true
		}
		return out
	}

	// key+value → containment match, exactly one hit.
	got, err := r.QueryArtifactsByProperty(ctx, artifact.PropertyQuery{Key: "Status", Value: "Live", Limit: 50})
	if err != nil {
		t.Fatalf("query Status=Live: %v", err)
	}
	found := ids(got)
	if !found[live] || found[draft] || found[other] {
		t.Fatalf("Status=Live matched %v, want only %s", found, live)
	}

	// key-only → every artifact carrying the key, regardless of value.
	got, err = r.QueryArtifactsByProperty(ctx, artifact.PropertyQuery{Key: "Status", Limit: 50})
	if err != nil {
		t.Fatalf("query Status(any): %v", err)
	}
	found = ids(got)
	if !found[live] || !found[draft] {
		t.Fatalf("Status(any) matched %v, want both %s and %s", found, live, draft)
	}
	if found[other] {
		t.Fatalf("Status(any) must not match the artifact without that key")
	}

	// A value that nobody has.
	got, err = r.QueryArtifactsByProperty(ctx, artifact.PropertyQuery{Key: "Status", Value: "Archived", Limit: 50})
	if err != nil {
		t.Fatalf("query Status=Archived: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Status=Archived matched %d artifacts, want 0", len(got))
	}

	// Facets: the keys in use, each with its distinct values.
	facets, err := r.PropertyFacets(ctx)
	if err != nil {
		t.Fatalf("facets: %v", err)
	}
	byKey := map[string][]string{}
	for _, f := range facets {
		byKey[f.Key] = f.Values
	}
	status, ok := byKey["Status"]
	if !ok {
		t.Fatalf("facets missing the Status key: %+v", facets)
	}
	seen := map[string]bool{}
	for _, v := range status {
		seen[v] = true
	}
	if !seen["Live"] || !seen["Draft"] {
		t.Fatalf("Status facet values = %v, want Live and Draft", status)
	}
	if _, ok := byKey["Ticker"]; !ok {
		t.Fatalf("facets missing the Ticker key: %+v", facets)
	}
}
