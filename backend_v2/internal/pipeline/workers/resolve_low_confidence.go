package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/entities"
	"aladin/backend_v2/internal/pipeline"
	"aladin/backend_v2/internal/ratelimit"
	"aladin/backend_v2/internal/websearch"
)

// EntityResolver is the slice of *entities.Resolver this worker needs (so it's fakeable).
type EntityResolver interface {
	Resolve(ctx context.Context, m entities.Mention) (string, error)
}

// ResolveLowConfidenceWorker enriches the entities the first pass was UNSURE about. For each
// record.enrichment.low_confidence_entities surface it web-searches for context, then runs
// the resolver with that context as a hint — so a niche/ambiguous name either resolves to an
// existing canonical entity (a confident or proposed match) or is created with real grounding,
// instead of sitting inert. Sequenced in the entity chain (after resolve_entities) so the record
// advances on a COMPLETE entity set — it can't move on until its low-confidence entities are
// resolved too.
type ResolveLowConfidenceWorker struct {
	records  db.RecordRepository
	resolver EntityResolver
	searcher websearch.Searcher
	limiter  *ratelimit.Limiter
}

func NewResolveLowConfidenceWorker(records db.RecordRepository, resolver EntityResolver, searcher websearch.Searcher, limiter *ratelimit.Limiter) *ResolveLowConfidenceWorker {
	return &ResolveLowConfidenceWorker{records: records, resolver: resolver, searcher: searcher, limiter: limiter}
}

func (w *ResolveLowConfidenceWorker) TaskType() string { return pipeline.TaskResolveLowConfidence }
func (w *ResolveLowConfidenceWorker) Concurrency() int { return 3 }

func (w *ResolveLowConfidenceWorker) Run(ctx context.Context, raw []byte) pipeline.Result {
	var p pipeline.ResolveEntitiesPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return pipeline.Result{
			TaskType: pipeline.TaskResolveLowConfidence,
			Err:      pipeline.ErrPermanent{Cause: fmt.Errorf("resolve_low_confidence: unmarshal payload: %w", err)},
		}
	}
	done := pipeline.Result{
		Type:          pipeline.ResultResolveLowConfidenceDone,
		TaskType:      pipeline.TaskResolveLowConfidence,
		Payload:       raw,
		RecordID:      p.RecordID,
		CorrelationID: p.CorrelationID,
	}
	fail := func(err error) pipeline.Result {
		return pipeline.Result{TaskType: pipeline.TaskResolveLowConfidence, Err: err, RecordID: p.RecordID, CorrelationID: p.CorrelationID}
	}
	if w.searcher == nil {
		return done // search disabled (no key) — no-op
	}

	log := slog.With("component", "pipeline", "stage", "resolve_low_confidence", "record_id", p.RecordID)

	rec, err := w.records.Get(ctx, p.RecordID)
	if err != nil {
		return fail(pipeline.ErrTransient{Cause: fmt.Errorf("resolve_low_confidence: get record: %w", err)})
	}
	if p.SourceRevision > 0 && rec.SourceRevision > p.SourceRevision {
		return done // superseded
	}
	surfaces := rec.Enrichment.LowConfidenceEntities
	if len(surfaces) == 0 {
		return done
	}

	for _, surface := range surfaces {
		surface = strings.TrimSpace(surface)
		if surface == "" {
			continue
		}
		if w.limiter != nil {
			if err := w.limiter.Wait(ctx); err != nil {
				return fail(pipeline.ErrTransient{Cause: err})
			}
		}
		// Web search is best-effort: a miss on one surface shouldn't fail the record — we just
		// resolve it without the extra context (still goes through the normal ladder).
		hint := rec.Enrichment.Summary
		if results, err := w.searcher.Search(ctx, surface, 3); err != nil {
			log.Warn("resolve_low_confidence: search failed, resolving without web context", "surface", surface, "err", err)
		} else if len(results) > 0 {
			hint = searchHint(rec.Enrichment.Summary, results[0])
		}

		if _, err := w.resolver.Resolve(ctx, entities.Mention{
			Surface:        surface,
			RecordID:       rec.ID,
			SourceRevision: rec.SourceRevision,
			ContextHint:    hint,
		}); err != nil {
			return fail(pipeline.ErrTransient{Cause: fmt.Errorf("resolve_low_confidence: resolve %q: %w", surface, err)})
		}
	}
	log.Info("resolve_low_confidence: resolved low-confidence entities", "count", len(surfaces))
	return done
}

// searchHint builds a bounded context hint from the record summary + the top search result,
// to strengthen the resolver's context-embedding match.
func searchHint(summary string, top websearch.SearchResult) string {
	const maxContent = 500
	content := top.Content
	if len(content) > maxContent {
		content = content[:maxContent]
	}
	parts := []string{summary, top.Title, content}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, " — ")
}
