package sync

import (
	"context"
	"errors"
	"time"

	"github.com/hibiken/asynq"
)

// Enqueuer hides asynq-specific sync and pipeline handoff policy from the sync queue.
type Enqueuer interface {
	EnqueueSync(ctx context.Context, queueName, taskType string, payload []byte, maxRetry int, timeout time.Duration) error
	EnqueueGlobalFirstPass(ctx context.Context, recordID string, payload []byte) error
}

type asynqEnqueuer struct {
	client *asynq.Client
}

func NewAsynqEnqueuer(client *asynq.Client) Enqueuer {
	return &asynqEnqueuer{client: client}
}

func (e *asynqEnqueuer) EnqueueSync(ctx context.Context, queueName, taskType string, payload []byte, maxRetry int, timeout time.Duration) error {
	task := asynq.NewTask(taskType, payload)
	_, err := e.client.EnqueueContext(ctx, task,
		asynq.Queue(queueName),
		asynq.MaxRetry(maxRetry),
		asynq.Timeout(timeout),
	)
	return err
}

func (e *asynqEnqueuer) EnqueueGlobalFirstPass(ctx context.Context, recordID string, payload []byte) error {
	task := asynq.NewTask(pipelineTaskGlobalFirstPass, payload)
	_, err := e.client.EnqueueContext(ctx, task,
		asynq.Queue(pipelineTaskGlobalFirstPass),
		asynq.MaxRetry(3),
		asynq.TaskID(recordID+":"+pipelineTaskGlobalFirstPass),
	)
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}
	return err
}

const pipelineTaskGlobalFirstPass = "pipeline:global_first_pass"
