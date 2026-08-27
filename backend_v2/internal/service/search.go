package service

import (
	"context"
	"log/slog"
	"strings"
	"sync"
)

// Global search — one federating endpoint behind the ⌘K command box. Federation, section
// grouping, and ordering live here (server-side); the client is a dumb renderer. Generalizes
// the ArtifactRefService.Search pattern (which already federates pages + shards) across every
// searchable source. See ~/.claude/plans/whimsical-squishing-conway.md.

// Section keys. Deliberately COARSE — two sections. Tickers, companies, and people are all
// entities (a security is an entity with a ticker detail view), so they share one section.
const (
	SearchSectionEntity   = "entity"
	SearchSectionArtifact = "artifact"
)

// searchSectionOrder is the backend-owned section priority + labels.
var searchSectionOrder = []struct{ key, label string }{
	{SearchSectionEntity, "Entities"},
	{SearchSectionArtifact, "Artifacts"},
}

const (
	searchDefaultPerProvider = 6
	searchMaxPerProvider     = 12
)

// SearchHit is one result row. Kind is the fine row type (drives the client's icon + route);
// the section it lands in is decided by its provider, not by Kind.
type SearchHit struct {
	Kind     string  `json:"kind"` // ticker | company | person | entity | page | shard | file | board | link
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Subtitle string  `json:"subtitle,omitempty"`
	// Locator points into the artifact ("page:12", "shape:<id>") when the hit came from
	// content rather than a title — the citable deep-open (READABLE_WORKSPACE L3).
	Locator  string  `json:"locator,omitempty"`
	Score    float64 `json:"score"`
}

// SearchSection is one typed group of hits, in backend priority order.
type SearchSection struct {
	Type  string      `json:"type"` // entity | artifact
	Label string      `json:"label"`
	Hits  []SearchHit `json:"hits"`
}

// SearchResponse is the whole global-search payload.
type SearchResponse struct {
	Sections []SearchSection `json:"sections"`
}

// SearchProvider federates one source. Section() declares which section its hits join, so a
// new source is one provider with zero client change.
type SearchProvider interface {
	Section() string
	Search(ctx context.Context, userID, query string, limit int) ([]SearchHit, error)
}

// SearchService runs every provider and assembles the sectioned response.
type SearchService interface {
	Search(ctx context.Context, userID, query string, limit int) (SearchResponse, error)
}

type defaultSearchService struct {
	providers []SearchProvider
}

// NewSearchService registers providers in order; within a section, earlier providers' hits
// lead (so tickers precede other entities).
func NewSearchService(providers ...SearchProvider) SearchService {
	return &defaultSearchService{providers: providers}
}

func (s *defaultSearchService) Search(ctx context.Context, userID, query string, limit int) (SearchResponse, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return SearchResponse{Sections: []SearchSection{}}, nil
	}
	if limit <= 0 || limit > searchMaxPerProvider {
		limit = searchDefaultPerProvider
	}

	// Fan out to every provider in parallel. A failing provider degrades to no hits — it
	// never fails the whole search.
	results := make([][]SearchHit, len(s.providers))
	var wg sync.WaitGroup
	for i, p := range s.providers {
		wg.Add(1)
		go func(i int, p SearchProvider) {
			defer wg.Done()
			hits, err := p.Search(ctx, userID, query, limit)
			if err != nil {
				slog.Warn("search: provider failed", "component", "search", "section", p.Section(), "err", err)
				return
			}
			results[i] = hits
		}(i, p)
	}
	wg.Wait()

	// Assemble sections in fixed order; within a section, concatenate providers in
	// registration order (preserving each provider's own ranking).
	sections := make([]SearchSection, 0, len(searchSectionOrder))
	for _, sec := range searchSectionOrder {
		var hits []SearchHit
		seen := map[string]bool{}
		for i, p := range s.providers {
			if p.Section() == sec.key {
				for _, hit := range results[i] {
					// One artifact can match on title AND body (two providers) — keep
					// the earlier provider's row, which carries the higher-priority form.
					if seen[hit.ID] {
						continue
					}
					seen[hit.ID] = true
					hits = append(hits, hit)
				}
			}
		}
		if len(hits) > 0 {
			sections = append(sections, SearchSection{Type: sec.key, Label: sec.label, Hits: hits})
		}
	}
	return SearchResponse{Sections: sections}, nil
}

