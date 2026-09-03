package research

import (
	"context"
	"encoding/json"
	"errors"

	coreservice "aladin/backend_v2/internal/service"
)

type entityService struct{ research ResearchService }

// NewEntityService adapts Research to the shared entity registry contract.
func NewEntityService(research ResearchService) coreservice.EntityService {
	return entityService{research: research}
}

func (entityService) Kind() string     { return "research" }
func (entityService) Observable() bool { return true }

func (s entityService) Exists(ctx context.Context, id string) (bool, error) {
	_, err := s.research.Get(ctx, id)
	if errors.Is(err, coreservice.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (s entityService) NodeView(ctx context.Context, id string) (coreservice.NodeView, error) {
	folder, err := s.research.Get(ctx, id)
	if err != nil {
		return coreservice.NodeView{}, err
	}
	hypothesis, truncated := coreservice.TruncateForView(folder.Hypothesis)
	raw, err := json.Marshal(map[string]any{
		"title":      folder.Title,
		"hypothesis": hypothesis,
		"sourceKind": folder.SourceKind,
		"execMode":   folder.ExecMode,
		"runState":   folder.RunState,
	})
	if err != nil {
		return coreservice.NodeView{}, err
	}
	return coreservice.NodeView{
		ID: folder.NodeID, Kind: "research", Title: folder.Title,
		Data: raw, Truncated: truncated,
	}, nil
}
