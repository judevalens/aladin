package sync

import (
	"context"
	"errors"
	"time"

	"github.com/hibiken/asynq"
)

// Enqueuer hides asynq-specific sync and pipeline handoff policy from the sync queue.
type Enqueuer interface {
	EnqueueSync(ctx context.Context, taskType string, payload []byte, maxRetry int, timeout time.Duration) error
	EnqueueFirstPass(ctx context.Context, artifactID string, payload []byte) error
}

type asynqEnqueuer struct {
	client *asynq.Client
}

func NewAsynqEnqueuer(client *asynq.Client) Enqueuer {
	return &asynqEnqueuer{client: client}
}

func (e *asynqEnqueuer) EnqueueSync(ctx context.Context, taskType string, payload []byte, maxRetry int, timeout time.Duration) error {
	task := asynq.NewTask(taskType, payload)
	_, err := e.client.EnqueueContext(ctx, task,
		asynq.Queue(QueueSync),
		asynq.MaxRetry(maxRetry),
		asynq.Timeout(timeout),
	)
	return err
}

func (e *asynqEnqueuer) EnqueueFirstPass(ctx context.Context, artifactID string, payload []byte) error {
	task := asynq.NewTask(pipelineTaskFirstPass, payload)
	_, err := e.client.EnqueueContext(ctx, task,
		asynq.Queue(pipelineTaskFirstPass),
		asynq.MaxRetry(3),
		asynq.TaskID(artifactID+":"+pipelineTaskFirstPass),
	)
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}
	return err
}

const pipelineTaskFirstPass = "pipeline:first_pass"