// scoreAt turns a provider's 0-based rank into a descending score, preserving its order.
func scoreAt(index int) float64 { return 1.0 - float64(index)*0.001 }

// ── providers: each wraps an existing search service (one source of truth) ──

// InstrumentSearchProvider federates ticker search into the Entities section.
type InstrumentSearchProvider struct{ svc InstrumentService }

func NewInstrumentSearchProvider(svc InstrumentService) InstrumentSearchProvider {
	return InstrumentSearchProvider{svc: svc}
}
func (p InstrumentSearchProvider) Section() string { return SearchSectionEntity }
func (p InstrumentSearchProvider) Search(ctx context.Context, _ /*userID*/, query string, limit int) ([]SearchHit, error) {
	hits, err := p.svc.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]SearchHit, 0, len(hits))
	for i, h := range hits {
		out = append(out, SearchHit{Kind: "ticker", ID: h.ID, Title: h.Symbol, Subtitle: h.Name, Score: scoreAt(i)})
	}
	return out, nil
}

// EntitySearchProvider federates entity search into the Entities section; company/person keep
// their kind, everything else collapses to a generic "entity".
type EntitySearchProvider struct{ svc EntityTagService }

func NewEntitySearchProvider(svc EntityTagService) EntitySearchProvider {
	return EntitySearchProvider{svc: svc}
}
func (p EntitySearchProvider) Section() string { return SearchSectionEntity }
func (p EntitySearchProvider) Search(ctx context.Context, userID, query string, limit int) ([]SearchHit, error) {
	hits, err := p.svc.Search(ctx, userID, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]SearchHit, 0, len(hits))
	for i, h := range hits {
		kind := h.Kind
		if kind != "company" && kind != "person" {
			kind = "entity"
		}
		out = append(out, SearchHit{Kind: kind, ID: h.ID, Title: h.Name, Score: scoreAt(i)})
	}
	return out, nil
}

// ContentSearchProvider federates the content index into the Artifacts section — body
// hits across every readable kind (pages, files, boards, links), each carrying the
// locator that makes it citable. Registered AFTER ArtifactSearchProvider so title
// matches outrank body matches and dedupe keeps the title row.
type ContentSearchProvider struct{ svc ContentIndexService }

func NewContentSearchProvider(svc ContentIndexService) ContentSearchProvider {
	return ContentSearchProvider{svc: svc}
}
func (p ContentSearchProvider) Section() string { return SearchSectionArtifact }
func (p ContentSearchProvider) Search(ctx context.Context, userID, query string, limit int) ([]SearchHit, error) {
	hits, err := p.svc.Search(ctx, userID, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]SearchHit, 0, len(hits))
	for i, h := range hits {
		kind := h.Kind
		if kind == "app" {
			kind = "shard" // the client's route vocabulary (ArtifactRef does the same)
		}
		out = append(out, SearchHit{
			Kind: kind, ID: h.ArtifactID, Title: h.Title,
			Subtitle: h.Snippet, Locator: h.Locator, Score: scoreAt(i),
		})
	}
	return out, nil
}

// ArtifactSearchProvider federates page + shard TITLE search into the Artifacts section
// (ArtifactRefService is the `#`-picker's source of truth and stays scoped to pages +
// shards; body hits across all kinds come from ContentSearchProvider above).
type ArtifactSearchProvider struct{ svc ArtifactRefService }

func NewArtifactSearchProvider(svc ArtifactRefService) ArtifactSearchProvider {
	return ArtifactSearchProvider{svc: svc}
}
func (p ArtifactSearchProvider) Section() string { return SearchSectionArtifact }
func (p ArtifactSearchProvider) Search(ctx context.Context, userID, query string, limit int) ([]SearchHit, error) {
	hits, err := p.svc.Search(ctx, userID, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]SearchHit, 0, len(hits))
	for i, h := range hits {
		out = append(out, SearchHit{Kind: h.Kind, ID: h.ID, Title: h.Label, Subtitle: h.Detail, Score: scoreAt(i)})
	}
	return out, nil
}
