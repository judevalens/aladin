package sync

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/hibiken/asynq"

	"aladin/backend_v2/internal/db"
)

const CompletionReasonTaskRetryExhausted = "task_retry_exhausted"

// NewAsynqErrorHandler returns the sync-aware worker error hook. It logs every
// handler error, but only mutates sync DB state when Asynq has exhausted retries.
func NewAsynqErrorHandler(streams db.ProviderStreamRepository, cycles db.SyncCycleRepository) asynq.ErrorHandler {
	return asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
		retryCount, retryOK := asynq.GetRetryCount(ctx)
		maxRetry, maxRetryOK := asynq.GetMaxRetry(ctx)
		handleAsynqTaskError(ctx, task, err, retryCount, maxRetry, retryOK && maxRetryOK, streams, cycles)
	})
}

func handleAsynqTaskError(
	ctx context.Context,
	task *asynq.Task,
	taskErr error,
	retryCount int,
	maxRetry int,
	hasRetryMetadata bool,
	streams db.ProviderStreamRepository,
	cycles db.SyncCycleRepository,
) {
	log := slog.With(
		"component", "worker",
		"task_type", task.Type(),
		"retry_count", retryCount,
		"max_retry", maxRetry,
	)
	log.Error("asynq task failed", "err", taskErr)

	if !strings.HasPrefix(task.Type(), "sync:") {
		return
	}
	if !hasRetryMetadata {
		log.Warn("sync: task failure missing retry metadata; skipping terminal DB update")
		return
	}
	if retryCount < maxRetry {
		return
	}

	var job db.SyncJob
	if err := json.Unmarshal(task.Payload(), &job); err != nil {
		log.Error("sync: decode failed task payload", "err", err)
		return
	}
	log = log.With(
		"provider", job.Provider,
		"provider_stream_id", job.ProviderStreamID,
		"cycle_id", job.CycleID,
		"correlation_id", job.CorrelationID,
	)

	if job.CycleID != "" {
		if err := cycles.MarkFailed(ctx, job.CycleID, CompletionReasonTaskRetryExhausted); err != nil {
			log.Error("sync: mark exhausted cycle failed", "err", err)
		}
	}
	if job.ProviderStreamID != "" {
		if err := streams.MarkSyncFailed(ctx, job.ProviderStreamID); err != nil {
			log.Error("sync: mark exhausted provider stream failed", "err", err)
		}
	}
	log.Error("sync: task retry exhausted; marked sync state failed", "err", taskErr)
}
