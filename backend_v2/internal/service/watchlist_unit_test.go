package service

import (
	"context"
	"errors"
	"testing"
)

type watchlistRepoCall struct {
	op           string
	userID       string
	listID       string
	instrumentID string
}

type fakeWatchlistRepository struct {
	lists     []Watchlist
	items     []WatchlistItem
	watchlist Watchlist
	found     bool
	defaultID string
	defaultOK bool
	created   *Watchlist
	calls     []watchlistRepoCall
	err       error
}

func (f *fakeWatchlistRepository) record(call watchlistRepoCall) { f.calls = append(f.calls, call) }

func (f *fakeWatchlistRepository) ListWatchlists(_ context.Context, userID string) ([]Watchlist, error) {
	f.record(watchlistRepoCall{op: "list-watchlists", userID: userID})
	return f.lists, f.err
}
func (f *fakeWatchlistRepository) CreateWatchlist(_ context.Context, w Watchlist, userID string) (Watchlist, error) {
	f.record(watchlistRepoCall{op: "create-watchlist", userID: userID})
	copyW := w
	f.created = &copyW
	return w, f.err
}
func (f *fakeWatchlistRepository) RenameWatchlist(_ context.Context, userID, id, _ string) error {
	f.record(watchlistRepoCall{op: "rename-watchlist", userID: userID, listID: id})
	return f.err
}
func (f *fakeWatchlistRepository) DeleteWatchlist(_ context.Context, userID, id string) error {
	f.record(watchlistRepoCall{op: "delete-watchlist", userID: userID, listID: id})
	return f.err
}
func (f *fakeWatchlistRepository) GetWatchlist(_ context.Context, userID, id string) (Watchlist, bool, error) {
	f.record(watchlistRepoCall{op: "get-watchlist", userID: userID, listID: id})
	return f.watchlist, f.found, f.err
}
func (f *fakeWatchlistRepository) DefaultWatchlistID(_ context.Context, userID string) (string, bool, error) {
	f.record(watchlistRepoCall{op: "default-watchlist", userID: userID})
	return f.defaultID, f.defaultOK, f.err
}
func (f *fakeWatchlistRepository) ListItems(_ context.Context, userID, listID string) ([]WatchlistItem, error) {
	f.record(watchlistRepoCall{op: "list-items", userID: userID, listID: listID})
	return f.items, f.err
}
func (f *fakeWatchlistRepository) AddItem(_ context.Context, userID, listID, instrumentID string) error {
	f.record(watchlistRepoCall{op: "add-item", userID: userID, listID: listID, instrumentID: instrumentID})
	return f.err
}
func (f *fakeWatchlistRepository) RemoveItem(_ context.Context, userID, listID, instrumentID string) error {
	f.record(watchlistRepoCall{op: "remove-item", userID: userID, listID: listID, instrumentID: instrumentID})
	return f.err
}

func TestWatchlistServiceRejectsInvalidInputBeforeRepository(t *testing.T) {
	t.Parallel()
	repo := &fakeWatchlistRepository{}
	svc := NewWatchlistService(repo)
	ctx := context.Background()
	tests := []struct {
		name string
		call func() error
	}{
		{name: "list missing user", call: func() error { _, err := svc.ListWatchlists(ctx, ""); return err }},
		{name: "create missing user", call: func() error { _, err := svc.CreateWatchlist(ctx, "", "Tech"); return err }},
		{name: "create missing name", call: func() error { _, err := svc.CreateWatchlist(ctx, "u1", " "); return err }},
		{name: "rename missing id", call: func() error { return svc.RenameWatchlist(ctx, "u1", "", "Tech") }},
		{name: "rename missing name", call: func() error { return svc.RenameWatchlist(ctx, "u1", "l1", "") }},
		{name: "delete missing id", call: func() error { return svc.DeleteWatchlist(ctx, "u1", "") }},
		{name: "list items missing user", call: func() error { _, err := svc.ListItems(ctx, "", "l1"); return err }},
		{name: "add missing instrument", call: func() error { return svc.AddItem(ctx, "u1", "l1", "") }},
		{name: "remove missing instrument", call: func() error { return svc.RemoveItem(ctx, "u1", "l1", "") }},
		{name: "resolve missing user", call: func() error { _, err := svc.ResolveInstruments(ctx, "", "l1"); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); !errors.Is(err, ErrInvalidWatchlistInput) {
				t.Fatalf("error = %v, want ErrInvalidWatchlistInput", err)
			}
		})
	}
	if len(repo.calls) != 0 {
		t.Fatalf("invalid input reached repository: %#v", repo.calls)
	}
}

