package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/pipeline/blackboard"
	"aladin/backend_v2/internal/pipeline/workers"
	"aladin/backend_v2/internal/search"
)

type Orchestrator struct {
	board     *blackboard.Blackboard
	artifacts db.ArtifactRepository
	insights  chan<- string
	client    *asynq.Client
	firstPass workers.FirstPassRunner
	search    workers.SearchRunner
	embed     workers.EmbedRunner
	graph     workers.GraphRunner
}

func NewOrchestrator(
	board *blackboard.Blackboard,
	artifacts db.ArtifactRepository,
	insights chan<- string,
	client *asynq.Client,
	firstPass workers.FirstPassRunner,
	search workers.SearchRunner,
	embed workers.EmbedRunner,
	graph workers.GraphRunner,
) *Orchestrator {
	return &Orchestrator{
		board:     board,
		artifacts: artifacts,
		insights:  insights,
		client:    client,
		firstPass: firstPass,
		search:    search,
		embed:     embed,
		graph:     graph,
	}
}

// Start tells each worker to register its own handler on the mux.
// Workers own their queue — they dequeue, process, and call back here with results.
// The orchestrator never processes artifacts; it only routes and persists.
func (o *Orchestrator) Start(mux *asynq.ServeMux) {
	o.firstPass.Register(mux, o.onFirstPassDone)
	o.search.Register(mux, o.onSearchDone)
	o.embed.Register(mux, o.onEmbedDone)
	o.graph.Register(mux, o.onGraphDone)
}

// EnqueueFirstPass is called by IngestHandler to kick off the pipeline.
func (o *Orchestrator) EnqueueFirstPass(ctx context.Context, artifactID string) error {
	return o.enqueueStage(workers.TaskFirstPass, artifactID)
}

func (o *Orchestrator) onFirstPassDone(ctx context.Context, artifactID string, result *workers.FirstPassResult, err error) error {
	entry, err2 := o.loadEntry(ctx, artifactID, "first_pass")
	if err2 != nil {
		return err2
	}
	if err != nil {
		return o.handleError(ctx, entry, workers.TaskFirstPass, err)
	}

	entry.FirstPass = &blackboard.FirstPassResult{
		Summary:               result.Summary,
		Entities:              result.Entities,
		Topics:                result.Topics,
		KeyClaims:             result.KeyClaims,
		LowConfidenceEntities: result.LowConfidenceEntities,
	}

	if len(result.LowConfidenceEntities) == 0 {
		entry.State = blackboard.StateEmbeddingPending
		if err := o.board.Set(ctx, entry); err != nil {
			return err
		}
		return o.enqueueStage(workers.TaskEmbed, artifactID)
	}

	entry.Search = &blackboard.SearchState{
		Pending:  result.LowConfidenceEntities,
		Resolved: make(map[string][]search.SearchResult),
	}
	entry.State = blackboard.StateSearching
	if err := o.board.Set(ctx, entry); err != nil {
		return err
	}
	return o.enqueueStage(workers.TaskSearch, artifactID)
}

func (o *Orchestrator) onSearchDone(ctx context.Context, artifactID string, result *workers.SearchResult, err error) error {
	entry, err2 := o.loadEntry(ctx, artifactID, "search")
	if err2 != nil {
		return err2
	}
	if err != nil {
		return o.handleError(ctx, entry, workers.TaskSearch, err)
	}

	if entry.Search == nil {
		entry.Search = &blackboard.SearchState{
			Resolved: make(map[string][]search.SearchResult),
		}
	}
	entry.Search.Resolved = result.Resolved
	entry.Search.Pending = nil
	entry.State = blackboard.StateEmbeddingPending
	if err := o.board.Set(ctx, entry); err != nil {
		return err
	}
	return o.enqueueStage(workers.TaskEmbed, artifactID)
}

func (o *Orchestrator) onEmbedDone(ctx context.Context, artifactID string, result *workers.EmbedResult, err error) error {
	entry, err2 := o.loadEntry(ctx, artifactID, "embed")
	if err2 != nil {
		return err2
	}
	if err != nil {
		return o.handleError(ctx, entry, workers.TaskEmbed, err)
	}

	entry.Embedding = result.Embedding
	entry.State = blackboard.StateGraphPending
	if err := o.board.Set(ctx, entry); err != nil {
		return err
	}
	return o.enqueueStage(workers.TaskGraph, artifactID)
}

