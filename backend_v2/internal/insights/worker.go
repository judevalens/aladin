package insights

import (
	"context"
	"log/slog"
)

// Worker consumes kg_ids from a channel and runs insight generation for each.
type Worker struct {
	gen     *Generator
	kgIDsCh <-chan string
}

func NewWorker(gen *Generator, kgIDs <-chan string) *Worker {
	return &Worker{gen: gen, kgIDsCh: kgIDs}
}

func (w *Worker) Start(ctx context.Context) {
	go w.run(ctx)
	slog.Info("insights: worker started", "component", "insights")
}

func (w *Worker) run(ctx context.Context) {
	for {
		pending := w.drain(ctx)
		if len(pending) == 0 {
			return
		}
		for kgID := range pending {
			count, err := w.gen.GenerateAndStore(ctx, kgID)
			if err != nil {
				slog.Error("insights: generation failed", "component", "insights", "kg_id", kgID, "err", err)
				continue
			}
			if count > 0 {
				slog.Info("insights: generated", "component", "insights", "kg_id", kgID, "count", count)
			}
		}
	}
}

func (w *Worker) drain(ctx context.Context) map[string]struct{} {
	ids := make(map[string]struct{})
	select {
	case <-ctx.Done():
		return ids
	case id := <-w.kgIDsCh:
		ids[id] = struct{}{}
	}
	for {
		select {
		case id := <-w.kgIDsCh:
			ids[id] = struct{}{}
		default:
			return ids
		}
	}
}
