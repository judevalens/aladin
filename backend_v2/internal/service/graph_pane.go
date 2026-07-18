package service

import "context"

// GraphPane is the "On the graph" side-pane payload for a thesis: the thesis claim, the
// claims about its entities (each grounded in N sources or not), the cited source
// records, and the entities it is about. Surfaces the entity + claim layers.
type GraphPane struct {
	Thesis          *GraphThesis          `json:"thesis"`
	Claims          []GraphClaim          `json:"claims"`
	Cites           []GraphCite           `json:"cites"`
	Entities        []GraphEntity         `json:"entities"`
	LinkedArtifacts []GraphLinkedArtifact `json:"linkedArtifacts"`
}

// GraphLinkedArtifact is another artifact connected to the pane's subject. Relation
// records how: 'referenced_by' / 'references' (a # cross-reference in either direction)
// or 'shared_entity' (both mention the same entity).
type GraphLinkedArtifact struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Kind     string `json:"kind"`
	Relation string `json:"relation"`
}

type GraphThesis struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Polarity  string `json:"polarity"`
	TrustTier string `json:"trustTier"`
	UpdatedAt string `json:"updatedAt"`
}

// GraphClaim is a claim about one of the thesis's entities. Grounded = backed by >=1
// source (claim_mentions); Sources = how many distinct sources back it.
type GraphClaim struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Grounded bool   `json:"grounded"`
	Sources  int    `json:"sources"`
}

type GraphCite struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	SourceURL string `json:"sourceUrl"`
	Provider  string `json:"provider"`
	CreatedAt string `json:"createdAt"`
}

// GraphEntity is an entity connected to the pane's subject. Origin records how it's
// connected — 'tag' (you tagged the page), 'mention' (inline @entity), or 'claim'
// (subject of a claim) — with tag > mention > claim precedence; only 'tag' is removable.
type GraphEntity struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Mentions int    `json:"mentions"`
	Origin   string `json:"origin"`
}

type GraphPaneService interface {
	ForThesis(ctx context.Context, thesisClaimID string) (*GraphPane, error)
	ForArtifact(ctx context.Context, artifactID string) (*GraphPane, error)
}

type GraphPaneRepository interface {
	ForThesis(ctx context.Context, thesisClaimID string) (*GraphPane, error)
	ForArtifact(ctx context.Context, artifactID string) (*GraphPane, error)
}

type DefaultGraphPaneService struct{ repo GraphPaneRepository }

func NewGraphPaneService(repo GraphPaneRepository) *DefaultGraphPaneService {
	return &DefaultGraphPaneService{repo: repo}
}

func (s *DefaultGraphPaneService) ForThesis(ctx context.Context, id string) (*GraphPane, error) {
	return s.repo.ForThesis(ctx, id)
}

func (s *DefaultGraphPaneService) ForArtifact(ctx context.Context, id string) (*GraphPane, error) {
	return s.repo.ForArtifact(ctx, id)
}
