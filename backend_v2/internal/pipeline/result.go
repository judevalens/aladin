package pipeline

import "context"

// Task type constants — one queue per stage.
const (
	TaskGlobalFirstPass = "pipeline:global_first_pass"
	TaskTenantMatch     = "pipeline:tenant_match"
	TaskFirstPass       = "pipeline:first_pass"
	TaskSearch          = "pipeline:search"
	TaskEmbed           = "pipeline:embed"
	TaskGraph           = "pipeline:graph"
	TaskResolveEntities = "pipeline:resolve_entities"
)

// Result discriminants — used by ResultHandler to route between stages.
const (
	ResultGlobalFirstPassDone   = "global_first_pass.done"
	ResultTenantMatchDone       = "tenant_match.done"
	ResultFirstPassSearchNeeded = "first_pass.search_needed"
	ResultFirstPassEmbedReady   = "first_pass.embed_ready"
	ResultSearchDone            = "search.done"
	ResultEmbedDone             = "embed.done"
	ResultGraphDone             = "graph.done"
	ResultResolveEntitiesDone   = "resolve_entities.done"
)

// Result is the outcome of a pipeline stage.
// Type is the discriminant the ResultHandler switches on for routing.
// Payload is the updated RecordPayload to forward to the next stage.
// TaskType identifies which worker produced this result (used for re-enqueue on error).
// Err is non-nil if the stage failed.
type Result struct {
	Type          string
	TaskType      string
	Payload       []byte
	Err           error
	RecordID      string
	CorrelationID string
	KgID          string
}

// ResultHandler processes the outcome of a pipeline stage.
// A single concrete implementation owns the full routing logic for one pipeline variant.
type ResultHandler interface {
	OnDone(ctx context.Context, result Result) error
}

// Worker is a single pipeline stage.
// It receives the accumulated RecordPayload as bytes, does its work,
// and returns a Result with a discriminant type for routing.
type Worker interface {
	TaskType() string
	Concurrency() int
	Run(ctx context.Context, payload []byte) Result
}
