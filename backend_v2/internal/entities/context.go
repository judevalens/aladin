package entities

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// The Entity Context surface (design/ENTITY_CONTEXT_PRD.md): the detail view for ONE
// entity, relationship-first. Two product invariants shape this file:
//
//   - Relationships are the understanding. Edges are typed, directional, and carry the
//     *why* — they are not a backlinks list.
//   - Context is never summarized. Bodies are the user's / source's own words, passed
//     through verbatim. This layer routes and connects material; it must not rewrite it.

// Entity edge relation types (the PRD's 7). Stored in relationships.rel_type; snake_case
// to match the column's existing vocabulary ('derived_from'). `supports` / `contradicts`
// are shared with the 00006 artifact/record bridge — same meaning, entity endpoints.
const (
	RelEnables     = "enables"
	RelEnabledBy   = "enabled_by"
	RelSupports    = "supports"
	RelContradicts = "contradicts"
	RelPartOf      = "part_of"
	RelInstance    = "instance"
	RelCompetes    = "competes"
)

// EntityEdgeKind is the relationships endpoint kind for an entity.
const EntityEdgeKind = "entity"

// Edge origin — drives the surface's YOURS / FOUND tag.
const (
	EdgeOriginYou     = "you"
	EdgeOriginWatcher = "watcher"
)

var validEntityRelTypes = map[string]bool{
	RelEnables: true, RelEnabledBy: true, RelSupports: true, RelContradicts: true,
	RelPartOf: true, RelInstance: true, RelCompetes: true,
}

// ValidEntityRelType reports whether rel is one of the surface's typed relations.
func ValidEntityRelType(rel string) bool { return validEntityRelTypes[rel] }

// EntityIdentity is the focused node: what this entity is.
type EntityIdentity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	// Gist is a one-sentence definition — the ONLY prose this page authors.
	Gist string `json:"gist"`
	// Watchers are the sources monitoring this entity. Always empty for now (streams are
	// global, not per-entity); the surface hides the chip row when empty.
	Watchers []string `json:"watchers"`
	// Since is the provenance line, e.g. "tracked 3 weeks · 14 pieces of context".
	Since string `json:"since"`
	// Placeholder marks an entity the @ picker minted that background resolution hasn't
	// settled yet (P1.2) — the surface can show it as unresolved rather than pretending.
	Placeholder bool `json:"placeholder"`
}

// EntityEdge is one typed, directional relationship to another entity, as seen FROM the
// focused entity (see EntityContextRepository.EdgesFor on direction flipping).
type EntityEdge struct {
	Rel string `json:"rel"`
	// ToID is the target entity — the surface navigates on it ("pull the thread").
	ToID   string `json:"toId"`
	To     string `json:"to"`
	Kind   string `json:"kind"`
	Why    string `json:"why"`
	Origin string `json:"origin"`
}

// EntityContextItem is one piece of accreted material. Body is VERBATIM.
type EntityContextItem struct {
	Type string `json:"type"` // quote | note | question
	Body string `json:"body"`
	Who  string `json:"who"`  // @handle, publication, r/sub, or "me"
	Time string `json:"time"` // relative, e.g. "2h"
	// Platform is "" when unknown → the surface hides the chip.
	Platform string `json:"platform"`
	// SourceID lets a note link back to the page it came from ("" for quotes).
	SourceID string `json:"sourceId,omitempty"`
}

// Merge suggestions — what the judge thinks, rendered as "Same, or just similar?".
const (
	MergeSuggestSynonym  = "synonym"  // judge said same → folding them is proposed
	MergeSuggestDistinct = "distinct" // judge said different
	MergeSuggestUnsure   = "unsure"   // judge abstained, or hasn't run yet
)

