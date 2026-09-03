package workerapp

import (
	"sort"

	"aladin/backend_v2/internal/insights"
	"aladin/backend_v2/internal/pipeline"

	"github.com/hibiken/asynq"
)

type pipelineTaskFamily interface {
	Register(*asynq.ServeMux)
	TaskTypes() []string
}

type syncTaskFamily interface {
	RegisterHandlers(*asynq.ServeMux)
	Queues() map[string]int
	TaskTypes() []string
}

// TaskRegistry is the one process-owned mapping of stable task names to handlers
// and queue weights. Producers continue to own payload encoding and idempotency.
type TaskRegistry struct {
	mux       *asynq.ServeMux
	queues    map[string]int
	taskTypes []string
}

func NewTaskRegistry(pipelineFamily pipelineTaskFamily, insightHandler asynq.Handler, syncFamily syncTaskFamily) *TaskRegistry {
	mux := asynq.NewServeMux()
	pipelineFamily.Register(mux)
	mux.Handle(insights.TaskGenerate, insightHandler)
	syncFamily.RegisterHandlers(mux)

	queues := map[string]int{
		pipeline.TaskGlobalFirstPass:      10,
		pipeline.TaskTenantMatch:          10,
		pipeline.TaskEmbed:                3,
		pipeline.TaskResolveEntities:      5,
		pipeline.TaskResolveLowConfidence: 3,
		pipeline.TaskGraphProject:         3,
		insights.TaskGenerate:             5,
	}
	for name, weight := range syncFamily.Queues() {
		queues[name] = weight
	}

	taskTypes := append([]string{}, pipelineFamily.TaskTypes()...)
	taskTypes = append(taskTypes, insights.TaskGenerate)
	taskTypes = append(taskTypes, syncFamily.TaskTypes()...)
	sort.Strings(taskTypes)
	return &TaskRegistry{mux: mux, queues: queues, taskTypes: taskTypes}
}

func (r *TaskRegistry) Handler() asynq.Handler { return r.mux }

func (r *TaskRegistry) Queues() map[string]int {
	out := make(map[string]int, len(r.queues))
	for name, weight := range r.queues {
		out[name] = weight
	}
	return out
}

func (r *TaskRegistry) TaskTypes() []string { return append([]string{}, r.taskTypes...) }
