package service

import "context"

type InsightService interface {
	List(context.Context, InsightListParams) (map[string]any, error)
	Stats(context.Context) (map[string]any, error)
	UpdateStatus(context.Context, string, string) error
}

type InsightRepository interface {
	List(context.Context, InsightListParams) (map[string]any, error)
	Stats(context.Context) (map[string]any, error)
	UpdateStatus(context.Context, string, string) error
}

type InsightListParams struct {
	Limit  int
	Offset int
	Type   string
	Status string
}

type InsightRecord struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	Entity     *string  `json:"entity"`
	Topic      *string  `json:"topic"`
	RecordIDs  []string `json:"recordIds"`
	Confidence float64  `json:"confidence"`
	UserStatus string   `json:"userStatus"`
	CreatedAt  string   `json:"createdAt"`
}

type DefaultInsightService struct{ repo InsightRepository }

func NewInsightService(repo InsightRepository) *DefaultInsightService {
	return &DefaultInsightService{repo: repo}
}

func (s *DefaultInsightService) List(ctx context.Context, params InsightListParams) (map[string]any, error) {
	return s.repo.List(ctx, params)
}

func (s *DefaultInsightService) Stats(ctx context.Context) (map[string]any, error) {
	return s.repo.Stats(ctx)
}

func (s *DefaultInsightService) UpdateStatus(ctx context.Context, id string, status string) error {
	return s.repo.UpdateStatus(ctx, id, status)
}
