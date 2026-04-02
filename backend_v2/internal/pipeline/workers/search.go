package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"aladin/backend_v2/internal/pipeline"
	"aladin/backend_v2/internal/search"
)

const searchBackoff = 30 * time.Second

type SearchWorker struct {
	searcher search.Searcher
}

func NewSearchWorker(searcher search.Searcher) *SearchWorker {
	return &SearchWorker{searcher: searcher}
}

func (w *SearchWorker) TaskType()    string { return pipeline.TaskSearch }
func (w *SearchWorker) Concurrency() int    { return 5 }

func (w *SearchWorker) Run(ctx context.Context, raw []byte) pipeline.Result {
	var p pipeline.ArtifactPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return pipeline.Result{
			TaskType: pipeline.TaskSearch,
			Err:      pipeline.ErrPermanent{Cause: fmt.Errorf("unmarshal payload: %w", err)},
		}
	}

	log := slog.With(
		"component", "pipeline",
		"stage", "search",
		"artifact_id", p.ArtifactID,
		"kg_id", p.KgID,
	)
	start := time.Now()

	if len(p.LowConfidenceEntities) == 0 {
		return errResult(pipeline.TaskSearch, p, pipeline.ErrPermanent{Cause: fmt.Errorf("no entities to search")})
	}

	if p.SearchResolved == nil {
		p.SearchResolved = make(map[string][]search.SearchResult)
	}

	log.Debug("search: resolving entities", "pending_count", len(p.LowConfidenceEntities))

	for _, entity := range p.LowConfidenceEntities {
		if _, done := p.SearchResolved[entity]; done {
			continue // already resolved in a previous attempt
		}
		results, err := w.searcher.Search(ctx, entity, 3)
		if err != nil {
			payload, _ := json.Marshal(p)
			var httpErr *search.ErrHTTP
			if errors.As(err, &httpErr) {
				switch {
				case httpErr.IsRateLimit():
					log.Warn("search: rate limited", "entity", entity, "retry_after", searchBackoff)
					return pipeline.Result{
						TaskType:   pipeline.TaskSearch,
						Payload:    payload,
						Err:        pipeline.ErrRateLimit{RetryAfter: searchBackoff},
						ArtifactID: p.ArtifactID,
						KgID:       p.KgID,
					}
				case httpErr.IsPermanent():
					log.Error("search: permanent error", "entity", entity, "status", httpErr.StatusCode)
					return pipeline.Result{
						TaskType:   pipeline.TaskSearch,
						Err:        pipeline.ErrPermanent{Cause: err},
						ArtifactID: p.ArtifactID,
						KgID:       p.KgID,
					}
				}
			}
			// Network error or transient HTTP failure — retry
			log.Warn("search: transient error", "entity", entity, "err", err)
			return pipeline.Result{
				TaskType:   pipeline.TaskSearch,
				Payload:    payload,
				Err:        pipeline.ErrTransient{Cause: err},
				ArtifactID: p.ArtifactID,
				KgID:       p.KgID,
			}
		}
		log.Debug("search: entity resolved", "entity", entity, "result_count", len(results))
		p.SearchResolved[entity] = results
	}

	log.Info("search: complete",
		"resolved_count", len(p.SearchResolved),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	payload, err := json.Marshal(p)
	if err != nil {
		return errResult(pipeline.TaskSearch, p, pipeline.ErrPermanent{Cause: err})
	}

	return pipeline.Result{
		Type:       pipeline.ResultSearchDone,
		TaskType:   pipeline.TaskSearch,
		Payload:    payload,
		ArtifactID: p.ArtifactID,
		KgID:       p.KgID,
	}
}
