package record

import (
	"context"
	"testing"
)

type fakeRepository struct {
	createdID    string
	similarLimit int
}

func (*fakeRepository) List(context.Context) ([]RecordResponse, error) { return nil, nil }
func (*fakeRepository) Children(context.Context, string, int, int) (map[string]any, error) {
	return nil, nil
}
func (r *fakeRepository) Create(_ context.Context, id, _, _, _, _, _ string) error {
	r.createdID = id
	return nil
}
func (*fakeRepository) Delete(context.Context, string) error                { return nil }
func (*fakeRepository) ResetForRetry(context.Context, string) (bool, error) { return false, nil }
func (r *fakeRepository) SimilarRecords(_ context.Context, _ string, limit int) ([]SimilarRecord, error) {
	r.similarLimit = limit
	return nil, nil
}

func TestServiceCreatesIDAndBoundsSimilarLimit(t *testing.T) {
	repo := &fakeRepository{}
	service := NewRecordService(repo)
	if err := service.Create(context.Background(), "", "note", "Label", "Content", "", ""); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(repo.createdID) <= len("record-") || repo.createdID[:len("record-")] != "record-" {
		t.Fatalf("generated id = %q, want record-*", repo.createdID)
	}
	if _, err := service.Similar(context.Background(), "record-1", 99); err != nil {
		t.Fatalf("Similar: %v", err)
	}
	if repo.similarLimit != 10 {
		t.Fatalf("similar limit = %d, want default 10", repo.similarLimit)
	}
}