// PendingMerge is one unanswered question the judge is asking about this entity: is this
// other entity the same thing? Every field is evidence FOR THE HUMAN to decide on — the
// system proposes, it does not quietly fold identities together.
type PendingMerge struct {
	MergeID   string `json:"mergeId"`
	OtherID   string `json:"otherId"`
	OtherName string `json:"otherName"`
	OtherKind string `json:"otherKind"`
	// Confidence is the lexical/vector similarity that surfaced the pair (0–1).
	Confidence float64 `json:"confidence"`
	Suggestion string  `json:"suggestion"`
	// Why is a plain-language reading of the evidence, e.g. "name overlap 78%".
	Why string `json:"why"`
	// Reason is the judge's own words when it gave any.
	Reason string `json:"reason,omitempty"`
}

// MergeQueueItem is one pending decision on the Entities inbox — a proposal to fold two
// entities together, shown as a pair so it can be decided without opening either.
type MergeQueueItem struct {
	MergeID    string  `json:"mergeId"`
	FromID     string  `json:"fromId"`
	FromName   string  `json:"fromName"`
	FromKind   string  `json:"fromKind"`
	IntoID     string  `json:"intoId"`
	IntoName   string  `json:"intoName"`
	IntoKind   string  `json:"intoKind"`
	Confidence float64 `json:"confidence"`
	Suggestion string  `json:"suggestion"` // synonym | distinct | unsure
	Why        string  `json:"why"`
}

// EntityDataPoint is one typed attribute from the entity's data-point map
// ({name,type,value|id}). Scalars carry Value; a `reference` type carries RefID (+ a cached
// Label for instant render) — the fast-follow. Read-only on this surface for now.
type EntityDataPoint struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value,omitempty"`
	RefID string `json:"refId,omitempty"`
	Label string `json:"label,omitempty"`
}

// EntityExternalID is one hard cross-system identity key (CIK/LEI/CUSIP/…).
type EntityExternalID struct {
	System string `json:"system"`
	Value  string `json:"value"`
}

// CompanyFacts is the kind='company' extension row — objective, rankable facts as real
// columns. Nil on the payload for any non-company entity.
type CompanyFacts struct {
	Sector      string `json:"sector,omitempty"`
	Industry    string `json:"industry,omitempty"`
	Description string `json:"description,omitempty"`
	Website     string `json:"website,omitempty"`
	Country     string `json:"country,omitempty"`
	Employees   int    `json:"employees,omitempty"`
	FoundedYear int    `json:"foundedYear,omitempty"`
}

// EntityContext is the whole surface payload for one entity.
type EntityContext struct {
	Entity  EntityIdentity      `json:"entity"`
	Edges   []EntityEdge        `json:"edges"`
	Context []EntityContextItem `json:"context"`
	// Merges are the judge's open questions about this entity's identity.
	Merges []PendingMerge `json:"merges"`
	// DataPoints are the entity's typed attributes; ExternalIds its hard identity keys.
	DataPoints  []EntityDataPoint  `json:"dataPoints"`
	ExternalIds []EntityExternalID `json:"externalIds"`
	// Company is the extension row for kind='company'; nil otherwise.
	Company *CompanyFacts `json:"company,omitempty"`
}

// DrawEdgeInput is the typed, provenance-carrying edge write (PRD §4.2/§4.5): who
// asserted it, between which entities, and why. Never a silent mutation.
type DrawEdgeInput struct {
	OwnerUserID string
	FromID      string
	ToID        string
	Rel         string
	Why         string
}

var (
	ErrEntityNotFound = errors.New("entity not found")
	// ErrMergeNotPending: the proposal was already decided (or never existed). Surfaces
	// as a 409 so a double-click reads as "someone got there first", not a 500.
	ErrMergeNotPending = errors.New("merge is not pending")
)

// EntityContextService assembles the Entity Context surface, accepts edge writes, and
// takes the user's decisions on the judge's merge proposals.
type EntityContextService interface {
	// Get returns identity + edges + context + pending merges for one entity. The id is
	// resolved through the merge overlay first, so a link to a merged-away entity lands
	// on its canonical root rather than 404ing or showing an orphan.
	Get(ctx context.Context, ownerUserID, entityID string) (EntityContext, error)
	// DrawEdge writes a user-authored edge (origin 'you'). Idempotent per
	// (user, from, to, rel).
	DrawEdge(ctx context.Context, in DrawEdgeInput) error
	// MergeQueue is the global inbox: every unanswered merge proposal, newest-confident
	// first — the decisions the judge is waiting on you for.
	MergeQueue(ctx context.Context, limit int) ([]MergeQueueItem, error)
	// AcceptMerge folds the two entities together (reversibly — ids stay resolvable).
	AcceptMerge(ctx context.Context, mergeID string) error
	// RejectMerge records that they are distinct. The row stays as negative evidence, so
	// the pair is never proposed again.
	RejectMerge(ctx context.Context, mergeID string) error
}

