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

// NewAsynqErrorHandler returns the worker error hook. It logs every handler error, and on retry
// exhaustion mutates terminal state for the two task families that own DB status: `sync:` tasks
// (provider-stream / cycle state) and `pipeline:` tasks (the record status).
func NewAsynqErrorHandler(records db.RecordRepository, streams db.ProviderStreamRepository, cycles db.SyncCycleRepository) asynq.ErrorHandler {
	return asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
		retryCount, retryOK := asynq.GetRetryCount(ctx)
		maxRetry, maxRetryOK := asynq.GetMaxRetry(ctx)
		taskID, _ := asynq.GetTaskID(ctx)
		handleAsynqTaskError(ctx, task, taskID, err, retryCount, maxRetry, retryOK && maxRetryOK, records, streams, cycles)
	})
}

func handleAsynqTaskError(
	ctx context.Context,
	task *asynq.Task,
	taskID string,
	taskErr error,
	retryCount int,
	maxRetry int,
	hasRetryMetadata bool,
	records db.RecordRepository,
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

	isPipeline := strings.HasPrefix(task.Type(), "pipeline:")
	isSync := strings.HasPrefix(task.Type(), "sync:")
	if !isPipeline && !isSync {
		return
	}
	if !hasRetryMetadata {
		log.Warn("task failure missing retry metadata; skipping terminal DB update")
		return
	}
	if retryCount < maxRetry {
		return
	}

	if isPipeline {
		// Pipeline stage tasks are keyed `recordID:pipeline:<stage>`. On exhaustion, mark the record
		// terminally failed — otherwise it lingers in a non-terminal status (invisible except as a
		// 15-min "stuck" row), throughput looks healthy, and there's no dead-letter surface.
		recordID := taskID
		if i := strings.IndexByte(taskID, ':'); i >= 0 {
			recordID = taskID[:i]
		}
		if recordID == "" {
			log.Warn("pipeline: task failure missing task id; can't mark record failed")
			return
		}
		if err := records.MarkFailed(ctx, recordID, "pipeline_task_retry_exhausted:"+task.Type()); err != nil {
			log.Error("pipeline: mark record failed", "record_id", recordID, "err", err)
			return
		}
		log.Error("pipeline: task retry exhausted; marked record failed", "record_id", recordID, "err", taskErr)
		return
	}

	// isSync
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
