package blackboard

import (
	"time"

	"aladin/backend_v2/internal/search"
)

type State string

const (
	StatePending          State = "pending"
	StateFirstPassRunning State = "first_pass_running"
	StateFirstPassDone    State = "first_pass_done"
	StateSearching        State = "searching"
	StateSearchBlocked    State = "search_blocked"
	StateEmbeddingPending State = "embedding_pending"
	StateEmbeddingRunning State = "embedding_running"
	StateGraphPending     State = "graph_pending"
	StateGraphRunning     State = "graph_running"
	StateComplete         State = "complete"

	StateFirstPassFailed State = "first_pass_failed"
	StateSearchFailed    State = "search_failed"
	StateEmbeddingFailed State = "embedding_failed"
	StateGraphFailed     State = "graph_failed"
)

// SyncedArtifact holds the raw content fetched by a syncer.
// It lives in Redis until the pipeline completes, then is written to PG once.
type SyncedArtifact struct {
	ExternalID string         `json:"external_id"`
	SourceID   string         `json:"source_id"`
	Type       string         `json:"type"`
	Label      string         `json:"label"`
	Content    string         `json:"content"`
	SourceURL  string         `json:"source_url"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type Entry struct {
	ArtifactID  string     `json:"artifact_id"`
	KgID        string     `json:"kg_id"`
	State       State      `json:"state"`
	Attempts    int        `json:"attempts"`
	MaxAttempts int        `json:"max_attempts"`
	RetryAt     *time.Time `json:"retry_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	Raw       *SyncedArtifact  `json:"raw,omitempty"`
	FirstPass *FirstPassResult `json:"first_pass,omitempty"`
	Search    *SearchState     `json:"search,omitempty"`
	Embedding []float32        `json:"embedding,omitempty"`
}

type FirstPassResult struct {
	Summary               string   `json:"summary"`
	Entities              []string `json:"entities"`
	Topics                []string `json:"topics"`
	KeyClaims             []string `json:"key_claims"`
	LowConfidenceEntities []string `json:"low_confidence_entities"`
}

type SearchState struct {
	Pending      []string                        `json:"pending"`
	Resolved     map[string][]search.SearchResult `json:"resolved"` // entity → full results
	BlockedUntil *time.Time                      `json:"blocked_until"`
}

// FailedState returns the corresponding failed state for a given running state.
func FailedState(s State) State {
	switch s {
	case StateFirstPassRunning:
		return StateFirstPassFailed
	case StateSearching:
		return StateSearchFailed
	case StateEmbeddingRunning:
		return StateEmbeddingFailed
	case StateGraphRunning:
		return StateGraphFailed
	default:
		return StateFirstPassFailed
	}
}

// PreviousState returns the state to retry from after a failure.
func PreviousState(s State) State {
	switch s {
	case StateFirstPassFailed:
		return StatePending
	case StateSearchFailed:
		return StateSearching
	case StateEmbeddingFailed:
		return StateEmbeddingPending
	case StateGraphFailed:
		return StateGraphPending
	default:
		return StatePending
	}
}
