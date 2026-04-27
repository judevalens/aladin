package service

import (
	"context"
	"strings"
)

type SourceService interface {
	List(context.Context) ([]SourceRecord, error)
	Create(context.Context, SourceCreateInput) (SourceRecord, error)
	Delete(context.Context, string) error
}

type SourceRepository interface {
	EnsureDefaultUserAndGraph(context.Context) (string, error)
	List(context.Context) ([]SourceRecord, error)
	Create(context.Context, string, string, *SourcePayload) (SourceRecord, error)
	Delete(context.Context, string) error
}

type SourceRecord struct {
	ID                   string         `json:"id"`
	Name                 string         `json:"name"`
	Type                 string         `json:"type"`
	SyncMode             string         `json:"syncMode"`
	SyncState            string         `json:"syncState"`
	Config               map[string]any `json:"config"`
	AutoPromoteThreshold float64        `json:"autoPromoteThreshold"`
	SuggestThreshold     float64        `json:"suggestThreshold"`
	CreatedAt            string         `json:"createdAt"`
	LastSyncedAt         *string        `json:"lastSyncedAt"`
}

type SourcePayload struct {
	Name     string
	Type     string
	SyncMode string
	Config   map[string]any
}

type SourceCreateInput struct {
	Kind                string `json:"kind"`
	Name                string `json:"name"`
	Subreddit           string `json:"subreddit"`
	Sort                string `json:"sort"`
	Query               string `json:"query"`
	Handle              string `json:"handle"`
	MinScore            *int   `json:"minScore"`
	IncludeComments     *bool  `json:"includeComments"`
	TopComments         *int   `json:"topComments"`
	SyncIntervalSeconds *int   `json:"syncIntervalSeconds"`
	Limit               *int   `json:"limit"`
	MinLikes            *int   `json:"minLikes"`
	MaxResults          *int   `json:"maxResults"`
}

type DefaultSourceService struct{ repo SourceRepository }

func NewSourceService(repo SourceRepository) *DefaultSourceService {
	return &DefaultSourceService{repo: repo}
}

func (s *DefaultSourceService) List(ctx context.Context) ([]SourceRecord, error) {
	if _, err := s.repo.EnsureDefaultUserAndGraph(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx)
}

func (s *DefaultSourceService) Create(ctx context.Context, input SourceCreateInput) (SourceRecord, error) {
	payload, err := buildSourcePayload(input)
	if err != nil {
		return SourceRecord{}, err
	}
	kgID, err := s.repo.EnsureDefaultUserAndGraph(ctx)
	if err != nil {
		return SourceRecord{}, err
	}
	return s.repo.Create(ctx, newID(""), kgID, payload)
}

func (s *DefaultSourceService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func buildSourcePayload(input SourceCreateInput) (*SourcePayload, error) {
	kind := strings.TrimSpace(input.Kind)
	if kind == "" {
		return nil, BadRequest("kind is required")
	}
	name := strings.TrimSpace(input.Name)

	switch kind {
	case "reddit_subreddit":
		subreddit := strings.TrimSpace(input.Subreddit)
		if subreddit == "" {
			return nil, BadRequest("subreddit is required")
		}
		subreddit = strings.Trim(strings.TrimPrefix(subreddit, "r/"), "/")
		sort := strings.ToLower(strings.TrimSpace(input.Sort))
		if sort == "" {
			sort = "new"
		}
		if sort != "new" && sort != "hot" && sort != "top" {
			return nil, BadRequest("sort must be one of: new, hot, top")
		}
		return &SourcePayload{
			Name:     firstNonEmpty(name, "r/"+subreddit),
			Type:     "reddit",
			SyncMode: "poll",
			Config: map[string]any{
				"subreddit":             subreddit,
				"min_score":             maxInt(intOrDefault(input.MinScore, 10), 0),
				"include_comments":      boolOrDefault(input.IncludeComments, true),
				"top_comments_n":        maxInt(intOrDefault(input.TopComments, 5), 0),
				"sort":                  sort,
				"after":                 nil,
				"last_seen_id":          nil,
				"sync_interval_seconds": intOrDefault(input.SyncIntervalSeconds, 10),
			},
		}, nil
	case "bluesky_search":
		query := strings.TrimSpace(input.Query)
		if query == "" {
			return nil, BadRequest("query is required")
		}
		return &SourcePayload{
			Name:     firstNonEmpty(name, "Bluesky: "+truncate(query, 48)),
			Type:     "bluesky",
			SyncMode: "poll",
			Config: map[string]any{
				"mode":                  "search",
				"query":                 query,
				"cursor":                nil,
				"limit":                 maxInt(intOrDefault(input.Limit, 50), 1),
				"sync_interval_seconds": intOrDefault(input.SyncIntervalSeconds, 10),
			},
		}, nil
	case "bluesky_account":
		handle := strings.TrimPrefix(strings.TrimSpace(input.Handle), "@")
		if handle == "" {
			return nil, BadRequest("handle is required")
		}
		return &SourcePayload{
			Name:     firstNonEmpty(name, "@"+handle),
			Type:     "bluesky",
			SyncMode: "poll",
			Config: map[string]any{
				"mode":                  "account",
				"handle":                handle,
				"cursor":                nil,
				"limit":                 maxInt(intOrDefault(input.Limit, 50), 1),
				"sync_interval_seconds": intOrDefault(input.SyncIntervalSeconds, 10),
			},
		}, nil
	case "twitter_search":
		query := strings.TrimSpace(input.Query)
		if query == "" {
			return nil, BadRequest("query is required")
		}
		return &SourcePayload{
			Name:     firstNonEmpty(name, "X: "+truncate(query, 48)),
			Type:     "twitter",
			SyncMode: "poll",
			Config: map[string]any{
				"mode":                  "search",
				"query":                 query,
				"since_id":              nil,
				"min_likes":             maxInt(intOrDefault(input.MinLikes, 5), 0),
				"max_results":           maxInt(intOrDefault(input.MaxResults, 25), 10),
				"sync_interval_seconds": intOrDefault(input.SyncIntervalSeconds, 10),
			},
		}, nil
	default:
		return nil, BadRequest("unsupported source kind '" + kind + "'")
	}
}

func intOrDefault(v *int, fallback int) int {
	if v == nil {
		return fallback
	}
	return *v
}

func boolOrDefault(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
