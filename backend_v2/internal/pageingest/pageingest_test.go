package pageingest

import (
	"context"
	"testing"
	"time"

	"aladin/backend_v2/internal/repo"
	coreservice "aladin/backend_v2/internal/service"
)

type fakePages struct {
	snap    repo.PageIngestSnapshot
	snapErr error
	marks   int
	markRev int64
}

func (f *fakePages) ListPagesDueForIngest(context.Context, time.Duration, int) ([]repo.PageIngestCandidate, error) {
	return nil, nil
}
func (f *fakePages) GetPageIngestSnapshot(context.Context, string) (repo.PageIngestSnapshot, error) {
	return f.snap, f.snapErr
}
func (f *fakePages) MarkPageIngested(_ context.Context, _ string, rev int64) error {
	f.marks++
	f.markRev = rev
	return nil
}

type fakeExtractor struct{ calls int }

func (f *fakeExtractor) ForArtifact(context.Context, string, string, string) (int, error) {
	f.calls++
	return 2, nil
}

// TestIngest_Guards is the heart of the supersede-on-new-edit requirement: the worker reads
// live page state and only extracts when the page is uningested AND idle — unless forced.
func TestIngest_Guards(t *testing.T) {
	idle := 10 * time.Minute
	now := time.Now()

	cases := []struct {
		name        string
		snap        repo.PageIngestSnapshot
		snapErr     error
		force       bool
		wantExtract bool
		wantMark    bool
	}{
		{
			name:        "idle and uningested → extract + mark",
			snap:        repo.PageIngestSnapshot{OwnerID: "u", Revision: 5, LastIngested: 0, UpdatedAt: now.Add(-30 * time.Minute)},
			wantExtract: true, wantMark: true,
		},
		{
			name:        "fresh edit (not idle) → defer, no extract",
			snap:        repo.PageIngestSnapshot{OwnerID: "u", Revision: 5, LastIngested: 0, UpdatedAt: now.Add(-1 * time.Minute)},
			wantExtract: false, wantMark: false,
		},
		{
			name:        "already ingested at this revision → skip",
			snap:        repo.PageIngestSnapshot{OwnerID: "u", Revision: 5, LastIngested: 5, UpdatedAt: now.Add(-30 * time.Minute)},
			wantExtract: false, wantMark: false,
		},
		{
			name:  "force bypasses idle + already-ingested",
			snap:  repo.PageIngestSnapshot{OwnerID: "u", Revision: 5, LastIngested: 5, UpdatedAt: now.Add(-1 * time.Minute)},
			force: true, wantExtract: true, wantMark: true,
		},
		{
			name:    "missing page → no-op",
			snapErr: coreservice.ErrNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pages := &fakePages{snap: tc.snap, snapErr: tc.snapErr}
			ext := &fakeExtractor{}
			w := NewWorker(pages, ext, nil).WithIdle(idle)
			if _, err := w.Ingest(context.Background(), "art-1", tc.force); err != nil {
				t.Fatalf("Ingest: %v", err)
			}
			if got := ext.calls > 0; got != tc.wantExtract {
				t.Fatalf("extract called=%v, want %v", got, tc.wantExtract)
			}
			if got := pages.marks > 0; got != tc.wantMark {
				t.Fatalf("mark called=%v, want %v", got, tc.wantMark)
			}
			if tc.wantMark && pages.markRev != tc.snap.Revision {
				t.Fatalf("marked revision=%d, want %d", pages.markRev, tc.snap.Revision)
			}
		})
	}
}
