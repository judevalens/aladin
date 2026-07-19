package service

import (
	"context"
	"errors"
	"testing"
)

type fakeSearchProvider struct {
	section string
	hits    []SearchHit
	err     error
}

func (f fakeSearchProvider) Section() string { return f.section }
func (f fakeSearchProvider) Search(context.Context, string, string, int) ([]SearchHit, error) {
	return f.hits, f.err
}

func TestSearchServiceGroupsOrdersAndDegrades(t *testing.T) {
	svc := NewSearchService(
		// entity section, provider A (tickers) — registered first, so leads its section.
		fakeSearchProvider{section: SearchSectionEntity, hits: []SearchHit{{Kind: "ticker", ID: "1", Title: "NVDA"}}},
		// a failing provider must NOT fail the whole search.
		fakeSearchProvider{section: SearchSectionEntity, err: errors.New("boom")},
		// entity section, provider B (entities) — appends after tickers.
		fakeSearchProvider{section: SearchSectionEntity, hits: []SearchHit{{Kind: "company", ID: "2", Title: "NVIDIA Corporation"}}},
		// artifact section.
		fakeSearchProvider{section: SearchSectionArtifact, hits: []SearchHit{{Kind: "page", ID: "3", Title: "My thesis"}}},
	)

	resp, err := svc.Search(context.Background(), "user", "nvidia", 10)
	if err != nil {
		t.Fatalf("search errored despite a degrading provider: %v", err)
	}
	if len(resp.Sections) != 2 {
		t.Fatalf("want 2 sections, got %d (%+v)", len(resp.Sections), resp.Sections)
	}

	// Section priority: entities before artifacts.
	if resp.Sections[0].Type != SearchSectionEntity || resp.Sections[1].Type != SearchSectionArtifact {
		t.Fatalf("section order wrong: %s then %s", resp.Sections[0].Type, resp.Sections[1].Type)
	}

	// Entities section merges both providers, tickers first (registration order).
	ent := resp.Sections[0].Hits
	if len(ent) != 2 || ent[0].Kind != "ticker" || ent[1].Kind != "company" {
		t.Fatalf("entity section wrong: %+v", ent)
	}
	if resp.Sections[1].Hits[0].Kind != "page" {
		t.Fatalf("artifact section wrong: %+v", resp.Sections[1].Hits)
	}

	// Empty query → no sections, no request.
	empty, err := svc.Search(context.Background(), "user", "  ", 10)
	if err != nil || len(empty.Sections) != 0 {
		t.Fatalf("empty query should return no sections: %+v (%v)", empty.Sections, err)
	}
}
