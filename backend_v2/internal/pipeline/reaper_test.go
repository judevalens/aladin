package pipeline

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/db"
)

type fakeStuckLister struct{ recs []*db.Record }

func (f *fakeStuckLister) ListStuck(_ context.Context, _, _ int) ([]*db.Record, error) {
	return f.recs, nil
}

func TestReaperReDrivesByStatus(t *testing.T) {
	t.Parallel()

	enq := &fakeEnqueuer{}
	lister := &fakeStuckLister{recs: []*db.Record{
		{ID: "rec-cap", Status: "captured", SourceRevision: 1},
		{ID: "rec-enr", Status: "enriched", SourceRevision: 1},
		{ID: "rec-weird", Status: "somethingelse"},
	}}

	n, err := NewReaper(lister, enq).Sweep(context.Background())
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if n != 2 {
		t.Fatalf("re-drove %d records, want 2 (the unknown status is skipped)", n)
	}

	byRecord := map[string]string{}
	for _, c := range enq.stageCalls {
		byRecord[c.recordID] = c.taskType
	}
	if byRecord["rec-cap"] != TaskGlobalFirstPass {
		t.Fatalf("captured re-drove to %q, want %q", byRecord["rec-cap"], TaskGlobalFirstPass)
	}
	if byRecord["rec-enr"] != TaskResolveEntities {
		t.Fatalf("enriched re-drove to %q, want %q", byRecord["rec-enr"], TaskResolveEntities)
	}
	if _, ok := byRecord["rec-weird"]; ok {
		t.Fatal("unknown-status record should not be re-driven")
	}
}
