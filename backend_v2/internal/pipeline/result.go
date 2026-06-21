package pipeline

import "context"

// Task type constants — one queue per stage.
const (
	TaskGlobalFirstPass = "pipeline:global_first_pass"
	TaskTenantMatch     = "pipeline:tenant_match"
	TaskEmbed           = "pipeline:embed"
	TaskResolveEntities      = "pipeline:resolve_entities"
	TaskResolveClaims        = "pipeline:resolve_claims"
	TaskResolveLowConfidence = "pipeline:resolve_low_confidence"
	TaskGraphProject         = "pipeline:graph"
)

// Result discriminants — used by ResultHandler to route between stages.
const (
	ResultGlobalFirstPassDone = "global_first_pass.done"
	ResultTenantMatchDone     = "tenant_match.done"
	ResultEmbedDone           = "embed.done"
	ResultResolveEntitiesDone      = "resolve_entities.done"
	ResultResolveClaimsDone        = "resolve_claims.done"
	ResultResolveLowConfidenceDone = "resolve_low_confidence.done"
	ResultGraphProjectDone         = "graph.done"
)

// Result is the outcome of a pipeline stage.
// Type is the discriminant the ResultHandler switches on for routing.
// Payload is the (lean) stage payload to forward to the next stage.
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
// It receives its stage payload as bytes, does its work,
// and returns a Result with a discriminant type for routing.
type Worker interface {
	TaskType() string
	Concurrency() int
	Run(ctx context.Context, payload []byte) Result
}
