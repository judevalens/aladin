package source

import (
	"context"
	"strings"

	coreservice "aladin/backend_v2/internal/service"

	"github.com/google/uuid"
)

type SourceService interface {
	List(context.Context) ([]SourceRecord, error)
	Create(context.Context, SourceCreateInput) (SourceRecord, error)
	Delete(context.Context, string) error
}

type SourceRepository interface {
	EnsureUserAndGraph(context.Context, string) (string, error)
	List(context.Context, string) ([]SourceRecord, error)
	Create(context.Context, string, string, string, *SourcePayload) (SourceRecord, error)
	Delete(context.Context, string, string) error
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
	Name       string
	Type       string
	SyncMode   string
	StreamKind string
	StreamKey  string
	Config     map[string]any
	Policy     map[string]any
}

type SourceCreateInput struct {
	Kind                string `json:"kind"`
	Name                string `json:"name"`
	Subreddit           string `json:"subreddit"`
	Feed                string `json:"feed"`
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
	principal, err := coreservice.RequirePrincipal(ctx)
	if err != nil {
		return nil, coreservice.ErrUnauthenticated
	}
	if _, err := s.repo.EnsureUserAndGraph(ctx, principal.UserID); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, principal.UserID)
}

func (s *DefaultSourceService) Create(ctx context.Context, input SourceCreateInput) (SourceRecord, error) {
	principal, err := coreservice.RequirePrincipal(ctx)
	if err != nil {
		return SourceRecord{}, coreservice.ErrUnauthenticated
	}
	payload, err := buildSourcePayload(input)
	if err != nil {
		return SourceRecord{}, err
	}
	kgID, err := s.repo.EnsureUserAndGraph(ctx, principal.UserID)
	if err != nil {
		return SourceRecord{}, err
	}
	return s.repo.Create(ctx, uuid.NewString(), principal.UserID, kgID, payload)
}

func (s *DefaultSourceService) Delete(ctx context.Context, id string) error {
	principal, err := coreservice.RequirePrincipal(ctx)
	if err != nil {
		return coreservice.ErrUnauthenticated
	}
	return s.repo.Delete(ctx, id, principal.UserID)
}

func buildSourcePayload(input SourceCreateInput) (*SourcePayload, error) {
	kind := strings.TrimSpace(input.Kind)
	if kind == "" {
		return nil, coreservice.BadRequest("kind is required")
	}
	name := strings.TrimSpace(input.Name)

	switch kind {
	case "hackernews_feed":
		feed := strings.ToLower(strings.TrimSpace(input.Feed))
		if feed == "" {
			feed = "topstories"
		}
		switch feed {
		case "topstories", "newstories", "beststories", "askstories", "showstories", "jobstories":
		default:
			return nil, coreservice.BadRequest("feed must be one of: topstories, newstories, beststories, askstories, showstories, jobstories")
		}
		return &SourcePayload{
			Name:       firstNonEmpty(name, "Hacker News: "+feed),
			Type:       "hackernews",
			SyncMode:   "poll",
			StreamKind: "feed",
			StreamKey:  feed,
			Config: map[string]any{
				"feed":                  feed,
				"offset":                0,
				"page_size":             maxInt(intOrDefault(input.Limit, 10), 1),
				"top_comments_n":        maxInt(intOrDefault(input.TopComments, 5), 0),
				"last_seen_item_id":     nil,
				"last_seen_item_time":   nil,
				"sync_interval_seconds": intOrDefault(input.SyncIntervalSeconds, 60),
			},
		}, nil
	case "reddit_subreddit":
		subreddit := strings.TrimSpace(input.Subreddit)
		if subreddit == "" {
			return nil, coreservice.BadRequest("subreddit is required")
		}
		subreddit = strings.Trim(strings.TrimPrefix(subreddit, "r/"), "/")
		sort := strings.ToLower(strings.TrimSpace(input.Sort))
		if sort == "" {
			sort = "new"
		}
		if sort != "new" && sort != "hot" && sort != "top" {
			return nil, coreservice.BadRequest("sort must be one of: new, hot, top")
		}
		return &SourcePayload{
			Name:       firstNonEmpty(name, "r/"+subreddit),
			Type:       "reddit",
			SyncMode:   "poll",
			StreamKind: "subreddit",
			StreamKey:  strings.ToLower(subreddit) + "/" + sort,
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
		query := normalizeSpace(input.Query)
		if query == "" {
			return nil, coreservice.BadRequest("query is required")
		}
		limit := clampInt(intOrDefault(input.Limit, 50), 1, 50)
		interval := maxInt(intOrDefault(input.SyncIntervalSeconds, 300), 60)
		return &SourcePayload{
			Name:       firstNonEmpty(name, "Bluesky: "+truncate(query, 48)),
			Type:       "bluesky",
			SyncMode:   "poll",
			StreamKind: "search",
			StreamKey:  strings.ToLower(query),
			Config: map[string]any{
				"mode":                  "search",
				"query":                 query,
				"sort":                  "top",
				"limit":                 limit,
				"sync_interval_seconds": interval,
			},
			Policy: map[string]any{
				"query":    query,
				"keywords": queryKeywords(query),
			},
		}, nil
	case "bluesky_account":
		return nil, coreservice.BadRequest("bluesky_account is not supported yet")
	case "twitter_search":
		query := strings.TrimSpace(input.Query)
		if query == "" {
			return nil, coreservice.BadRequest("query is required")
		}
		return &SourcePayload{
			Name:       firstNonEmpty(name, "X: "+truncate(query, 48)),
			Type:       "twitter",
			SyncMode:   "poll",
			StreamKind: "search",
			StreamKey:  query,
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
		return nil, coreservice.BadRequest("unsupported source kind '" + kind + "'")
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

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func normalizeSpace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func queryKeywords(query string) []string {
	normalized := normalizeSpace(query)
	if normalized == "" {
		return nil
	}
	keywords := []string{strings.ToLower(normalized)}
	for _, token := range strings.Fields(normalized) {
		token = strings.ToLower(strings.TrimSpace(token))
		if len(token) <= 2 {
			continue
		}
		if !containsString(keywords, token) {
			keywords = append(keywords, token)
		}
	}
	return keywords
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