func TestWatchlistServiceNormalizesNilCollections(t *testing.T) {
	t.Parallel()
	repo := &fakeWatchlistRepository{}
	svc := NewWatchlistService(repo)
	lists, err := svc.ListWatchlists(context.Background(), "u1")
	if err != nil || lists == nil || len(lists) != 0 {
		t.Fatalf("lists = %#v, err = %v; want non-nil empty slice", lists, err)
	}
	items, err := svc.ListItems(context.Background(), "u1", "l1")
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("items = %#v, err = %v; want non-nil empty slice", items, err)
	}
}

func TestWatchlistServicePreservesRepositoryCancellation(t *testing.T) {
	t.Parallel()
	repo := &fakeWatchlistRepository{err: context.Canceled}
	svc := NewWatchlistService(repo)
	_, err := svc.ListWatchlists(context.Background(), "u1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled identity", err)
	}
}

func TestWatchlistServiceLazilyCreatesDefaultList(t *testing.T) {
	t.Parallel()
	repo := &fakeWatchlistRepository{}
	svc := NewWatchlistService(repo)
	if _, err := svc.ListItems(context.Background(), "u1", ""); err != nil {
		t.Fatal(err)
	}
	if repo.created == nil || repo.created.Name != "Watchlist" || repo.created.Kind != WatchlistManual || repo.created.Position != 0 {
		t.Fatalf("created = %#v, want default manual Watchlist at position zero", repo.created)
	}
	last := repo.calls[len(repo.calls)-1]
	if last.op != "list-items" || last.listID != repo.created.ID {
		t.Fatalf("last call = %#v, want list-items for created id", last)
	}
}

func TestWatchlistServiceResolvesNameCaseInsensitivelyOrCreates(t *testing.T) {
	t.Parallel()
	repo := &fakeWatchlistRepository{lists: []Watchlist{{ID: "semis", Name: "Semiconductors"}}}
	svc := NewWatchlistService(repo)
	id, err := svc.ResolveOrCreateByName(context.Background(), "u1", " sEmIcOnDuCtOrS ")
	if err != nil || id != "semis" || repo.created != nil {
		t.Fatalf("id = %q, created = %#v, err = %v", id, repo.created, err)
	}

	repo.lists = nil
	id, err = svc.ResolveOrCreateByName(context.Background(), "u1", " Energy ")
	if err != nil || id == "" || repo.created == nil || repo.created.Name != "Energy" {
		t.Fatalf("id = %q, created = %#v, err = %v", id, repo.created, err)
	}
}

func TestWatchlistCompatibilityShimsResolveDefaultList(t *testing.T) {
	t.Parallel()
	repo := &fakeWatchlistRepository{defaultID: "default", defaultOK: true}
	svc := NewWatchlistService(repo)
	ctx := context.Background()
	if _, err := svc.List(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Add(ctx, "u1", "inst-1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Remove(ctx, "u1", "inst-1"); err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"list-items", "add-item", "remove-item"} {
		found := false
		for _, call := range repo.calls {
			if call.op == op && call.listID == "default" {
				found = true
			}
		}
		if !found {
			t.Fatalf("calls = %#v, missing %s on default list", repo.calls, op)
		}
	}
}

func TestWatchlistResolveInstrumentsDispatchesByKind(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		kind    string
		wantErr error
		wantLen int
	}{
		{name: "manual", kind: WatchlistManual, wantLen: 1},
		{name: "screener reserved", kind: WatchlistScreener, wantErr: ErrScreenerNotImplemented},
		{name: "hybrid reserved", kind: WatchlistHybrid, wantErr: ErrScreenerNotImplemented},
		{name: "unknown falls back to members", kind: "future", wantLen: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &fakeWatchlistRepository{
				watchlist: Watchlist{ID: "l1", Kind: tt.kind}, found: true,
				items: []WatchlistItem{{InstrumentID: "inst-1"}},
			}
			svc := NewWatchlistService(repo)
			items, err := svc.ResolveInstruments(context.Background(), "u1", "l1")
			if !errors.Is(err, tt.wantErr) || len(items) != tt.wantLen {
				t.Fatalf("items = %#v, err = %v; want len %d, err %v", items, err, tt.wantLen, tt.wantErr)
			}
		})
	}
}
