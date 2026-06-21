package service

import "context"

// SignalService backs the Signals surface — the curated feed whose PRIMARY object is the claim
// (a contestable, entity-grounded proposition). Enriched records may be mixed in later for a
// "Digg-like" feel; for now a signal IS a claim.
type SignalService interface {
	List(context.Context, SignalListParams) (map[string]any, error)
}

type SignalRepository interface {
	List(context.Context, SignalListParams) (map[string]any, error)
}

type SignalListParams struct {
	Limit  int
	Offset int
	Sort   string // "recent" (default) | "top" (by signal_score)
}

// ClaimSubjectRef is an entity a claim is about.
type ClaimSubjectRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ClaimSignal is one claim rendered as a signal card: the proposition, who/what it's about, the
// evidence tally (how many sources assert vs deny it), and how it connects to other claims.
type ClaimSignal struct {
	ID          string            `json:"id"`
	Text        string            `json:"text"`
	Polarity    string            `json:"polarity"`
	TrustTier   string            `json:"trustTier"`
	Subjects    []ClaimSubjectRef `json:"subjects"`
	AssertCount int               `json:"assertCount"`
	DenyCount   int               `json:"denyCount"`
	Supports    int               `json:"supports"`
	Contradicts int               `json:"contradicts"`
	Qualifies   int               `json:"qualifies"`
	SignalScore float64           `json:"signalScore"`
	CreatedAt   string            `json:"createdAt"`
}

type DefaultSignalService struct{ repo SignalRepository }

func NewSignalService(repo SignalRepository) *DefaultSignalService {
	return &DefaultSignalService{repo: repo}
}

func (s *DefaultSignalService) List(ctx context.Context, params SignalListParams) (map[string]any, error) {
	return s.repo.List(ctx, params)
}
