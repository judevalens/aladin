package sync_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"aladin/backend_v2/internal/db"
	syncpkg "aladin/backend_v2/internal/sync"
	"aladin/backend_v2/internal/sync/syncers"
)

type integrationSourceRepo struct {
	source         *db.Source
	markStartedIDs []string
	markSyncedIDs  []string
	markPageIDs    []string
}

func (r *integrationSourceRepo) GetByID(ctx context.Context, id string) (*db.Source, error) {
	return r.source, nil
}

func (r *integrationSourceRepo) ClaimBatch(ctx context.Context, limit int) ([]*db.Source, error) {
	if r.source == nil {
		return nil, nil
	}
	return []*db.Source{r.source}, nil
}

func (r *integrationSourceRepo) MarkSyncStarted(ctx context.Context, id string) error {
	r.markStartedIDs = append(r.markStartedIDs, id)
	return nil
}

func (r *integrationSourceRepo) MarkSyncPage(ctx context.Context, id string, configUpdates map[string]any) error {
	r.markPageIDs = append(r.markPageIDs, id)
	for k, v := range configUpdates {
		r.source.Config[k] = v
	}
	return nil
}

func (r *integrationSourceRepo) MarkSynced(ctx context.Context, id string, configUpdates map[string]any) error {
	r.markSyncedIDs = append(r.markSyncedIDs, id)
	now := time.Now().UTC()
	r.source.LastRefreshAt = &now
	for k, v := range configUpdates {
		r.source.Config[k] = v
	}
	return nil
}

func (r *integrationSourceRepo) MarkSyncFailed(ctx context.Context, id string) error { return nil }
func (r *integrationSourceRepo) Release(ctx context.Context, id string) error        { return nil }

type integrationCycleRepo struct {
	cyclesBySource   map[string][]*db.SyncCycle
	created          []*db.SyncCycle
	updatedIDs       []string
	completedIDs     []string
	completedReasons []string
}

func (r *integrationCycleRepo) ListActiveBySource(ctx context.Context, sourceID string) ([]*db.SyncCycle, error) {
	return r.cyclesBySource[sourceID], nil
}

func (r *integrationCycleRepo) Create(ctx context.Context, cycle *db.SyncCycle) error {
	if cycle.CreatedAt.IsZero() {
		cycle.CreatedAt = time.Now().UTC()
	}
	r.created = append(r.created, cycle)
	r.cyclesBySource[cycle.SourceID] = append(r.cyclesBySource[cycle.SourceID], cycle)
	return nil
}

func (r *integrationCycleRepo) MarkRunning(ctx context.Context, id string) error {
	r.updateCycle(id, func(c *db.SyncCycle) { c.Status = syncpkg.CycleStatusRunning })
	return nil
}

func (r *integrationCycleRepo) UpdateProgress(ctx context.Context, id string, cursor map[string]any, headBoundary map[string]any) error {
	r.updatedIDs = append(r.updatedIDs, id)
	r.updateCycle(id, func(c *db.SyncCycle) {
		c.Status = syncpkg.CycleStatusActive
		c.Cursor = cloneMap(cursor)
		c.HeadBoundary = cloneMap(headBoundary)
	})
	return nil
}

func (r *integrationCycleRepo) MarkActive(ctx context.Context, id string) error {
	r.updateCycle(id, func(c *db.SyncCycle) { c.Status = syncpkg.CycleStatusActive })
	return nil
}

func (r *integrationCycleRepo) Complete(ctx context.Context, id string, headBoundary map[string]any, completionReason string) error {
	r.completedIDs = append(r.completedIDs, id)
	r.completedReasons = append(r.completedReasons, completionReason)
	r.updateCycle(id, func(c *db.SyncCycle) {
		c.Status = syncpkg.CycleStatusComplete
		c.HeadBoundary = cloneMap(headBoundary)
		c.CompletionReason = completionReason
		now := time.Now().UTC()
		c.CompletedAt = &now
	})
	return nil
}

