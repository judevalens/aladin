package workerapp

import (
	"context"
	"reflect"
	"testing"

	"aladin/backend_v2/internal/insights"
	"aladin/backend_v2/internal/pipeline"

	"github.com/hibiken/asynq"
)

type fakePipelineFamily struct{ called *int }

func (f fakePipelineFamily) Register(mux *asynq.ServeMux) {
	mux.HandleFunc(pipeline.TaskGlobalFirstPass, func(context.Context, *asynq.Task) error {
		*f.called++
		return nil
	})
}
func (fakePipelineFamily) TaskTypes() []string { return []string{pipeline.TaskGlobalFirstPass} }

type fakeSyncFamily struct{ called *int }

func (f fakeSyncFamily) RegisterHandlers(mux *asynq.ServeMux) {
	mux.HandleFunc("sync:reddit", func(context.Context, *asynq.Task) error {
		*f.called++
		return nil
	})
}
func (fakeSyncFamily) Queues() map[string]int { return map[string]int{"sync_head:reddit": 6} }
func (fakeSyncFamily) TaskTypes() []string    { return []string{"sync:reddit"} }

type countHandler struct{ called *int }

func (h countHandler) ProcessTask(context.Context, *asynq.Task) error {
	*h.called++
	return nil
}

func TestTaskRegistryPreservesNamesQueuesAndHandlers(t *testing.T) {
	pipelineCalls, insightCalls, syncCalls := 0, 0, 0
	registry := NewTaskRegistry(fakePipelineFamily{&pipelineCalls}, countHandler{&insightCalls}, fakeSyncFamily{&syncCalls})
	wantTypes := []string{insights.TaskGenerate, pipeline.TaskGlobalFirstPass, "sync:reddit"}
	if got := registry.TaskTypes(); !reflect.DeepEqual(got, wantTypes) {
		t.Fatalf("task types = %v, want %v", got, wantTypes)
	}
	queues := registry.Queues()
	if queues[pipeline.TaskGlobalFirstPass] != 10 || queues[insights.TaskGenerate] != 5 || queues["sync_head:reddit"] != 6 {
		t.Fatalf("queue weights changed: %v", queues)
	}
	for _, taskType := range wantTypes {
		if err := registry.Handler().ProcessTask(context.Background(), asynq.NewTask(taskType, nil)); err != nil {
			t.Fatalf("process %s: %v", taskType, err)
		}
	}
	if pipelineCalls != 1 || insightCalls != 1 || syncCalls != 1 {
		t.Fatalf("handler calls = pipeline %d insight %d sync %d", pipelineCalls, insightCalls, syncCalls)
	}
	queues["mutated"] = 99
	if _, ok := registry.Queues()["mutated"]; ok {
		t.Fatal("Queues exposed mutable registry state")
	}
}
