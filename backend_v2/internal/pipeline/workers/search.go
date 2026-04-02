package workers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"

	"aladin/backend_v2/internal/pipeline/blackboard"
	"aladin/backend_v2/internal/search"
)

const (
	TaskSearch    = "pipeline:search"
	searchBackoff = 30 * time.Second
)

type SearchResult struct {
	Resolved map[string][]search.SearchResult
}

type SearchWorker struct {
	board    *blackboard.Blackboard
	searcher search.Searcher
}

func NewSearchWorker(
	board *blackboard.Blackboard,
	searcher search.Searcher,
) *SearchWorker {
	return &SearchWorker{
		board:    board,
		searcher: searcher,
	}
}

func (w *SearchWorker) Register(mux *asynq.ServeMux, onDone func(context.Context, string, *SearchResult, error) error) {
	mux.HandleFunc(TaskSearch, func(ctx context.Context, t *asynq.Task) error {
		id := parseArtifactID(t.Payload())
		result, err := w.Run(ctx, id)
		return onDone(ctx, id, result, err)
	})
}

func (w *SearchWorker) Run(ctx context.Context, artifactID string) (*SearchResult, error) {
	entry, err := w.board.Get(ctx, artifactID)
	if err != nil {
		return nil, ErrTransient{Cause: err}
	}
	if entry == nil {
		return nil, ErrPermanent{Cause: fmt.Errorf("entry not found: %s", artifactID)}
	}

	log := slog.With(
		"component", "pipeline",
		"stage", "search",
		"artifact_id", entry.ArtifactID,
		"kg_id", entry.KgID,
	)
	start := time.Now()

	if entry.Search == nil {
		return nil, ErrPermanent{Cause: fmt.Errorf("search state missing from entry")}
	}

	log.Debug("search: resolving entities", "pending_count", len(entry.Search.Pending))

	for _, entity := range entry.Search.Pending {
		results, err := w.searcher.Search(ctx, entity, 3)
		if err != nil {
			log.Warn("search: rate limited",
				"entity", entity,
				"retry_after", searchBackoff,
			)
			// Write partial progress before returning so the next run picks up where it left off
			_ = w.board.Set(ctx, entry)
			return nil, ErrRateLimit{RetryAfter: searchBackoff}
		}
		log.Debug("search: entity resolved", "entity", entity, "result_count", len(results))
		entry.Search.Resolved[entity] = results
		entry.Search.Pending = remove(entry.Search.Pending, entity)
	}

	log.Info("search: complete",
		"resolved_count", len(entry.Search.Resolved),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return &SearchResult{Resolved: entry.Search.Resolved}, nil
}

func remove(slice []string, item string) []string {
	out := slice[:0]
	for _, s := range slice {
		if s != item {
			out = append(out, s)
		}
	}
	return out
}