// MergeStore is the narrow slice of the entity repository this service needs to act on
// merge decisions — satisfied structurally by db.EntityRepository, so the service package
// stays free of a db import cycle (same pattern as entities.JudgeStore).
type MergeStore interface {
	AcceptMerge(ctx context.Context, mergeID string) error
	RejectMerge(ctx context.Context, mergeID string) error
}

type EntityContextRepository interface {
	Identity(ctx context.Context, entityID string) (EntityIdentity, error)
	EdgesFor(ctx context.Context, ownerUserID, entityID string) ([]EntityEdge, error)
	ContextFor(ctx context.Context, ownerUserID, entityID string) ([]EntityContextItem, error)
	InsertEdge(ctx context.Context, in DrawEdgeInput) error
	// PendingMergesFor returns the judge's unanswered proposals touching this entity.
	PendingMergesFor(ctx context.Context, entityID string) ([]PendingMerge, error)
	// MergeQueue returns all unanswered proposals (the global inbox).
	MergeQueue(ctx context.Context, limit int) ([]MergeQueueItem, error)
	// ResolveRoot follows the merge overlay to the entity's canonical root.
	ResolveRoot(ctx context.Context, entityID string) (string, error)
	// Exists reports whether an entity id is real (endpoint integrity for edge writes:
	// relationships deliberately has no FKs on its polymorphic endpoints).
	Exists(ctx context.Context, entityID string) (bool, error)
	// DataPointsFor returns the entity's typed data-point map.
	DataPointsFor(ctx context.Context, entityID string) ([]EntityDataPoint, error)
	// ExternalIdsFor returns the entity's hard cross-system identity keys.
	ExternalIdsFor(ctx context.Context, entityID string) ([]EntityExternalID, error)
	// CompanyFor returns the kind='company' extension row, or nil if the entity has none.
	CompanyFor(ctx context.Context, entityID string) (*CompanyFacts, error)
}

type DefaultEntityContextService struct {
	repo   EntityContextRepository
	merges MergeStore
}

// NewEntityContextService takes the merge store separately: reads come from the surface's
// own repo, decisions go to the shared entity registry (where the judge also writes).
func NewEntityContextService(repo EntityContextRepository, merges MergeStore) *DefaultEntityContextService {
	return &DefaultEntityContextService{repo: repo, merges: merges}
}

func (s *DefaultEntityContextService) Get(ctx context.Context, ownerUserID, entityID string) (EntityContext, error) {
	entityID = strings.TrimSpace(entityID)
	if entityID == "" {
		return EntityContext{}, ErrInvalidEntityTag
	}
	root, err := s.repo.ResolveRoot(ctx, entityID)
	if err != nil {
		return EntityContext{}, err
	}
	identity, err := s.repo.Identity(ctx, root)
	if err != nil {
		return EntityContext{}, err
	}
	edges, err := s.repo.EdgesFor(ctx, ownerUserID, root)
	if err != nil {
		return EntityContext{}, err
	}
	items, err := s.repo.ContextFor(ctx, ownerUserID, root)
	if err != nil {
		return EntityContext{}, err
	}
	merges, err := s.repo.PendingMergesFor(ctx, root)
	if err != nil {
		return EntityContext{}, err
	}
	// Non-nil slices: the surface's empty states key off length, and JSON null would
	// force the client to guard.
	if edges == nil {
		edges = []EntityEdge{}
	}
	if items == nil {
		items = []EntityContextItem{}
	}
	if merges == nil {
		merges = []PendingMerge{}
	}
	dataPoints, err := s.repo.DataPointsFor(ctx, root)
	if err != nil {
		return EntityContext{}, err
	}
	externalIds, err := s.repo.ExternalIdsFor(ctx, root)
	if err != nil {
		return EntityContext{}, err
	}
	var company *CompanyFacts
	if identity.Kind == "company" {
		company, err = s.repo.CompanyFor(ctx, root)
		if err != nil {
			return EntityContext{}, err
		}
	}
	if dataPoints == nil {
		dataPoints = []EntityDataPoint{}
	}
	if externalIds == nil {
		externalIds = []EntityExternalID{}
	}
	identity.Since = provenanceLine(identity.Since, len(items))
	return EntityContext{
		Entity: identity, Edges: edges, Context: items, Merges: merges,
		DataPoints: dataPoints, ExternalIds: externalIds, Company: company,
	}, nil
}

