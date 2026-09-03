package pipeline

import (
	"context"
	"log/slog"
	"sort"

	"github.com/hibiken/asynq"
)

// Orchestrator is a generic pipeline runner.
// It holds a registry of Workers and delegates routing to a ResultHandler.
// It knows nothing about what workers do or how the pipeline is shaped —
// that logic lives entirely in the ResultHandler.
type Orchestrator struct {
	workers map[string]Worker
	handler ResultHandler
}

// TaskTypes returns the stable task names owned by this pipeline family.
// It is used by process composition to inventory registrations without
// reaching into the orchestrator's handler map.
func (o *Orchestrator) TaskTypes() []string {
	types := make([]string, 0, len(o.workers))
	for taskType := range o.workers {
		types = append(types, taskType)
	}
	sort.Strings(types)
	return types
}

func NewOrchestrator(handler ResultHandler) *Orchestrator {
	return &Orchestrator{
		workers: make(map[string]Worker),
		handler: handler,
	}
}

// Add registers a worker. Call once per stage at startup.
func (o *Orchestrator) Add(w Worker) {
	o.workers[w.TaskType()] = w
}

// Register wires all workers onto the mux.
func (o *Orchestrator) Register(mux *asynq.ServeMux) {
	for _, w := range o.workers {
		w := w
		mux.HandleFunc(w.TaskType(), func(ctx context.Context, t *asynq.Task) error {
			result := w.Run(ctx, t.Payload())
			result.TaskType = w.TaskType()
			return o.handler.OnDone(ctx, result)
		})
		slog.Info("orchestrator: worker registered",
			"component", "orchestrator",
			"task_type", w.TaskType(),
			"concurrency", w.Concurrency(),
		)
	}
}
