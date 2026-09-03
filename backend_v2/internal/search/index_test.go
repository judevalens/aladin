package search

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type fakeContentIndexRepo struct {
	stale        []StaleArtifact
	pageBlocks   map[string]json.RawMessage
	artifactBody map[string]string
	replaced     map[string][]ContentRow
	searchHits   []ContentHit
}

func (f *fakeContentIndexRepo) StaleArtifacts(context.Context, int) ([]StaleArtifact, error) {
	return f.stale, nil
}

func (f *fakeContentIndexRepo) ReplaceRows(_ context.Context, target StaleArtifact, rows []ContentRow) error {
	if f.replaced == nil {
		f.replaced = map[string][]ContentRow{}
	}
	f.replaced[target.ArtifactID] = rows
	return nil
}

func (f *fakeContentIndexRepo) Search(context.Context, string, string, int) ([]ContentHit, error) {
	return f.searchHits, nil
}

func (f *fakeContentIndexRepo) PageBlocks(_ context.Context, artifactID string) (json.RawMessage, error) {
	return f.pageBlocks[artifactID], nil
}

func (*fakeContentIndexRepo) FilePages(context.Context, string) ([]NumberedPage, error) {
	return nil, nil
}

func (f *fakeContentIndexRepo) ArtifactBody(_ context.Context, artifactID string) (string, string, error) {
	return artifactID, f.artifactBody[artifactID], nil
}

func TestContentIndexSweepProjectsPageAndBoard(t *testing.T) {
	now := time.Now()
	repo := &fakeContentIndexRepo{
		stale: []StaleArtifact{
			{ArtifactID: "page-1", UserID: "user-1", Kind: "page", SourceStamp: now},
			{ArtifactID: "board-1", UserID: "user-1", Kind: "board", SourceStamp: now},
		},
		pageBlocks: map[string]json.RawMessage{
			"page-1": json.RawMessage(`[{"id":"block-1","content":[{"type":"text","text":"alpha thesis"}]}]`),
		},
		artifactBody: map[string]string{
			"board-1": `{"document":{"store":{"shape:t1":{"typeName":"shape","type":"aladin-task","id":"shape:t1","props":{"text":"read filing"}}}}}`,
		},
	}

	count, err := NewContentIndexService(repo).Sweep(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("indexed %d artifacts, want 2", count)
	}
	if got := repo.replaced["page-1"]; len(got) != 1 || got[0].Locator != "block:block-1" || got[0].Text != "alpha thesis" {
		t.Fatalf("page rows = %+v", got)
	}
	if got := repo.replaced["board-1"]; len(got) != 1 || got[0].Locator != "shape:shape:t1" || got[0].Text != "task: read filing" {
		t.Fatalf("board rows = %+v", got)
	}
}

func TestContentIndexSearchRejectsEmptyScopeOrQuery(t *testing.T) {
	repo := &fakeContentIndexRepo{searchHits: []ContentHit{{ArtifactID: "page-1"}}}
	svc := NewContentIndexService(repo)

	for _, tc := range []struct{ userID, query string }{{"", "alpha"}, {"user-1", "   "}} {
		got, err := svc.Search(context.Background(), tc.userID, tc.query, 10)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("Search(%q, %q) = %+v, want nil", tc.userID, tc.query, got)
		}
	}
}