func (s *DefaultEntityContextService) MergeQueue(ctx context.Context, limit int) ([]MergeQueueItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	items, err := s.repo.MergeQueue(ctx, limit)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []MergeQueueItem{}
	}
	return items, nil
}

func (s *DefaultEntityContextService) AcceptMerge(ctx context.Context, mergeID string) error {
	return s.decide(ctx, mergeID, s.merges.AcceptMerge)
}

func (s *DefaultEntityContextService) RejectMerge(ctx context.Context, mergeID string) error {
	return s.decide(ctx, mergeID, s.merges.RejectMerge)
}

// decide normalizes both decisions: a missing/already-decided row surfaces as
// ErrMergeNotPending rather than a raw pgx no-rows error, so a double-click is a 409.
func (s *DefaultEntityContextService) decide(ctx context.Context, mergeID string, apply func(context.Context, string) error) error {
	if strings.TrimSpace(mergeID) == "" {
		return ErrInvalidEntityTag
	}
	if s.merges == nil {
		return ErrMergeNotPending
	}
	if err := apply(ctx, mergeID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMergeNotPending
		}
		return err
	}
	return nil
}

func (s *DefaultEntityContextService) DrawEdge(ctx context.Context, in DrawEdgeInput) error {
	in.FromID = strings.TrimSpace(in.FromID)
	in.ToID = strings.TrimSpace(in.ToID)
	in.Rel = strings.TrimSpace(in.Rel)
	in.Why = strings.TrimSpace(in.Why)
	if in.OwnerUserID == "" || in.FromID == "" || in.ToID == "" {
		return ErrInvalidEntityTag
	}
	if !ValidEntityRelType(in.Rel) {
		return ErrInvalidEntityTag
	}
	if in.FromID == in.ToID {
		return ErrInvalidEntityTag // an entity cannot relate to itself
	}
	// Endpoints resolve through merges, so an edge drawn to a merged-away entity binds to
	// the thing it actually became.
	from, err := s.repo.ResolveRoot(ctx, in.FromID)
	if err != nil {
		return err
	}
	to, err := s.repo.ResolveRoot(ctx, in.ToID)
	if err != nil {
		return err
	}
	if from == to {
		return ErrInvalidEntityTag // same entity after merge resolution
	}
	// relationships has no FK on its polymorphic endpoints — the service is the integrity
	// boundary (00006's design).
	for _, id := range []string{from, to} {
		ok, err := s.repo.Exists(ctx, id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrEntityNotFound
		}
	}
	in.FromID, in.ToID = from, to
	return s.repo.InsertEdge(ctx, in)
}

// provenanceLine renders the meta line: "tracked 3 weeks · 14 pieces of context".
// tracked carries the age phrase the repo computed; count is the context total.
func provenanceLine(tracked string, count int) string {
	parts := []string{}
	if tracked != "" {
		parts = append(parts, "tracked "+tracked)
	}
	noun := "pieces"
	if count == 1 {
		noun = "piece"
	}
	amount := strconv.Itoa(count)
	if count == 0 {
		amount = "no"
	}
	parts = append(parts, amount+" "+noun+" of context")
	return strings.Join(parts, " · ")
}
