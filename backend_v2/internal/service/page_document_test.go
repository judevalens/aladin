package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakePageDocStore struct {
	blocks      json.RawMessage
	revision    int64
	getErr      error
	saveErr     error
	lastSaved   savedCall
	saveCalls   int
	missingPage bool
}

type savedCall struct {
	pageID     string
	blocks     json.RawMessage
	searchText string
	expectRev  int64
}

func (f *fakePageDocStore) GetPageBlocks(_ context.Context, _ string) (json.RawMessage, int64, error) {
	if f.missingPage {
		return nil, 0, ErrNotFound
	}
	if f.getErr != nil {
		return nil, 0, f.getErr
	}
	return f.blocks, f.revision, nil
}

func (f *fakePageDocStore) SavePageBlocks(_ context.Context, pageID string, blocks json.RawMessage, searchText string, expectedRev int64) (int64, error) {
	f.saveCalls++
	f.lastSaved = savedCall{pageID: pageID, blocks: blocks, searchText: searchText, expectRev: expectedRev}
	if f.saveErr != nil {
		return 0, f.saveErr
	}
	f.revision++
	f.blocks = blocks
	return f.revision, nil
}

func writeContext(scopes ...string) context.Context {
	if len(scopes) == 0 {
		scopes = []string{ScopeArtifactsRead, ScopeArtifactsWrite}
	}
	return WithPrincipal(context.Background(), Principal{
		UserID:    "00000000-0000-0000-0000-000000000001",
		ActorType: ActorTypeIntegrationToken,
		ActorID:   "token-1",
		Scopes:    scopes,
	})
}

func TestPageDocumentService_GetBlocks(t *testing.T) {
	store := &fakePageDocStore{
		blocks:   json.RawMessage(`[{"id":"a","type":"paragraph","content":[{"type":"text","text":"hi"}],"children":[]}]`),
		revision: 3,
	}
	svc := NewPageDocumentService(store)

	blocks, rev, err := svc.GetBlocks(writeContext(), "page-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rev != 3 {
		t.Fatalf("revision: got %d want 3", rev)
	}
	if string(blocks) != string(store.blocks) {
		t.Fatalf("blocks mismatch")
	}
}

func TestPageDocumentService_GetBlocks_RequiresReadScope(t *testing.T) {
	store := &fakePageDocStore{}
	svc := NewPageDocumentService(store)
	ctx := writeContext("nothing:relevant")
	_, _, err := svc.GetBlocks(ctx, "page-1")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestPageDocumentService_ReplaceAll_ComputesSearchText(t *testing.T) {
	store := &fakePageDocStore{revision: 1}
	svc := NewPageDocumentService(store)
	blocks := json.RawMessage(`[
		{"id":"a","type":"heading","content":[{"type":"text","text":"Title"}],"children":[]},
		{"id":"b","type":"paragraph","content":[{"type":"text","text":"Body"}],"children":[]}
	]`)

	newRev, err := svc.ReplaceAll(writeContext(), "page-1", blocks, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newRev != 2 {
		t.Fatalf("revision: got %d want 2", newRev)
	}
	if store.lastSaved.searchText != "Title\nBody" {
		t.Fatalf("search_text: got %q want %q", store.lastSaved.searchText, "Title\nBody")
	}
	if store.lastSaved.expectRev != 1 {
		t.Fatalf("expectedRevision passed through: got %d want 1", store.lastSaved.expectRev)
	}
}

func TestPageDocumentService_ReplaceAll_RequiresWriteScope(t *testing.T) {
	store := &fakePageDocStore{}
	svc := NewPageDocumentService(store)
	ctx := writeContext(ScopeArtifactsRead)
	_, err := svc.ReplaceAll(ctx, "page-1", json.RawMessage(`[]`), 0)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestPageDocumentService_ReplaceAll_RejectsNonArray(t *testing.T) {
	store := &fakePageDocStore{}
	svc := NewPageDocumentService(store)
	cases := []json.RawMessage{
		nil,
		json.RawMessage(``),
		json.RawMessage(`{"not":"array"}`),
		json.RawMessage(`"string"`),
		json.RawMessage(`null`),
	}
	for _, c := range cases {
		if _, err := svc.ReplaceAll(writeContext(), "page-1", c, 0); err == nil {
			t.Fatalf("expected error for input %q, got nil", string(c))
		}
	}
	if store.saveCalls != 0 {
		t.Fatalf("store should not have been called; got %d save calls", store.saveCalls)
	}
}

func TestPageDocumentService_ReplaceAll_ConflictFromStore(t *testing.T) {
	store := &fakePageDocStore{saveErr: ErrConflict}
	svc := NewPageDocumentService(store)
	_, err := svc.ReplaceAll(writeContext(), "page-1", json.RawMessage(`[]`), 1)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestPageDocumentService_BlockOpsNotImplemented(t *testing.T) {
	store := &fakePageDocStore{}
	svc := NewPageDocumentService(store)
	ctx := writeContext()

	if _, _, err := svc.ReplaceBlock(ctx, "p", "b", json.RawMessage(`[]`), 0); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("ReplaceBlock: expected ErrNotImplemented, got %v", err)
	}
	if _, _, err := svc.InsertBlocks(ctx, "p", BlockPosition{}, json.RawMessage(`[]`), 0); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("InsertBlocks: expected ErrNotImplemented, got %v", err)
	}
	if _, err := svc.DeleteBlock(ctx, "p", "b", 0); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("DeleteBlock: expected ErrNotImplemented, got %v", err)
	}
}
