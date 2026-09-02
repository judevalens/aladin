package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aladin/backend_v2/internal/app"
	coreservice "aladin/backend_v2/internal/service"
)

type watchlistCall struct {
	op           string
	userID       string
	listID       string
	instrumentID string
	name         string
}

// apiOnlyWatchlistSvc intentionally implements exactly the consumer-owned
// contract. This compile-time assertion prevents the route dependency from
// silently growing back to the broader service surface.
type apiOnlyWatchlistSvc struct{}

func (apiOnlyWatchlistSvc) ListWatchlists(context.Context, string) ([]coreservice.Watchlist, error) {
	return nil, nil
}
func (apiOnlyWatchlistSvc) CreateWatchlist(context.Context, string, string) (coreservice.Watchlist, error) {
	return coreservice.Watchlist{}, nil
}
func (apiOnlyWatchlistSvc) RenameWatchlist(context.Context, string, string, string) error {
	return nil
}
func (apiOnlyWatchlistSvc) DeleteWatchlist(context.Context, string, string) error { return nil }
func (apiOnlyWatchlistSvc) ListItems(context.Context, string, string) ([]coreservice.WatchlistItem, error) {
	return nil, nil
}
func (apiOnlyWatchlistSvc) AddItem(context.Context, string, string, string) error { return nil }
func (apiOnlyWatchlistSvc) RemoveItem(context.Context, string, string, string) error {
	return nil
}

var _ watchlistService = apiOnlyWatchlistSvc{}

type fakeWatchlistSvc struct {
	created *coreservice.Watchlist
	lists   []coreservice.Watchlist
	items   []coreservice.WatchlistItem
	err     error
	calls   []watchlistCall
}

func (f *fakeWatchlistSvc) record(call watchlistCall) { f.calls = append(f.calls, call) }

func (f *fakeWatchlistSvc) ListWatchlists(_ context.Context, userID string) ([]coreservice.Watchlist, error) {
	f.record(watchlistCall{op: "list-watchlists", userID: userID})
	return f.lists, f.err
}
func (f *fakeWatchlistSvc) CreateWatchlist(_ context.Context, userID, name string) (coreservice.Watchlist, error) {
	f.record(watchlistCall{op: "create-watchlist", userID: userID, name: name})
	w := coreservice.Watchlist{ID: "new", Name: name, Kind: coreservice.WatchlistManual}
	f.created = &w
	return w, f.err
}
func (f *fakeWatchlistSvc) RenameWatchlist(_ context.Context, userID, id, name string) error {
	f.record(watchlistCall{op: "rename-watchlist", userID: userID, listID: id, name: name})
	return f.err
}
func (f *fakeWatchlistSvc) DeleteWatchlist(_ context.Context, userID, id string) error {
	f.record(watchlistCall{op: "delete-watchlist", userID: userID, listID: id})
	return f.err
}
func (f *fakeWatchlistSvc) ListItems(_ context.Context, userID, listID string) ([]coreservice.WatchlistItem, error) {
	f.record(watchlistCall{op: "list-items", userID: userID, listID: listID})
	return f.items, f.err
}
func (f *fakeWatchlistSvc) AddItem(_ context.Context, userID, listID, instrumentID string) error {
	f.record(watchlistCall{op: "add-item", userID: userID, listID: listID, instrumentID: instrumentID})
	return f.err
}
func (f *fakeWatchlistSvc) RemoveItem(_ context.Context, userID, listID, instrumentID string) error {
	f.record(watchlistCall{op: "remove-item", userID: userID, listID: listID, instrumentID: instrumentID})
	return f.err
}
func (f *fakeWatchlistSvc) ResolveOrCreateByName(context.Context, string, string) (string, error) {
	return "l1", f.err
}
func (f *fakeWatchlistSvc) ResolveInstruments(context.Context, string, string) ([]coreservice.WatchlistItem, error) {
	return f.items, f.err
}
func (f *fakeWatchlistSvc) List(context.Context, string) ([]coreservice.WatchlistItem, error) {
	return f.items, f.err
}
func (f *fakeWatchlistSvc) Add(context.Context, string, string) error    { return f.err }
func (f *fakeWatchlistSvc) Remove(context.Context, string, string) error { return f.err }

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

func serveWatchlistRequest(t *testing.T, svc *fakeWatchlistSvc, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	server := NewWithDependencies(":0", app.StaticDependencies{WatchlistSvc: svc})
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, authedReq(method, path, body))
	return rec
}

func decodeObject(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return body
}

func assertOKBody(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	body := decodeObject(t, rec)
	if len(body) != 1 || body["ok"] != true {
		t.Fatalf("body = %#v, want {ok:true}", body)
	}
}

func TestWatchlistRouteContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantCall   watchlistCall
		assertBody func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "list watchlists", method: http.MethodGet, path: "/api/watchlists", wantStatus: http.StatusOK,
			wantCall: watchlistCall{op: "list-watchlists", userID: "u1"},
			assertBody: func(t *testing.T, rec *httptest.ResponseRecorder) {
				body := decodeObject(t, rec)
				lists, ok := body["watchlists"].([]any)
				if !ok || len(lists) != 1 || len(body) != 1 {
					t.Fatalf("body = %#v, want one watchlists field with one item", body)
				}
			},
		},
		{
			name: "create watchlist", method: http.MethodPost, path: "/api/watchlists", body: `{"name":"Shorts"}`, wantStatus: http.StatusCreated,
			wantCall: watchlistCall{op: "create-watchlist", userID: "u1", name: "Shorts"},
			assertBody: func(t *testing.T, rec *httptest.ResponseRecorder) {
				body := decodeObject(t, rec)
				for _, key := range []string{"id", "name", "kind", "position", "itemCount", "createdAt"} {
					if _, ok := body[key]; !ok {
						t.Fatalf("body = %#v, missing %q", body, key)
					}
				}
				if body["id"] != "new" || body["name"] != "Shorts" || body["kind"] != coreservice.WatchlistManual {
					t.Fatalf("body = %#v, want created watchlist", body)
				}
			},
		},
		{
			name: "rename watchlist", method: http.MethodPatch, path: "/api/watchlists/l1", body: `{"name":"Longs"}`, wantStatus: http.StatusOK,
			wantCall:   watchlistCall{op: "rename-watchlist", userID: "u1", listID: "l1", name: "Longs"},
			assertBody: assertOKBody,
		},
		{
			name: "delete watchlist", method: http.MethodDelete, path: "/api/watchlists/l1", wantStatus: http.StatusOK,
			wantCall:   watchlistCall{op: "delete-watchlist", userID: "u1", listID: "l1"},
			assertBody: assertOKBody,
		},
		{
			name: "list items", method: http.MethodGet, path: "/api/watchlists/l1/items", wantStatus: http.StatusOK,
			wantCall: watchlistCall{op: "list-items", userID: "u1", listID: "l1"},
			assertBody: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var items []map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil || len(items) != 1 {
					t.Fatalf("body = %q, want one-item JSON array: %v", rec.Body.String(), err)
				}
				if items[0]["instrumentId"] != "inst-1" || items[0]["symbol"] != "NVDA" {
					t.Fatalf("items = %#v", items)
				}
			},
		},
		{
			name: "add item", method: http.MethodPost, path: "/api/watchlists/l1/items", body: `{"instrumentId":"inst-1"}`, wantStatus: http.StatusCreated,
			wantCall:   watchlistCall{op: "add-item", userID: "u1", listID: "l1", instrumentID: "inst-1"},
			assertBody: assertOKBody,
		},
		{
			name: "remove item", method: http.MethodDelete, path: "/api/watchlists/l1/items/inst-1", wantStatus: http.StatusOK,
			wantCall:   watchlistCall{op: "remove-item", userID: "u1", listID: "l1", instrumentID: "inst-1"},
			assertBody: assertOKBody,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := &fakeWatchlistSvc{
				lists: []coreservice.Watchlist{{ID: "l1", Name: "Tech", Kind: coreservice.WatchlistManual, ItemCount: 1}},
				items: []coreservice.WatchlistItem{{InstrumentID: "inst-1", Symbol: "NVDA", Name: "NVIDIA", Exchange: "NASDAQ", AddedAt: "2026-09-02"}},
			}
			rec := serveWatchlistRequest(t, svc, tt.method, tt.path, tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if len(svc.calls) != 1 || svc.calls[0] != tt.wantCall {
				t.Fatalf("calls = %#v, want %#v", svc.calls, tt.wantCall)
			}
			tt.assertBody(t, rec)
		})
	}
}

func TestWatchlistRouteRequiresAuthentication(t *testing.T) {
	t.Parallel()
	server := NewWithDependencies(":0", app.StaticDependencies{AuthSvc: &fakeAuthService{}, WatchlistSvc: &fakeWatchlistSvc{}})
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/watchlists", nil))
	if rec.Code != http.StatusUnauthorized || strings.TrimSpace(rec.Body.String()) != `{"error":"Unauthenticated"}` {
		t.Fatalf("response = %d %q, want 401 JSON error", rec.Code, rec.Body.String())
	}
}

func TestWatchlistCreateRejectsInvalidJSON(t *testing.T) {
	t.Parallel()
	rec := serveWatchlistRequest(t, &fakeWatchlistSvc{}, http.MethodPost, "/api/watchlists", `{"name":`)
	if rec.Code != http.StatusBadRequest || strings.TrimSpace(rec.Body.String()) != `{"error":"invalid json body"}` {
		t.Fatalf("response = %d %q, want 400 invalid json body", rec.Code, rec.Body.String())
	}
}

func TestWatchlistErrorContract(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantError  string
	}{
		{name: "invalid input", err: coreservice.ErrInvalidWatchlistInput, wantStatus: http.StatusBadRequest, wantError: coreservice.ErrInvalidWatchlistInput.Error()},
		{name: "not found", err: coreservice.ErrWatchlistNotFound, wantStatus: http.StatusNotFound, wantError: "Watchlist not found"},
		{name: "service failure", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantError: "database unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := serveWatchlistRequest(t, &fakeWatchlistSvc{err: tt.err}, http.MethodPatch, "/api/watchlists/l1", `{"name":"X"}`)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if body := decodeObject(t, rec); body["error"] != tt.wantError || len(body) != 1 {
				t.Fatalf("body = %#v, want error %q", body, tt.wantError)
			}
		})
	}
}

func TestWatchlistCreateHandler(t *testing.T) {
	t.Parallel()
	svc := &fakeWatchlistSvc{}
	routes := newWatchlistRoutes(svc)

	rec := httptest.NewRecorder()
	routes.handleWatchlistsCreate(rec, authedReq(http.MethodPost, "/api/watchlists", `{"name":"Shorts"}`))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if svc.created == nil || svc.created.Name != "Shorts" {
		t.Fatalf("service not called with name Shorts: %+v", svc.created)
	}
}

func TestWatchlistRenameNotFoundMaps404(t *testing.T) {
	t.Parallel()
	svc := &fakeWatchlistSvc{err: coreservice.ErrWatchlistNotFound}
	routes := newWatchlistRoutes(svc)

	rec := httptest.NewRecorder()
	req := authedReq(http.MethodPatch, "/api/watchlists/nope", `{"name":"X"}`)
	req.SetPathValue("id", "nope")
	routes.handleWatchlistRename(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}
