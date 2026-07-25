package sync

import (
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"

	"aladin/backend_v2/internal/db"
)

// fakeRecords records MarkFailed calls; other RecordRepository methods are unused here (embedding
// the interface leaves them nil — they'd panic if called, which the tests ensure they aren't).
type fakeRecords struct {
	db.RecordRepository
	failed map[string]string
}

func (f *fakeRecords) MarkFailed(_ context.Context, id, reason string) error {
	if f.failed == nil {
		f.failed = map[string]string{}
	}
	f.failed[id] = reason
	return nil
}

// A pipeline stage task that exhausts its retries marks its record terminally failed (record id is
// the prefix of the recordID:pipeline:<stage> task id).
func TestErrorHandler_PipelineExhaustionMarksRecordFailed(t *testing.T) {
	rec := &fakeRecords{}
	task := asynq.NewTask("pipeline:embed", nil)
	handleAsynqTaskError(context.Background(), task, "record-abc:pipeline:embed", errors.New("boom"),
		3 /*retryCount*/, 3 /*maxRetry*/, true /*hasRetryMetadata*/, rec, nil, nil)

	if reason, ok := rec.failed["record-abc"]; !ok || reason == "" {
		t.Fatalf("record not marked failed on exhaustion; got %+v", rec.failed)
	}
}

// Before exhaustion, nothing terminal happens (asynq will retry).
func TestErrorHandler_PipelineNotExhaustedNoMark(t *testing.T) {
	rec := &fakeRecords{}
	task := asynq.NewTask("pipeline:embed", nil)
	handleAsynqTaskError(context.Background(), task, "record-abc:pipeline:embed", errors.New("boom"),
		1 /*retryCount*/, 3 /*maxRetry*/, true, rec, nil, nil)

	if len(rec.failed) != 0 {
		t.Fatalf("must not mark failed before exhaustion; got %+v", rec.failed)
	}
}

// A task in neither the pipeline: nor sync: family must not touch record state.
func TestErrorHandler_UnrelatedTaskNoMark(t *testing.T) {
	rec := &fakeRecords{}
	task := asynq.NewTask("insights:generate", nil)
	handleAsynqTaskError(context.Background(), task, "whatever", errors.New("boom"), 3, 3, true, rec, nil, nil)

	if len(rec.failed) != 0 {
		t.Fatalf("unrelated task must not mark a record failed; got %+v", rec.failed)
	}
}
