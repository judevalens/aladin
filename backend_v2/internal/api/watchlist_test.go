package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aladin/backend_v2/internal/app"
	coreservice "aladin/backend_v2/internal/service"
)

type fakeWatchlistSvc struct {
	created   *coreservice.Watchlist
	renameErr error
}

func (f *fakeWatchlistSvc) ListWatchlists(context.Context, string) ([]coreservice.Watchlist, error) {
	return []coreservice.Watchlist{{ID: "l1", Name: "Tech", Kind: "manual", ItemCount: 3}}, nil
}
func (f *fakeWatchlistSvc) CreateWatchlist(_ context.Context, _, name string) (coreservice.Watchlist, error) {
	w := coreservice.Watchlist{ID: "new", Name: name, Kind: "manual"}
	f.created = &w
	return w, nil
}
func (f *fakeWatchlistSvc) RenameWatchlist(context.Context, string, string, string) error {
	return f.renameErr
}
func (f *fakeWatchlistSvc) DeleteWatchlist(context.Context, string, string) error { return nil }
func (f *fakeWatchlistSvc) ListItems(context.Context, string, string) ([]coreservice.WatchlistItem, error) {
	return []coreservice.WatchlistItem{}, nil
}
func (f *fakeWatchlistSvc) AddItem(context.Context, string, string, string) error    { return nil }
func (f *fakeWatchlistSvc) RemoveItem(context.Context, string, string, string) error { return nil }
func (f *fakeWatchlistSvc) ResolveOrCreateByName(context.Context, string, string) (string, error) {
	return "l1", nil
}
func (f *fakeWatchlistSvc) ResolveInstruments(context.Context, string, string) ([]coreservice.WatchlistItem, error) {
	return nil, nil
}
func (f *fakeWatchlistSvc) List(context.Context, string) ([]coreservice.WatchlistItem, error) {
	return nil, nil
}
func (f *fakeWatchlistSvc) Add(context.Context, string, string) error    { return nil }
func (f *fakeWatchlistSvc) Remove(context.Context, string, string) error { return nil }

func authedReq(method, path, body string) *http.Request {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	ctx := coreservice.WithPrincipal(r.Context(), coreservice.Principal{UserID: "u1"})
	return r.WithContext(ctx)
}

func TestWatchlistCreateHandler(t *testing.T) {
	t.Parallel()
	svc := &fakeWatchlistSvc{}
	s := &Server{deps: app.StaticDependencies{WatchlistSvc: svc}}

	rec := httptest.NewRecorder()
	s.handleWatchlistsCreate(rec, authedReq(http.MethodPost, "/api/watchlists", `{"name":"Shorts"}`))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if svc.created == nil || svc.created.Name != "Shorts" {
		t.Fatalf("service not called with name Shorts: %+v", svc.created)
	}
}

func TestWatchlistRenameNotFoundMaps404(t *testing.T) {
	t.Parallel()
	svc := &fakeWatchlistSvc{renameErr: coreservice.ErrWatchlistNotFound}
	s := &Server{deps: app.StaticDependencies{WatchlistSvc: svc}}

	rec := httptest.NewRecorder()
	req := authedReq(http.MethodPatch, "/api/watchlists/nope", `{"name":"X"}`)
	req.SetPathValue("id", "nope")
	s.handleWatchlistRename(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}
