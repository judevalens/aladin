package system

import "context"

type SystemService interface {
	Ready(context.Context) error
	WorkerStatus(context.Context) (map[string]any, error)
	// PipelineStats is a read-only health snapshot of the ingestion pipeline:
	// record counts by status, stuck/throughput figures, and insight/match/edge
	// breakdowns. Surfaces stuck records (the silent-failure problem) without
	// touching the live write/retry path.
	PipelineStats(context.Context) (map[string]any, error)
}

type SystemRepository interface {
	Ready(context.Context) error
	WorkerStatus(context.Context) (map[string]any, error)
	PipelineStats(context.Context) (map[string]any, error)
}

type DefaultSystemService struct{ repo SystemRepository }

func NewSystemService(repo SystemRepository) *DefaultSystemService {
	return &DefaultSystemService{repo: repo}
}

func (s *DefaultSystemService) Ready(ctx context.Context) error {
	return s.repo.Ready(ctx)
}

func (s *DefaultSystemService) WorkerStatus(ctx context.Context) (map[string]any, error) {
	return s.repo.WorkerStatus(ctx)
}

func (s *DefaultSystemService) PipelineStats(ctx context.Context) (map[string]any, error) {
	return s.repo.PipelineStats(ctx)
}
