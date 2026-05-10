package insights

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"
)

const TaskGenerate = "insights:generate"

type GeneratePayload struct {
	KgID           string   `json:"kg_id"`
	RecordID       string   `json:"record_id"`
	SourceRevision int64    `json:"source_revision"`
	CorrelationID  string   `json:"correlation_id"`
	GeneratorKeys  []string `json:"generator_keys,omitempty"`
}

type AsynqEnqueuer struct {
	client *asynq.Client
}

func NewAsynqEnqueuer(client *asynq.Client) *AsynqEnqueuer {
	return &AsynqEnqueuer{client: client}
}

func (e *AsynqEnqueuer) EnqueueInsightGeneration(
	ctx context.Context,
	kgID string,
	recordID string,
	sourceRevision int64,
	correlationID string,
	generatorKeys []string,
) error {
	payload, err := json.Marshal(GeneratePayload{
		KgID:           kgID,
		RecordID:       recordID,
		SourceRevision: sourceRevision,
		CorrelationID:  correlationID,
		GeneratorKeys:  generatorKeys,
	})
	if err != nil {
		return fmt.Errorf("marshal insight generate payload: %w", err)
	}
	task := asynq.NewTask(TaskGenerate, payload)
	_, err = e.client.EnqueueContext(ctx, task,
		asynq.Queue(TaskGenerate),
		asynq.MaxRetry(3),
		asynq.TaskID(fmt.Sprintf("%s:%s:%d:%s", TaskGenerate, kgID, sourceRevision, recordID)),
	)
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}
	return err
}

type GenerateHandler struct {
	generator *Generator
}

func NewGenerateHandler(generator *Generator) *GenerateHandler {
	return &GenerateHandler{generator: generator}
}

func (h *GenerateHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload GeneratePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal insight generate payload: %w", err)
	}
	_, err := h.generator.GenerateForRecord(ctx, RecordInsightInput{
		KgID:           payload.KgID,
		RecordID:       payload.RecordID,
		SourceRevision: payload.SourceRevision,
		CorrelationID:  payload.CorrelationID,
		GeneratorKeys:  payload.GeneratorKeys,
	})
	return err
}

func RegisterGenerateHandler(mux *asynq.ServeMux, generator *Generator) {
	mux.Handle(TaskGenerate, NewGenerateHandler(generator))
}
