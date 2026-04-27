package service

import "context"

type SystemService interface {
	Ready(context.Context) error
	WorkerStatus(context.Context) (map[string]any, error)
}

type SystemRepository interface {
	Ready(context.Context) error
	WorkerStatus(context.Context) (map[string]any, error)
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
