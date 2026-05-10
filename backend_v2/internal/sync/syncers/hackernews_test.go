package syncers

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/sync"
)

type fakeHackerNewsClient struct {
	ids       []int64
	items     map[int64]*hackerNewsItem
	itemErrs  map[int64]error
	idCalls   []string
	itemCalls []int64
}

func (f *fakeHackerNewsClient) FetchStoryIDs(ctx context.Context, feed string) ([]int64, error) {
	f.idCalls = append(f.idCalls, feed)
	return append([]int64(nil), f.ids...), nil
}

func (f *fakeHackerNewsClient) FetchItem(ctx context.Context, id int64) (*hackerNewsItem, error) {
	f.itemCalls = append(f.itemCalls, id)
	if err := f.itemErrs[id]; err != nil {
		return nil, err
	}
	if item := f.items[id]; item != nil {
		copy := *item
		copy.Kids = append([]int64(nil), item.Kids...)
		return &copy, nil
	}
	return &hackerNewsItem{ID: id, Type: "story", Title: "Story", Time: hackerNewsTestNowUnix(), Score: 1}, nil
}

func TestHackerNewsBuildJobDefaultsToTopStories(t *testing.T) {
	t.Parallel()

	s := NewHackerNewsSyncer(sync.NewNoopSeenStore())
	job, err := s.BuildJob(db.ProviderStream{
		ID:       "source-1",
		Name:     "hn",
		Provider: "hackernews",
		Config:   map[string]any{},
	}, nil)
	if err != nil {
		t.Fatalf("BuildJob error: %v", err)
	}

	var payload db.SyncJob
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Provider != "hackernews" || payload.JobType != "fetch_stories" {
		t.Fatalf("payload source/job = %s/%s, want hackernews/fetch_stories", payload.Provider, payload.JobType)
	}
	if got := payload.Payload["feed"]; got != "topstories" {
		t.Fatalf("feed = %v, want topstories", got)
	}
}

func TestHackerNewsExecuteHydratesAcceptedStoriesInline(t *testing.T) {
	t.Parallel()

	now := hackerNewsTestNowUnix()
	client := &fakeHackerNewsClient{
		ids: []int64{101, 102, 103},
		items: map[int64]*hackerNewsItem{
			101: hnStory(101, "First story", "https://example.com/first", 120, 42, now, 201),
			102: hnStory(102, "Second story", "", 20, 10, now, 202),
			201: hnComment(201, 101, "alice", "first <p>comment", now+1),
			202: hnComment(202, 102, "bob", "second comment", now+1),
		},
	}
	s := NewHackerNewsSyncerWithClient(client, fakeSeenStore{known: map[string]bool{"hn:103": true}})

	result, err := s.Execute(context.Background(), &db.SyncJob{
		ProviderStreamID: "source-1",
		Provider:         "hackernews",
		JobType:          "fetch_stories",
		Payload: map[string]any{
			"feed":           "topstories",
			"page_size":      3,
			"top_comments_n": 2,
		},
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if len(result.Records) != 2 {
		t.Fatalf("record count = %d, want 2", len(result.Records))
	}
	if got, want := client.itemCalls, []int64{101, 201, 102, 202}; !reflect.DeepEqual(got, want) {
		t.Fatalf("item calls = %v, want %v", got, want)
	}
	if strings.Contains(result.Records[0].Content, "first comment") {
		t.Fatal("durable Content includes raw comment body")
	}
	if !strings.Contains(result.Records[0].EnrichmentContent, "first comment") {
		t.Fatalf("EnrichmentContent = %q, want hydrated comment context", result.Records[0].EnrichmentContent)
	}
	if result.CompletionReason != sync.CompletionReasonSeenOverlap {
		t.Fatalf("completion = %q, want seen overlap", result.CompletionReason)
	}
	if got := result.ProviderStreamUpdates["last_seen_item_id"]; got != "hn:101" {
		t.Fatalf("last_seen_item_id = %v, want hn:101", got)
	}
	if len(result.NewCycles) != 2 {
		t.Fatalf("new hydration cycles = %d, want 2", len(result.NewCycles))
	}
	if result.NewCycles[0].Kind != sync.CycleKindHydration || result.NewCycles[0].TargetExternalID != "hn:101" {
		t.Fatalf("first hydration cycle = %#v, want hn:101 hydration", result.NewCycles[0])
	}
}

func TestHackerNewsExecuteFailsWholePageWhenInlineHydrationFails(t *testing.T) {
	t.Parallel()

	now := hackerNewsTestNowUnix()
	s := NewHackerNewsSyncerWithClient(&fakeHackerNewsClient{
		ids: []int64{101},
		items: map[int64]*hackerNewsItem{
			101: hnStory(101, "First story", "https://example.com/first", 120, 42, now, 201),
		},
		itemErrs: map[int64]error{201: errors.New("comment unavailable")},
	}, sync.NewNoopSeenStore())

	_, err := s.Execute(context.Background(), &db.SyncJob{
		ProviderStreamID: "source-1",
		Provider:         "hackernews",
		JobType:          "fetch_stories",
		Payload: map[string]any{
			"feed":           "topstories",
			"page_size":      1,
			"top_comments_n": 1,
		},
	})
	if err == nil {
		t.Fatal("Execute returned nil, want hydration failure")
	}
}

func TestHackerNewsHydrationReturnsRehydratedRecord(t *testing.T) {
	t.Parallel()

	now := hackerNewsTestNowUnix()
	s := NewHackerNewsSyncerWithClient(&fakeHackerNewsClient{
		items: map[int64]*hackerNewsItem{
			101: hnStory(101, "First story", "https://example.com/first", 120, 42, now, 201),
			201: hnComment(201, 101, "alice", "first comment", now+1),
		},
	}, sync.NewNoopSeenStore())

	result, err := s.Execute(context.Background(), &db.SyncJob{
		ProviderStreamID: "source-1",
		Provider:         "hackernews",
		JobType:          "fetch_hydrated_item",
		Payload: map[string]any{
			"feed":           "topstories",
			"item_id":        101,
			"top_comments_n": 1,
		},
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("record count = %d, want 1", len(result.Records))
	}
	if result.Records[0].ExternalID != "hn:101" {
		t.Fatalf("external id = %q, want hn:101", result.Records[0].ExternalID)
	}
	if got := result.Records[0].Metadata["rehydrated"]; got != true {
		t.Fatalf("rehydrated metadata = %v, want true", got)
	}
	if !result.HasMore {
		t.Fatal("HasMore = false, want warm hydration cycle to remain active")
	}
	if got := result.CursorUpdates["hydration_interval_seconds"]; got == nil {
		t.Fatalf("hydration_interval_seconds missing in cursor updates: %v", result.CursorUpdates)
	}
}

func hnStory(id int64, title string, url string, score int, descendants int, created int64, kids ...int64) *hackerNewsItem {
	return &hackerNewsItem{
		ID:          id,
		Type:        "story",
		By:          "submitter",
		Time:        created,
		Title:       title,
		URL:         url,
		Score:       score,
		Descendants: descendants,
		Kids:        kids,
	}
}

func hnComment(id int64, parent int64, by string, text string, created int64) *hackerNewsItem {
	return &hackerNewsItem{
		ID:     id,
		Type:   "comment",
		By:     by,
		Time:   created,
		Text:   text,
		Parent: parent,
	}
}

func hackerNewsTestNowUnix() int64 {
	return time.Now().UTC().Unix()
}
