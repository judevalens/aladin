package research

import (
	"aladin/backend_v2/internal/artifact"
	"context"
	"strings"

	coreservice "aladin/backend_v2/internal/service"
)

// BrowserNodeResponse remains the shared tree projection used by the browser
// and sync surfaces; Research owns the workflow that creates it.
type BrowserNodeResponse = artifact.BrowserNodeResponse

// research.go — the research bench's write side (design/RESEARCH_SURFACE_PRD.md §5).
//
// A research folder is a tree node with kind='research' plus a 1:1 extension row holding
// strategy facts. It is NOT a folder with a label: the kind buys structural slots that
// exist at creation, a constrained `+` menu, seeded typed properties, a stable address,
// and state. One research folder = one strategy — including the case where no code exists
// yet, which is why the extension row is created sparse rather than deferred.

// ResearchFolder is a research folder's own facts (the extension row), read for the
// Overview. The tree row itself rides the normal node sync path, so this carries only
// what the tree frame deliberately leaves off.
type ResearchFolder struct {
	NodeID     string  `json:"nodeId"`
	Title      string  `json:"title"`
	ParentID   *string `json:"parentId"`
	Hypothesis string  `json:"hypothesis"`
	SourceKind string  `json:"sourceKind"`
	SourceRef  string  `json:"sourceRef,omitempty"`
	CommitSHA  string  `json:"commitSha,omitempty"`
	CodeHash   string  `json:"codeHash,omitempty"`
	ExecMode   string  `json:"execMode"`
	RunState   string  `json:"runState"`
}

// ResearchCreateInput creates a research folder. Everything but the title is optional —
// §5's sparse extension row. A caller that only knows "I want to start researching X"
// supplies a title and nothing else.
type ResearchCreateInput struct {
	ID         string
	Title      string
	ParentID   *string
	Hypothesis string
}

type ResearchService interface {
	Create(context.Context, ResearchCreateInput) (BrowserNodeResponse, error)
	Get(context.Context, string) (ResearchFolder, error)
	Update(ctx context.Context, id string, patch ResearchPatch) (BrowserNodeResponse, error)
}

type ResearchRepository interface {
	// CreateResearchFolder writes the tree node and its extension row in ONE tx and
	// emits the node's upsert frame, so the row and the sync event commit together.
	CreateResearchFolder(context.Context, ResearchCreateInput, int64) (BrowserNodeResponse, error)
	GetResearchFolder(context.Context, string) (ResearchFolder, error)
	// UpdateResearchFolder applies the patch and emits the node's upsert frame, so the
	// change reaches open surfaces the same way every other node change does — even for
	// fields that don't ride the frame, since the Overview refetches off that event.
	UpdateResearchFolder(ctx context.Context, id string, patch ResearchPatch) (BrowserNodeResponse, error)
	// NextResearchPosition is the sibling position for a new node under parentID.
	NextResearchPosition(context.Context, *string) (int64, error)
	// ParentIsFolder reports whether parentID is a plain folder the caller owns.
	ParentIsFolder(context.Context, string) (bool, error)
}

type researchService struct{ repo ResearchRepository }

// NewResearchService returns the interface, never the concrete type (DI exposes
// interfaces — see CLAUDE.md's backend conventions).
func NewResearchService(repo ResearchRepository) ResearchService {
	return &researchService{repo: repo}
}

func (s *researchService) Create(ctx context.Context, in ResearchCreateInput) (BrowserNodeResponse, error) {
	if err := coreservice.RequireScope(ctx, coreservice.ScopeArtifactsWrite); err != nil {
		return BrowserNodeResponse{}, err
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return BrowserNodeResponse{}, coreservice.BadRequest("title is required")
	}
	in.Hypothesis = strings.TrimSpace(in.Hypothesis)
	in.ParentID = coreservice.TrimStringPtr(in.ParentID)

	// §5: "Comparing two strategies is a view ABOVE research folders, never a folder
	// containing them." A research folder therefore nests inside plain folders only —
	// nesting research in research is the shape the PRD explicitly rules out.
	if in.ParentID != nil {
		ok, err := s.repo.ParentIsFolder(ctx, *in.ParentID)
		if err != nil {
			return BrowserNodeResponse{}, err
		}
		if !ok {
			return BrowserNodeResponse{}, coreservice.BadRequest("a research folder can only be created at the root or inside a plain folder")
		}
	}

	if in.ID = strings.TrimSpace(in.ID); in.ID == "" {
		in.ID = coreservice.NewID("research-", "")
	}
	position, err := s.repo.NextResearchPosition(ctx, in.ParentID)
	if err != nil {
		return BrowserNodeResponse{}, err
	}
	return s.repo.CreateResearchFolder(ctx, in, position)
}

// ResearchPatch is the research folder's mutable surface. Nil means "leave alone", so a
// hypothesis edit never touches the title and vice versa.
type ResearchPatch struct {
	Title      *string
	Hypothesis *string
}

func (s *researchService) Update(ctx context.Context, id string, patch ResearchPatch) (BrowserNodeResponse, error) {
	if err := coreservice.RequireScope(ctx, coreservice.ScopeArtifactsWrite); err != nil {
		return BrowserNodeResponse{}, err
	}
	if strings.TrimSpace(id) == "" {
		return BrowserNodeResponse{}, coreservice.ErrNotFound
	}
	if patch.Title == nil && patch.Hypothesis == nil {
		return BrowserNodeResponse{}, coreservice.BadRequest("nothing to update")
	}
	if patch.Title != nil {
		title := strings.TrimSpace(*patch.Title)
		if title == "" {
			return BrowserNodeResponse{}, coreservice.BadRequest("title cannot be empty")
		}
		patch.Title = &title
	}
	if patch.Hypothesis != nil {
		// The hypothesis is prose and may legitimately be cleared.
		hypothesis := strings.TrimSpace(*patch.Hypothesis)
		patch.Hypothesis = &hypothesis
	}
	return s.repo.UpdateResearchFolder(ctx, id, patch)
}

func (s *researchService) Get(ctx context.Context, id string) (ResearchFolder, error) {
	if err := coreservice.RequireScope(ctx, coreservice.ScopeArtifactsRead); err != nil {
		return ResearchFolder{}, err
	}
	if strings.TrimSpace(id) == "" {
		return ResearchFolder{}, coreservice.ErrNotFound
	}
	return s.repo.GetResearchFolder(ctx, id)
}
