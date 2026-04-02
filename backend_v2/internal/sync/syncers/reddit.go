package syncers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/sync"
)

const (
	redditAPI   = "https://www.reddit.com"
	userAgent   = "aladin-bot/0.1"
	rateLimit   = 10 // requests per minute
)

type RedditSyncer struct {
	client      *http.Client
	lastRequest time.Time
	minInterval time.Duration
}

func NewRedditSyncer() *RedditSyncer {
	return &RedditSyncer{
		client:      &http.Client{Timeout: 10 * time.Second},
		minInterval: time.Minute / rateLimit,
	}
}

func (r *RedditSyncer) SourceType() string { return "reddit" }

// BuildJob constructs the initial sync job for a Reddit source.
// Validates required config fields and returns an error for invalid sources.
func (r *RedditSyncer) BuildJob(source db.Source) (*db.ScheduledJob, error) {
	subreddit, _ := source.Config["subreddit"].(string)
	if subreddit == "" {
		return nil, fmt.Errorf("reddit source %q missing required config field: subreddit", source.Name)
	}

	payload, err := json.Marshal(db.SyncJob{
		SourceID:    source.ID,
		KgID:        source.KgID,
		SourceType:  "reddit",
		JobType:     "fetch_posts",
		Payload:     map[string]any{"subreddit": subreddit, "after": ""},
		Priority:    5,
		MaxAttempts: 3,
	})
	if err != nil {
		return nil, fmt.Errorf("reddit BuildJob marshal: %w", err)
	}

	return &db.ScheduledJob{
		ID:       source.ID,
		Type:     "sync:reddit",
		Payload:  payload,
		Priority: 5,
		MaxRetry: 3,
		Timeout:  5 * time.Minute,
	}, nil
}

func (r *RedditSyncer) Execute(ctx context.Context, job *db.SyncJob) (*sync.Result, error) {
	r.applyRateLimit()

	subreddit, _ := job.Payload["subreddit"].(string)
	after, _ := job.Payload["after"].(string)

	url := fmt.Sprintf("%s/r/%s/new.json?limit=25&after=%s", redditAPI, subreddit, after)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", userAgent)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reddit fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reddit status %d", resp.StatusCode)
	}

	var body struct {
		Data struct {
			After    string `json:"after"`
			Children []struct {
				Data struct {
					ID         string  `json:"id"`
					Title      string  `json:"title"`
					Selftext   string  `json:"selftext"`
					Permalink  string  `json:"permalink"`
					Score      int     `json:"score"`
					CreatedUTC float64 `json:"created_utc"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("reddit decode: %w", err)
	}

	result := &sync.Result{}
	for _, child := range body.Data.Children {
		d := child.Data
		content := d.Title
		if d.Selftext != "" {
			content += "\n\n" + d.Selftext
		}
		result.Artifacts = append(result.Artifacts, &sync.RawArtifact{
			ExternalID: d.ID,
			Type:       "post",
			Label:      d.Title,
			Content:    content,
			SourceURL:  redditAPI + d.Permalink,
			Metadata:   map[string]any{"score": d.Score},
		})
	}

	if body.Data.After != "" {
		result.NextJob = &db.SyncJob{
			SourceID:    job.SourceID,
			SnapshotID:  job.SnapshotID,
			SourceType:  "reddit",
			JobType:     "fetch_posts",
			Payload:     map[string]any{"subreddit": subreddit, "after": body.Data.After},
			Priority:    job.Priority,
			MaxAttempts: job.MaxAttempts,
		}
	}

	return result, nil
}

func (r *RedditSyncer) applyRateLimit() {
	elapsed := time.Since(r.lastRequest)
	if elapsed < r.minInterval {
		time.Sleep(r.minInterval - elapsed)
	}
	r.lastRequest = time.Now()
}
