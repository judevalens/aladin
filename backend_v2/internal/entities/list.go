package entities

import (
	"context"
	"strings"
)

// The Entities index: browse the entity layer. Until now entities were only reachable
// through a page's tag bar or the `@` picker — the registry itself was invisible, and
// the search path returns nothing for an empty query (it's a typeahead). This is the
// front door.

// Entity list sort orders.
const (
	EntitySortAttention = "attention" // decisions the system owes you, most first
	EntitySortLinks     = "links"
	EntitySortName      = "name"
)

// Entity list status filters — the header pills. "" = everything.
const (
	EntityFilterPending    = "pending"    // has an unanswered merge proposal (attention > 0)
	EntityFilterUnresolved = "unresolved" // trust_tier = 'placeholder'
)

// EntityListItem is one card on the index.
type EntityListItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Gist      string `json:"gist"`
	TrustTier string `json:"trustTier"`
	// UpdatedAt drives the card's "updated Xd ago" line (RFC3339; rendered relative).
	UpdatedAt string `json:"updatedAt"`
	// Links / Sources are what the entity is wired to and what it was seen in.
	Links   int `json:"links"`
	Sources int `json:"sources"`
	// Attention is the count of merge proposals still awaiting a decision on this
	// entity — the questions the judge is asking you about it.
	Attention int      `json:"attention"`
	Aliases   []string `json:"aliases"`
}

// EntitySummary is the header band: the shape of the whole visible registry, NOT of the
// filtered result (a filter shouldn't make the layer look smaller than it is).
type EntitySummary struct {
	Total int `json:"total"`
	// PendingDecisions is every unanswered merge proposal — "answers at stake".
	PendingDecisions int            `json:"pendingDecisions"`
	Tiers            map[string]int `json:"tiers"`
}

type EntityListQuery struct {
	OwnerUserID string
	Query       string
	Kind        string
	// Filter narrows to a status pill (pending | unresolved); "" is everything.
	Filter string
	Sort   string
	Limit  int
}

type EntityListResult struct {
	Entities []EntityListItem `json:"entities"`
	Summary  EntitySummary    `json:"summary"`
}

type EntityListService interface {
	List(ctx context.Context, q EntityListQuery) (EntityListResult, error)
}

type EntityListRepository interface {
	List(ctx context.Context, q EntityListQuery) ([]EntityListItem, error)
	Summary(ctx context.Context, ownerUserID string) (EntitySummary, error)
}

const (
	entityListDefaultLimit = 60
	entityListMaxLimit     = 100
)

var validEntitySorts = map[string]bool{
	EntitySortAttention: true, EntitySortLinks: true, EntitySortName: true,
}

type DefaultEntityListService struct {
	repo EntityListRepository
}

func NewEntityListService(repo EntityListRepository) *DefaultEntityListService {
	return &DefaultEntityListService{repo: repo}
}

func (s *DefaultEntityListService) List(ctx context.Context, q EntityListQuery) (EntityListResult, error) {
	q.Query = strings.TrimSpace(q.Query)
	q.Kind = strings.TrimSpace(q.Kind)
	q.Filter = strings.TrimSpace(q.Filter)
	if q.Filter != EntityFilterPending && q.Filter != EntityFilterUnresolved {
		q.Filter = ""
	}
	if !validEntitySorts[q.Sort] {
		q.Sort = EntitySortAttention
	}
	if q.Limit <= 0 || q.Limit > entityListMaxLimit {
		q.Limit = entityListDefaultLimit
	}

	items, err := s.repo.List(ctx, q)
	if err != nil {
		return EntityListResult{}, err
	}
	summary, err := s.repo.Summary(ctx, q.OwnerUserID)
	if err != nil {
		return EntityListResult{}, err
	}
	if items == nil {
		items = []EntityListItem{}
	}
	if summary.Tiers == nil {
		summary.Tiers = map[string]int{}
	}
	return EntityListResult{Entities: items, Summary: summary}, nil
}