func (r *integrationCycleRepo) updateCycle(id string, fn func(*db.SyncCycle)) {
	for _, cycles := range r.cyclesBySource {
		for _, cycle := range cycles {
			if cycle != nil && cycle.ID == id {
				fn(cycle)
			}
		}
	}
}

type integrationSeenStore struct {
	known map[string]bool
}

func (s *integrationSeenStore) Seen(ctx context.Context, sourceID string, externalIDs []string) (map[string]bool, error) {
	out := make(map[string]bool, len(externalIDs))
	for _, id := range externalIDs {
		if s.known[id] {
			out[id] = true
		}
	}
	return out, nil
}

func (s *integrationSeenStore) MarkSeen(ctx context.Context, sourceID string, externalIDs []string) error {
	for _, id := range externalIDs {
		s.known[id] = true
	}
	return nil
}

type integrationEnqueuer struct{}

func (e *integrationEnqueuer) EnqueueSync(ctx context.Context, queueName, taskType string, payload []byte, maxRetry int, timeout time.Duration) error {
	return nil
}

func (e *integrationEnqueuer) EnqueueFirstPass(ctx context.Context, artifactID string, payload []byte) error {
	return nil
}

type redditFlowFixture struct {
	Pages map[string]redditFlowPage `json:"pages"`
}

type redditFlowPage struct {
	After    string               `json:"after"`
	Children []redditFlowArtifact `json:"children"`
}

type redditFlowArtifact struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Permalink  string  `json:"permalink"`
	Score      int     `json:"score"`
	CreatedUTC float64 `json:"created_utc"`
}

type fixtureRedditClient struct {
	fixture redditFlowFixture
}

