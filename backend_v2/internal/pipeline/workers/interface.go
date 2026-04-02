package workers

import (
	"context"

	"github.com/hibiken/asynq"
)

// Each Runner knows its own queue (task type), registers its own handler on the mux,
// and reports results back to the orchestrator via the onDone callback.
// The orchestrator never calls Run directly — it only receives results.

type FirstPassRunner interface {
	Register(mux *asynq.ServeMux, onDone func(context.Context, string, *FirstPassResult, error) error)
}

type SearchRunner interface {
	Register(mux *asynq.ServeMux, onDone func(context.Context, string, *SearchResult, error) error)
}

type EmbedRunner interface {
	Register(mux *asynq.ServeMux, onDone func(context.Context, string, *EmbedResult, error) error)
}

type GraphRunner interface {
	Register(mux *asynq.ServeMux, onDone func(context.Context, string, *GraphResult, error) error)
}
