package feed

import "context"

type FeedService interface {
	List(context.Context, FeedListParams) (map[string]any, error)
	Topics(context.Context) ([]string, error)
	Sources(context.Context) ([]FeedSourceRecord, error)
	UpdateStatus(context.Context, string, string) error
}

type FeedRepository interface {
	List(context.Context, FeedListParams) (map[string]any, error)
	Topics(context.Context) ([]string, error)
	Sources(context.Context) ([]FeedSourceRecord, error)
	UpdateStatus(context.Context, string, string) error
}

type FeedListParams struct {
	Limit      int
	Offset     int
	SourceType string
	Topic      string
	SavedOnly  bool
	Sort       string
}

type FeedSourceRecord struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	SyncState    string  `json:"syncState"`
	LastSyncedAt *string `json:"lastSyncedAt"`
}

type FeedItem struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Label       string         `json:"label"`
	Content     string         `json:"content"`
	SourceURL   *string        `json:"sourceUrl"`
	Enrichment  any            `json:"enrichment"`
	Metadata    map[string]any `json:"metadata"`
	UserStatus  *string        `json:"userStatus"`
	SourceType  *string        `json:"sourceType"`
	SourceName  *string        `json:"sourceName"`
	SignalScore float64        `json:"signalScore"`
	CreatedAt   string         `json:"createdAt"`
}

type DefaultFeedService struct{ repo FeedRepository }

func NewFeedService(repo FeedRepository) *DefaultFeedService {
	return &DefaultFeedService{repo: repo}
}

func (s *DefaultFeedService) List(ctx context.Context, params FeedListParams) (map[string]any, error) {
	return s.repo.List(ctx, params)
}

func (s *DefaultFeedService) Topics(ctx context.Context) ([]string, error) {
	return s.repo.Topics(ctx)
}

func (s *DefaultFeedService) Sources(ctx context.Context) ([]FeedSourceRecord, error) {
	return s.repo.Sources(ctx)
}

func (s *DefaultFeedService) UpdateStatus(ctx context.Context, id string, status string) error {
	return s.repo.UpdateStatus(ctx, id, status)
}
