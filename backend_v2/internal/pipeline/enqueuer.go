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
	EnqueueStage(ctx context.Context, taskType, artifactID string, payload []byte) error
	EnqueueStageDelayed(ctx context.Context, taskType, artifactID string, payload []byte, delay time.Duration) error
}

type asynqEnqueuer struct {
	client *asynq.Client
}

func NewAsynqEnqueuer(client *asynq.Client) Enqueuer {
	return &asynqEnqueuer{client: client}
}

func (e *asynqEnqueuer) EnqueueStage(ctx context.Context, taskType, artifactID string, payload []byte) error {
	task := asynq.NewTask(taskType, payload)
	_, err := e.client.EnqueueContext(ctx, task,
		asynq.Queue(taskType),
		asynq.MaxRetry(defaultStageMaxRetry),
		asynq.TaskID(stageTaskID(artifactID, taskType)),
	)
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}
	return err
}

func (e *asynqEnqueuer) EnqueueStageDelayed(ctx context.Context, taskType, artifactID string, payload []byte, delay time.Duration) error {
	task := asynq.NewTask(taskType, payload)
	_, err := e.client.EnqueueContext(ctx, task,
		asynq.Queue(taskType),
		asynq.ProcessIn(delay),
		asynq.MaxRetry(defaultStageMaxRetry),
		asynq.TaskID(stageTaskID(artifactID, taskType)),
	)
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}
	return err
}

func stageTaskID(artifactID, taskType string) string {
	return artifactID + ":" + taskType
}