func (o *Orchestrator) onGraphDone(ctx context.Context, artifactID string, _ *workers.GraphResult, err error) error {
	entry, err2 := o.loadEntry(ctx, artifactID, "graph")
	if err2 != nil {
		return err2
	}
	if err != nil {
		return o.handleError(ctx, entry, workers.TaskGraph, err)
	}
	return o.persist(ctx, entry)
}

func (o *Orchestrator) loadEntry(ctx context.Context, artifactID, stage string) (*blackboard.Entry, error) {
	entry, err := o.board.Get(ctx, artifactID)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		slog.Warn("orchestrator: entry not found, skipping",
			"component", "orchestrator",
			"artifact_id", artifactID,
			"stage", stage,
		)
		return nil, fmt.Errorf("entry not found: %s", artifactID)
	}
	return entry, nil
}

// Recover re-enqueues in-flight entries from Redis on startup.
// asynq.Unique prevents duplicates if the task is already queued.
func (o *Orchestrator) Recover(ctx context.Context) error {
	entries, err := o.board.All(ctx)
	if err != nil {
		return fmt.Errorf("orchestrator recover: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}
	slog.Info("orchestrator: recovering in-flight entries", "component", "orchestrator", "count", len(entries))
	for _, entry := range entries {
		taskType := stageForState(entry.State)
		if taskType == "" {
			slog.Warn("orchestrator: unrecoverable state, skipping",
				"component", "orchestrator",
				"artifact_id", entry.ArtifactID,
				"state", entry.State,
			)
			continue
		}
		if err := o.enqueueStage(taskType, entry.ArtifactID); err != nil {
			slog.Warn("orchestrator: recover enqueue failed (may be duplicate)",
				"component", "orchestrator",
				"artifact_id", entry.ArtifactID,
				"task", taskType,
				"err", err,
			)
		}
	}
	return nil
}

func stageForState(state blackboard.State) string {
	switch state {
	case blackboard.StatePending,
		blackboard.StateFirstPassRunning,
		blackboard.StateFirstPassDone:
		return workers.TaskFirstPass
	case blackboard.StateSearching,
		blackboard.StateSearchBlocked:
		return workers.TaskSearch
	case blackboard.StateEmbeddingPending,
		blackboard.StateEmbeddingRunning:
		return workers.TaskEmbed
	case blackboard.StateGraphPending,
		blackboard.StateGraphRunning:
		return workers.TaskGraph
	default:
		return ""
	}
}

func (o *Orchestrator) handleError(ctx context.Context, entry *blackboard.Entry, taskType string, err error) error {
	log := slog.With("component", "orchestrator", "artifact_id", entry.ArtifactID, "task", taskType)

	var rateLimitErr workers.ErrRateLimit
	var transientErr workers.ErrTransient
	var permanentErr workers.ErrPermanent

	switch {
	case errors.As(err, &rateLimitErr):
		log.Warn("orchestrator: rate limited",
			"retry_after", rateLimitErr.RetryAfter,
			"err", err,
		)
		payload, _ := json.Marshal(struct {
			ArtifactID string `json:"artifact_id"`
		}{entry.ArtifactID})
		task := asynq.NewTask(taskType, payload)
		_, enqErr := o.client.Enqueue(task,
			asynq.ProcessIn(rateLimitErr.RetryAfter),
			asynq.MaxRetry(0),
			asynq.Unique(10*time.Minute),
		)
		if enqErr != nil {
			log.Error("orchestrator: rate-limit enqueue failed", "err", enqErr)
		}

	case errors.As(err, &transientErr):
		entry.Attempts++
		backoff := exponentialBackoff(entry.Attempts)
		log.Warn("orchestrator: transient error",
			"attempts", entry.Attempts,
			"backoff", backoff,
			"err", err,
		)
		if entry.Attempts >= entry.MaxAttempts {
			log.Error("orchestrator: max attempts reached, dropping",
				"max_attempts", entry.MaxAttempts,
			)
			_ = o.board.Delete(ctx, entry.ArtifactID)
			return nil
		}
		_ = o.board.Set(ctx, entry)
		payload, _ := json.Marshal(struct {
			ArtifactID string `json:"artifact_id"`
		}{entry.ArtifactID})
		task := asynq.NewTask(taskType, payload)
		_, enqErr := o.client.Enqueue(task,
			asynq.ProcessIn(backoff),
			asynq.MaxRetry(0),
			asynq.Unique(10*time.Minute),
		)
		if enqErr != nil {
			log.Error("orchestrator: transient retry enqueue failed", "err", enqErr)
		}

	case errors.As(err, &permanentErr):
		log.Error("orchestrator: permanent error, dropping", "err", err)
		_ = o.board.Delete(ctx, entry.ArtifactID)

	default:
		entry.Attempts++
		backoff := exponentialBackoff(entry.Attempts)
		log.Warn("orchestrator: unknown error, treating as transient",
			"attempts", entry.Attempts,
			"backoff", backoff,
			"err", err,
		)
		if entry.Attempts >= entry.MaxAttempts {
			log.Error("orchestrator: max attempts reached, dropping")
			_ = o.board.Delete(ctx, entry.ArtifactID)
			return nil
		}
		_ = o.board.Set(ctx, entry)
		payload, _ := json.Marshal(struct {
			ArtifactID string `json:"artifact_id"`
		}{entry.ArtifactID})
		task := asynq.NewTask(taskType, payload)
		_, enqErr := o.client.Enqueue(task,
			asynq.ProcessIn(backoff),
			asynq.MaxRetry(0),
			asynq.Unique(10*time.Minute),
		)
		if enqErr != nil {
			log.Error("orchestrator: unknown retry enqueue failed", "err", enqErr)
		}
	}

	return nil
}

func (o *Orchestrator) enqueueStage(taskType, artifactID string) error {
	payload, _ := json.Marshal(struct {
		ArtifactID string `json:"artifact_id"`
	}{artifactID})
	task := asynq.NewTask(taskType, payload)
	_, err := o.client.Enqueue(task,
		asynq.MaxRetry(0),
		asynq.Unique(10*time.Minute),
	)
	return err
}

// persist writes the completed artifact to PG and signals the insight worker.
func (o *Orchestrator) persist(ctx context.Context, entry *blackboard.Entry) error {
	log := slog.With("component", "orchestrator", "artifact_id", entry.ArtifactID, "kg_id", entry.KgID)

	if entry.Raw == nil {
		log.Error("orchestrator: complete entry missing raw artifact, dropping")
		_ = o.board.Delete(ctx, entry.ArtifactID)
		return nil
	}

	enrichment := buildEnrichmentJSON(entry)
	a := &db.CompletedArtifact{
		ID:         entry.ArtifactID,
		ExternalID: entry.Raw.ExternalID,
		SourceID:   entry.Raw.SourceID,
		Type:       entry.Raw.Type,
		Label:      entry.Raw.Label,
		Content:    entry.Raw.Content,
		SourceURL:  entry.Raw.SourceURL,
		Metadata:   entry.Raw.Metadata,
		Enrichment: enrichment,
		Embedding:  entry.Embedding,
	}

	if err := o.artifacts.SaveComplete(ctx, a); err != nil {
		log.Error("orchestrator: save complete failed", "err", err)
		return err
	}

	select {
	case o.insights <- entry.KgID:
	default:
	}

	_ = o.board.Delete(ctx, entry.ArtifactID)
	log.Info("orchestrator: artifact complete",
		"external_id", entry.Raw.ExternalID,
		"source_id", entry.Raw.SourceID,
	)
	return nil
}

func buildEnrichmentJSON(entry *blackboard.Entry) []byte {
	type doc struct {
		Summary       string                           `json:"summary,omitempty"`
		Entities      []string                         `json:"entities,omitempty"`
		Topics        []string                         `json:"topics,omitempty"`
		KeyClaims     []string                         `json:"key_claims,omitempty"`
		SearchContext map[string][]search.SearchResult `json:"search_context,omitempty"`
	}
	d := doc{}
	if entry.FirstPass != nil {
		d.Summary   = entry.FirstPass.Summary
		d.Entities  = entry.FirstPass.Entities
		d.Topics    = entry.FirstPass.Topics
		d.KeyClaims = entry.FirstPass.KeyClaims
	}
	if entry.Search != nil {
		d.SearchContext = entry.Search.Resolved
	}
	data, _ := json.Marshal(d)
	return data
}

func exponentialBackoff(attempts int) time.Duration {
	seconds := 30
	for i := 0; i < attempts; i++ {
		seconds *= 4
	}
	if seconds > 3600 {
		seconds = 3600
	}
	return time.Duration(seconds) * time.Second
}
