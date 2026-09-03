package page

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"aladin/backend_v2/internal/apperror"
	"aladin/backend_v2/internal/auth"
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
		return nil, 0, apperror.ErrNotFound
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
		scopes = []string{auth.ScopeArtifactsRead, auth.ScopeArtifactsWrite}
	}
	return auth.WithPrincipal(context.Background(), auth.Principal{
		UserID:    "00000000-0000-0000-0000-000000000001",
		ActorType: auth.ActorTypeIntegrationToken,
		ActorID:   "token-1",
		Scopes:    scopes,
	})
}

func TestPageDocumentService_GetBlocks(t *testing.T) {
	store := &fakePageDocStore{
		blocks:   json.RawMessage(`[{"id":"a","type":"paragraph","content":[{"type":"text","text":"hi"}],"children":[]}]`),
		revision: 3,
	}
	svc := NewDocumentService(store)

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
	svc := NewDocumentService(store)
	ctx := writeContext("nothing:relevant")
	_, _, err := svc.GetBlocks(ctx, "page-1")
	if !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("expected auth.ErrForbidden, got %v", err)
	}
}

func TestPageDocumentService_ReplaceAll_ComputesSearchText(t *testing.T) {
	store := &fakePageDocStore{revision: 1}
	svc := NewDocumentService(store)
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
	svc := NewDocumentService(store)
	ctx := writeContext(auth.ScopeArtifactsRead)
	_, err := svc.ReplaceAll(ctx, "page-1", json.RawMessage(`[]`), 0)
	if !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("expected auth.ErrForbidden, got %v", err)
	}
}

func TestPageDocumentService_ReplaceAll_RejectsNonArray(t *testing.T) {
	store := &fakePageDocStore{}
	svc := NewDocumentService(store)
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
	store := &fakePageDocStore{saveErr: apperror.ErrConflict}
	svc := NewDocumentService(store)
	_, err := svc.ReplaceAll(writeContext(), "page-1", json.RawMessage(`[]`), 1)
	if !errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("expected apperror.ErrConflict, got %v", err)
	}
}

func TestPageDocumentService_ReplaceBlock(t *testing.T) {
	store := &fakePageDocStore{
		blocks: json.RawMessage(`[{"id":"a","type":"paragraph","content":[{"type":"text","text":"orig"}],"children":[]},{"id":"b","type":"paragraph","content":[],"children":[]}]`),
	}
	svc := NewDocumentService(store)
	replacement := json.RawMessage(`[{"id":"new","type":"heading","content":[{"type":"text","text":"NEW"}],"children":[]}]`)

	_, count, err := svc.ReplaceBlock(writeContext(), "page-1", "a", replacement, 0)
	if err != nil {
		t.Fatalf("ReplaceBlock: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	if !strings.Contains(string(store.lastSaved.blocks), `"id":"a"`) {
		t.Fatalf("first new block should inherit original id; saved = %s", string(store.lastSaved.blocks))
	}
	if !strings.Contains(string(store.lastSaved.blocks), `"type":"heading"`) {
		t.Fatalf("expected heading in saved blocks: %s", string(store.lastSaved.blocks))
	}
	if !strings.Contains(store.lastSaved.searchText, "NEW") {
		t.Fatalf("search_text = %q, want it to include NEW", store.lastSaved.searchText)
	}
}

func TestPageDocumentService_ReplaceBlock_NotFound(t *testing.T) {
	store := &fakePageDocStore{
		blocks: json.RawMessage(`[{"id":"a","type":"paragraph","content":[],"children":[]}]`),
	}
	svc := NewDocumentService(store)
	_, _, err := svc.ReplaceBlock(writeContext(), "page-1", "missing", json.RawMessage(`[{"id":"x","type":"p"}]`), 0)
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("error = %v, want apperror.ErrNotFound", err)
	}
}

