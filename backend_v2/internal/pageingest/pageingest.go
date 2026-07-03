// Package pageingest folds authored pages (the "you-stream") into the knowledge engine.
// A page's tags/@mentions ground LLM claim extraction over its text
// (claims.AuthoredExtractor); this package adds the triggers: an ambient idle-sweep
// (auto-ingest a page once it's been quiet ~10 min) and an on-demand path (the manual
// "Connect" button, force=true — Y3).
//
// Supersede-on-new-edit: the queued job is revision-LESS and coalescing (one pending ingest
// per page), and the worker reads LIVE page state at run time with two guards — already-
// ingested (revision) and not-idle (a fresh edit landed after enqueue). So a stale queued
// job never processes an actively-edited page, the 10-min idle window effectively resets on
// every edit, and the latest revision always wins.
package pageingest

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"aladin/backend_v2/internal/repo"
	coreservice "aladin/backend_v2/internal/service"
)

// TaskIngestAuthoredPage is the asynq task type + queue name for an ambient page ingest.
const TaskIngestAuthoredPage = "page:ingest_authored"

// DefaultIdle is how long a page must be quiet before the ambient sweep ingests it.
const DefaultIdle = 10 * time.Minute

// PageStore is the page_documents access the worker needs (satisfied by
// *repo.PostgresArtifactRepository).
type PageStore interface {
	ListPagesDueForIngest(ctx context.Context, idleFor time.Duration, limit int) ([]repo.PageIngestCandidate, error)
	GetPageIngestSnapshot(ctx context.Context, artifactID string) (repo.PageIngestSnapshot, error)
	MarkPageIngested(ctx context.Context, artifactID string, revision int64) error
}

// Extractor runs authored claim extraction for a page (satisfied by
// *claims.AuthoredExtractor).
type Extractor interface {
	ForArtifact(ctx context.Context, artifactID, ownerID, text string) (int, error)
}

type payload struct {
	ArtifactID string `json:"artifact_id"`
}

// Worker is the ambient ingest handler + sweep. It also exposes Ingest for the synchronous
// manual trigger (Y3).
type Worker struct {
	pages     PageStore
	extractor Extractor
	client    *asynq.Client
	idle      time.Duration
}

func NewWorker(pages PageStore, extractor Extractor, client *asynq.Client) *Worker {
	return &Worker{pages: pages, extractor: extractor, client: client, idle: DefaultIdle}
}

// WithIdle overrides the idle window (tests).
func (w *Worker) WithIdle(d time.Duration) *Worker {
	if d > 0 {
		w.idle = d
	}
	return w
}

// Enqueue queues an ambient ingest for a page. The TaskID is revision-LESS and per-page, so a
// re-enqueue while one is pending coalesces (ErrTaskIDConflict → no-op): at most one pending
// ingest per page, always resolving to the page's latest state at run time.
func (w *Worker) Enqueue(ctx context.Context, artifactID string) error {
	if w == nil || w.client == nil || strings.TrimSpace(artifactID) == "" {
		return nil
	}
	body, err := json.Marshal(payload{ArtifactID: artifactID})
	if err != nil {
		return err
	}
	_, err = w.client.EnqueueContext(ctx, asynq.NewTask(TaskIngestAuthoredPage, body),
		asynq.Queue(TaskIngestAuthoredPage),
		asynq.MaxRetry(3),
		asynq.TaskID(artifactID+":ingest_authored_page"),
	)
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}
	return err
}

// Sweep enqueues an ambient ingest for every page that is due (edits past last ingest, idle
// >= the idle window). Returns the number enqueued. Intended to run on a periodic ticker.
func (w *Worker) Sweep(ctx context.Context, limit int) (int, error) {
	due, err := w.pages.ListPagesDueForIngest(ctx, w.idle, limit)
	if err != nil {
		return 0, err
	}
	for _, c := range due {
		if err := w.Enqueue(ctx, c.ArtifactID); err != nil {
			return 0, err
		}
	}
	return len(due), nil
}

// Ingest folds a page's authored claims into the engine. force=true (manual trigger) bypasses
// the idle guard and the already-ingested gate; force=false (ambient) skips pages already
// ingested at their current revision and pages edited within the idle window (a fresh edit
// landed after enqueue → defer to the next sweep). Returns the number of claims stored.
// Idempotent: extraction dedups, and the revision gate makes a repeat run a no-op.
func (w *Worker) Ingest(ctx context.Context, artifactID string, force bool) (int, error) {
	snap, err := w.pages.GetPageIngestSnapshot(ctx, artifactID)
	if err != nil {
		if errors.Is(err, coreservice.ErrNotFound) {
			return 0, nil // page gone; nothing to do
		}
		return 0, err
	}
	if !force {
		if snap.Revision <= snap.LastIngested {
			return 0, nil // already ingested at this revision
		}
		if time.Since(snap.UpdatedAt) < w.idle {
			return 0, nil // not idle: a new edit landed after enqueue — the next sweep retries
		}
	}
	n, err := w.extractor.ForArtifact(ctx, artifactID, snap.OwnerID, snap.Text)
	if err != nil {
		return 0, err
	}
	if err := w.pages.MarkPageIngested(ctx, artifactID, snap.Revision); err != nil {
		return n, err
	}
	return n, nil
}

// ProcessTask is the asynq handler for ambient ingests (never forces).
func (w *Worker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p payload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	_, err := w.Ingest(ctx, p.ArtifactID, false)
	return err
}

// RegisterHandler wires the ambient ingest handler onto the asynq mux.
func RegisterHandler(mux *asynq.ServeMux, w *Worker) {
	mux.Handle(TaskIngestAuthoredPage, w)
}
