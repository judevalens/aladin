package graphpane

import "context"

// GraphPane is the "On the graph" side-pane payload for an artifact: the entities it is
// connected to and the other artifacts it links to. Surfaces the entity layer.
type GraphPane struct {
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

// GraphEntity is an entity connected to the pane's subject. Origin records how it's
// connected — 'tag' (you tagged the page) or 'mention' (inline @entity) — with
// tag > mention precedence; only 'tag' is removable.
type GraphEntity struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Mentions int    `json:"mentions"`
	Origin   string `json:"origin"`
}

type GraphPaneService interface {
	ForArtifact(ctx context.Context, artifactID string) (*GraphPane, error)
}

type GraphPaneRepository interface {
	ForArtifact(ctx context.Context, artifactID string) (*GraphPane, error)
}

type DefaultGraphPaneService struct{ repo GraphPaneRepository }

func NewGraphPaneService(repo GraphPaneRepository) *DefaultGraphPaneService {
	return &DefaultGraphPaneService{repo: repo}
}

func (s *DefaultGraphPaneService) ForArtifact(ctx context.Context, id string) (*GraphPane, error) {
	return s.repo.ForArtifact(ctx, id)
}