func (f fixtureRedditClient) FetchListing(ctx context.Context, state syncers.RedditCycleState) (*syncers.RedditListingResponse, error) {
	page, ok := f.fixture.Pages[state.After]
	if !ok {
		return &syncers.RedditListingResponse{}, nil
	}
	resp := &syncers.RedditListingResponse{}
	resp.Data.After = page.After
	for _, child := range page.Children {
		resp.Data.Children = append(resp.Data.Children, redditChild(child.ID, child.Title, child.Permalink, child.Score, child.CreatedUTC))
	}
	return resp, nil
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func TestOrchestratorWithRealRedditSyncerRefreshTakesOverThenOlderCycleResumes(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	source := &db.Source{
		ID:            "source-1",
		Type:          "reddit",
		KgID:          "kg-1",
		SyncInterval:  300,
		LastRefreshAt: ptrTime(now.Add(-2 * time.Hour)),
		Config: map[string]any{
			"subreddit": "golang",
		},
	}
	olderCycle := &db.SyncCycle{
		ID:        "cycle-old",
		SourceID:  source.ID,
		Kind:      syncpkg.CycleKindRefresh,
		Status:    syncpkg.CycleStatusActive,
		CreatedAt: now.Add(-119 * time.Minute),
		Cursor: map[string]any{
			"subreddit": "golang",
			"after":     "old-page-1",
		},
	}
	sources := &integrationSourceRepo{source: source}
	cycles := &integrationCycleRepo{
		cyclesBySource: map[string][]*db.SyncCycle{
			source.ID: {olderCycle},
		},
	}
	seen := &integrationSeenStore{known: map[string]bool{"seen-1": true}}
	reddit := syncers.NewRedditSyncerWithClient(loadFixtureRedditClient(t, "reddit_refresh_takeover_flow.json"), seen)
	orch := syncpkg.NewOrchestrator(&integrationEnqueuer{}, sources, cycles, seen, syncpkg.NewFreshnessFirstArbiter(), reddit)
	mux := asynq.NewServeMux()
	orch.RegisterHandlers(mux)

	jobs, err := orch.ClaimBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("first ClaimBatch: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("first ClaimBatch jobs = %d, want 1", len(jobs))
	}
	var firstJob db.SyncJob
	if err := json.Unmarshal(jobs[0].Payload, &firstJob); err != nil {
		t.Fatalf("unmarshal first job: %v", err)
	}
	if firstJob.CycleID == olderCycle.ID {
		t.Fatal("first job reused older cycle, want a new refresh cycle")
	}
	if err := mux.ProcessTask(context.Background(), asynq.NewTask(jobs[0].Type, jobs[0].Payload)); err != nil {
		t.Fatalf("process first page: %v", err)
	}
	if len(cycles.updatedIDs) != 1 || cycles.updatedIDs[0] != firstJob.CycleID {
		t.Fatalf("updated cycle ids = %v, want [%s]", cycles.updatedIDs, firstJob.CycleID)
	}

	jobs, err = orch.ClaimBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("second ClaimBatch: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("second ClaimBatch jobs = %d, want 1", len(jobs))
	}
	var secondJob db.SyncJob
	if err := json.Unmarshal(jobs[0].Payload, &secondJob); err != nil {
		t.Fatalf("unmarshal second job: %v", err)
	}
	if secondJob.CycleID != firstJob.CycleID {
		t.Fatalf("second job cycle = %q, want %q", secondJob.CycleID, firstJob.CycleID)
	}
	if err := mux.ProcessTask(context.Background(), asynq.NewTask(jobs[0].Type, jobs[0].Payload)); err != nil {
		t.Fatalf("process second page: %v", err)
	}
	if len(cycles.completedIDs) == 0 || cycles.completedIDs[0] != firstJob.CycleID {
		t.Fatalf("completed cycle ids = %v, want first completion for %s", cycles.completedIDs, firstJob.CycleID)
	}
	if len(cycles.completedReasons) == 0 || cycles.completedReasons[0] != syncpkg.CompletionReasonSeenOverlap {
		t.Fatalf("completion reasons = %v, want first reason %s", cycles.completedReasons, syncpkg.CompletionReasonSeenOverlap)
	}

	jobs, err = orch.ClaimBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("third ClaimBatch: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("third ClaimBatch jobs = %d, want 1", len(jobs))
	}
	var thirdJob db.SyncJob
	if err := json.Unmarshal(jobs[0].Payload, &thirdJob); err != nil {
		t.Fatalf("unmarshal third job: %v", err)
	}
	if thirdJob.CycleID != olderCycle.ID {
		t.Fatalf("third job cycle = %q, want %q", thirdJob.CycleID, olderCycle.ID)
	}
	if err := mux.ProcessTask(context.Background(), asynq.NewTask(jobs[0].Type, jobs[0].Payload)); err != nil {
		t.Fatalf("process older cycle page: %v", err)
	}
	if got := cycles.completedReasons[len(cycles.completedReasons)-1]; got != syncpkg.CompletionReasonExhausted {
		t.Fatalf("final completion reason = %q, want %q", got, syncpkg.CompletionReasonExhausted)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func loadFixtureRedditClient(t *testing.T, name string) syncers.RedditClient {
	t.Helper()

	path := filepath.Join("testdata", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}

	var fixture redditFlowFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", path, err)
	}
	return fixtureRedditClient{fixture: fixture}
}

func redditChild(id, title, permalink string, score int, createdUTC float64) struct {
	Data struct {
		ID         string  `json:"id"`
		Title      string  `json:"title"`
		Selftext   string  `json:"selftext"`
		Permalink  string  `json:"permalink"`
		Score      int     `json:"score"`
		CreatedUTC float64 `json:"created_utc"`
	} `json:"data"`
} {
	var child struct {
		Data struct {
			ID         string  `json:"id"`
			Title      string  `json:"title"`
			Selftext   string  `json:"selftext"`
			Permalink  string  `json:"permalink"`
			Score      int     `json:"score"`
			CreatedUTC float64 `json:"created_utc"`
		} `json:"data"`
	}
	child.Data.ID = id
	child.Data.Title = title
	child.Data.Permalink = permalink
	child.Data.Score = score
	child.Data.CreatedUTC = createdUTC
	return child
}
