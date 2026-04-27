package pipeline

import (
	"context"
	"errors"
	"time"

	"github.com/hibiken/asynq"
)

const defaultStageMaxRetry = 3

// Enqueuer hides asynq-specific task construction and queue policy from handlers.
type Enqueuer interface {
	EnqueueStage(ctx context.Context, taskType, recordID string, payload []byte) error
}

type asynqEnqueuer struct {
	client *asynq.Client
}

func NewAsynqEnqueuer(client *asynq.Client) Enqueuer {
	return &asynqEnqueuer{client: client}
}

func (e *asynqEnqueuer) EnqueueStage(ctx context.Context, taskType, recordID string, payload []byte) error {
	task := asynq.NewTask(taskType, payload)
	_, err := e.client.EnqueueContext(ctx, task,
		asynq.Queue(taskType),
		asynq.MaxRetry(defaultStageMaxRetry),
		asynq.TaskID(stageTaskID(recordID, taskType)),
	)
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}
	return err
}

func stageTaskID(recordID, taskType string) string {
	return recordID + ":" + taskType
}

// RetryDelay is the asynq RetryDelayFunc for pipeline tasks.
// It honours ErrRateLimit.RetryAfter when set, and falls back to asynq's
// default exponential backoff for all other errors.
func RetryDelay(n int, err error, t *asynq.Task) time.Duration {
	var rl ErrRateLimit
	if errors.As(err, &rl) && rl.RetryAfter > 0 {
		return rl.RetryAfter
	}
	return asynq.DefaultRetryDelayFunc(n, err, t)
}

// IsFailure reports whether an error should consume asynq retry budget.
// Rate limits should be delayed and retried, but should not count as failures.
func IsFailure(err error) bool {
	var rl ErrRateLimit
	return !errors.As(err, &rl)
}