func TestPageDocumentService_InsertBlocks_After(t *testing.T) {
	store := &fakePageDocStore{
		blocks: json.RawMessage(`[{"id":"a","type":"p","content":[],"children":[]}]`),
	}
	svc := NewDocumentService(store)
	after := "a"
	_, ids, err := svc.InsertBlocks(
		writeContext(),
		"page-1",
		BlockPosition{AfterID: &after},
		json.RawMessage(`[{"id":"x","type":"p"},{"id":"y","type":"p"}]`),
		0,
	)
	if err != nil {
		t.Fatalf("InsertBlocks: %v", err)
	}
	if !equalStrSlices(ids, []string{"x", "y"}) {
		t.Fatalf("inserted ids = %v, want [x y]", ids)
	}
	if !strings.Contains(string(store.lastSaved.blocks), `"id":"x"`) {
		t.Fatalf("saved blocks do not include x: %s", string(store.lastSaved.blocks))
	}
}

func TestPageDocumentService_InsertBlocks_PositionValidation(t *testing.T) {
	store := &fakePageDocStore{blocks: json.RawMessage(`[{"id":"a","type":"p"}]`)}
	svc := NewDocumentService(store)
	ctx := writeContext()

	if _, _, err := svc.InsertBlocks(ctx, "page-1", BlockPosition{}, json.RawMessage(`[{"id":"x","type":"p"}]`), 0); err == nil {
		t.Fatal("expected position validation error for empty position")
	}
	a := "a"
	end := "end"
	if _, _, err := svc.InsertBlocks(ctx, "page-1", BlockPosition{AfterID: &a, At: &end}, json.RawMessage(`[{"id":"x","type":"p"}]`), 0); err == nil {
		t.Fatal("expected error when multiple position fields are set")
	}
	bad := "middle"
	if _, _, err := svc.InsertBlocks(ctx, "page-1", BlockPosition{At: &bad}, json.RawMessage(`[{"id":"x","type":"p"}]`), 0); err == nil {
		t.Fatal("expected error for invalid at value")
	}
}

func TestPageDocumentService_DeleteBlock(t *testing.T) {
	store := &fakePageDocStore{
		blocks: json.RawMessage(`[{"id":"a","type":"p","content":[],"children":[]},{"id":"b","type":"p","content":[],"children":[]}]`),
	}
	svc := NewDocumentService(store)
	if _, err := svc.DeleteBlock(writeContext(), "page-1", "a", 0); err != nil {
		t.Fatalf("DeleteBlock: %v", err)
	}
	if !strings.Contains(string(store.lastSaved.blocks), `"id":"b"`) || strings.Contains(string(store.lastSaved.blocks), `"id":"a"`) {
		t.Fatalf("saved blocks = %s, want only b", string(store.lastSaved.blocks))
	}
}

func TestPageDocumentService_DeleteBlock_RefusesLast(t *testing.T) {
	store := &fakePageDocStore{
		blocks: json.RawMessage(`[{"id":"a","type":"p","content":[],"children":[]}]`),
	}
	svc := NewDocumentService(store)
	_, err := svc.DeleteBlock(writeContext(), "page-1", "a", 0)
	var requestErr apperror.BadRequest
	if !errors.As(err, &requestErr) {
		t.Fatalf("error = %v, want apperror.BadRequest", err)
	}
	if store.saveCalls != 0 {
		t.Fatalf("store should not be called; got %d save calls", store.saveCalls)
	}
}

func TestPageDocumentService_DeleteBlock_NotFound(t *testing.T) {
	store := &fakePageDocStore{
		blocks: json.RawMessage(`[{"id":"a","type":"p","content":[],"children":[]},{"id":"b","type":"p","content":[],"children":[]}]`),
	}
	svc := NewDocumentService(store)
	if _, err := svc.DeleteBlock(writeContext(), "page-1", "missing", 0); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("error = %v, want apperror.ErrNotFound", err)
	}
}

func equalStrSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
