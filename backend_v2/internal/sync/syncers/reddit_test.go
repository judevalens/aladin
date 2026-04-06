package syncers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/sync"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type fakeSeenStore struct {
	known map[string]bool
}

func (f fakeSeenStore) Seen(ctx context.Context, sourceID string, externalIDs []string) (map[string]bool, error) {
	out := make(map[string]bool, len(externalIDs))
	for _, id := range externalIDs {
		if f.known[id] {
			out[id] = true
		}
	}
	return out, nil
}

func (f fakeSeenStore) MarkSeen(ctx context.Context, sourceID string, externalIDs []string) error {
	return nil
}

func TestRedditBuildJobBootstrapForNewSource(t *testing.T) {
	t.Parallel()

	s := NewRedditSyncer(sync.NewNoopSeenStore())
	job, err := s.BuildJob(db.Source{
		ID:   "source-1",
		KgID: "kg-1",
		Name: "golang",
		Type: "reddit",
		Config: map[string]any{
			"subreddit": "golang",
		},
	}, nil)
	if err != nil {
		t.Fatalf("BuildJob error: %v", err)
	}

	var payload db.SyncJob
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := payload.Payload["bootstrap"]; ok {
		t.Fatal("bootstrap should not be present")
	}
	if got := payload.Payload["next_last_seen_id"]; got != "" {
		t.Fatalf("next_last_seen_id = %v, want empty", got)
	}
}

func TestRedditBuildJobCarriesLastSeenBoundary(t *testing.T) {
	t.Parallel()

	s := NewRedditSyncer(sync.NewNoopSeenStore())
	job, err := s.BuildJob(db.Source{
		ID:   "source-1",
		KgID: "kg-1",
		Name: "golang",
		Type: "reddit",
		Config: map[string]any{
			"subreddit":             "golang",
			"last_seen_id":          "seen-123",
			"last_seen_created_utc": float64(1234),
		},
	}, nil)
	if err != nil {
		t.Fatalf("BuildJob error: %v", err)
	}

	var payload db.SyncJob
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := payload.Payload["stop_before_id"]; ok {
		t.Fatal("stop_before_id should not be present")
	}
	if _, ok := payload.Payload["bootstrap"]; ok {
		t.Fatal("bootstrap should not be present")
	}
}

func TestRedditExecuteStopsAtSeenID(t *testing.T) {
	t.Parallel()

	s := NewRedditSyncer(fakeSeenStore{known: map[string]bool{"seen-1": true}})
	s.client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"data":{"after":"t3_next","children":[
			{"data":{"id":"new-3","title":"newest","selftext":"","permalink":"/r/golang/comments/new3/x","score":10,"created_utc":300}},
			{"data":{"id":"new-2","title":"newer","selftext":"","permalink":"/r/golang/comments/new2/x","score":9,"created_utc":200}},
			{"data":{"id":"seen-1","title":"seen","selftext":"","permalink":"/r/golang/comments/seen1/x","score":8,"created_utc":100}}
		]}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

	job := &db.SyncJob{
		SourceID:   "source-1",
		SourceType: "reddit",
		Payload: map[string]any{
			"subreddit":                  "golang",
			"after":                      "",
			"next_last_seen_id":          "",
			"next_last_seen_created_utc": float64(0),
		},
	}

	result, err := s.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.HasMore {
		t.Fatal("HasMore = true, want false after hitting boundary")
	}
	if result.CompletionReason != sync.CompletionReasonSeenOverlap {
		t.Fatalf("CompletionReason = %q, want %q", result.CompletionReason, sync.CompletionReasonSeenOverlap)
	}
	if len(result.Artifacts) != 2 {
		t.Fatalf("artifact count = %d, want 2", len(result.Artifacts))
	}
	if got := result.SourceUpdates["last_seen_id"]; got != "new-3" {
		t.Fatalf("last_seen_id = %v, want new-3", got)
	}
	if got := floatFromAny(result.SourceUpdates["last_seen_created_utc"]); got != 300 {
		t.Fatalf("last_seen_created_utc = %v, want 300", got)
	}
	if got := result.HeadBoundary["id"]; got != "new-3" {
		t.Fatalf("head_boundary.id = %v, want new-3", got)
	}
	if got := floatFromAny(result.HeadBoundary["created_utc"]); got != 300 {
		t.Fatalf("head_boundary.created_utc = %v, want 300", got)
	}
}

func TestRedditExecuteCarriesNextBoundaryAcrossPagination(t *testing.T) {
	t.Parallel()

	s := NewRedditSyncer(sync.NewNoopSeenStore())
	s.client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"data":{"after":"t3_next","children":[
			{"data":{"id":"new-3","title":"newest","selftext":"","permalink":"/r/golang/comments/new3/x","score":10,"created_utc":300}},
			{"data":{"id":"new-2","title":"newer","selftext":"","permalink":"/r/golang/comments/new2/x","score":9,"created_utc":200}}
		]}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

	job := &db.SyncJob{
		CorrelationID: "corr-1",
		SourceID:      "source-1",
		SourceType:    "reddit",
		Priority:      5,
		MaxAttempts:   3,
		Payload: map[string]any{
			"subreddit":                  "golang",
			"after":                      "",
			"next_last_seen_id":          "",
			"next_last_seen_created_utc": float64(0),
		},
	}

	result, err := s.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.HasMore {
		t.Fatal("HasMore = false, want true for pagination follow-up")
	}
	if result.CompletionReason != "" {
		t.Fatalf("CompletionReason = %q, want empty for continued pagination", result.CompletionReason)
	}
	if got := result.CursorUpdates["after"]; got != "t3_next" {
		t.Fatalf("after = %v, want t3_next", got)
	}
	if got := result.SourceUpdates["last_seen_id"]; got != "new-3" {
		t.Fatalf("last_seen_id = %v, want new-3", got)
	}
	if got := result.HeadBoundary["id"]; got != "new-3" {
		t.Fatalf("head_boundary.id = %v, want new-3", got)
	}
}

func TestRedditExecuteDoesNotAdvanceHighWaterMarkWhenFirstPostIsSeen(t *testing.T) {
	t.Parallel()

	s := NewRedditSyncer(fakeSeenStore{known: map[string]bool{"seen-1": true}})
	s.client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"data":{"after":"t3_next","children":[
			{"data":{"id":"seen-1","title":"seen","selftext":"","permalink":"/r/golang/comments/seen1/x","score":8,"created_utc":100}}
		]}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

	job := &db.SyncJob{
		SourceID:   "source-1",
		SourceType: "reddit",
		Payload: map[string]any{
			"subreddit":                  "golang",
			"after":                      "",
			"next_last_seen_id":          "",
			"next_last_seen_created_utc": float64(0),
		},
	}

	result, err := s.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if len(result.Artifacts) != 0 {
		t.Fatalf("artifact count = %d, want 0", len(result.Artifacts))
	}
	if result.CompletionReason != sync.CompletionReasonSeenOverlap {
		t.Fatalf("CompletionReason = %q, want %q", result.CompletionReason, sync.CompletionReasonSeenOverlap)
	}
	if _, ok := result.SourceUpdates["last_seen_id"]; ok {
		t.Fatal("last_seen_id should not advance when boundary is first post")
	}
	if len(result.HeadBoundary) != 0 {
		t.Fatalf("head_boundary = %v, want empty", result.HeadBoundary)
	}
}
