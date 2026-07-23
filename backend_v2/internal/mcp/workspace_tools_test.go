package mcpserver

import (
	"context"
	"errors"
	"testing"

	"aladin/backend_v2/internal/service"
)

// --- fakes -----------------------------------------------------------------

type fakeSearchService struct {
	userID string
	query  string
	limit  int
	resp   service.SearchResponse
	err    error
}

func (f *fakeSearchService) Search(_ context.Context, userID, query string, limit int) (service.SearchResponse, error) {
	f.userID, f.query, f.limit = userID, query, limit
	return f.resp, f.err
}

type fakeEntityContextService struct {
	getUserID string
	getID     string
	ec        service.EntityContext
	drawn     *service.DrawEdgeInput
	err       error
}

func (f *fakeEntityContextService) Get(_ context.Context, ownerUserID, entityID string) (service.EntityContext, error) {
	f.getUserID, f.getID = ownerUserID, entityID
	return f.ec, f.err
}
func (f *fakeEntityContextService) DrawEdge(_ context.Context, in service.DrawEdgeInput) error {
	copyIn := in
	f.drawn = &copyIn
	return f.err
}
func (f *fakeEntityContextService) MergeQueue(context.Context, int) ([]service.MergeQueueItem, error) {
	return nil, nil
}
func (f *fakeEntityContextService) AcceptMerge(context.Context, string) error { return nil }
func (f *fakeEntityContextService) RejectMerge(context.Context, string) error { return nil }

type fakeWatchlistService struct {
	items   []service.WatchlistItem
	addUser string
	addID   string
	err     error
}

func (f *fakeWatchlistService) List(_ context.Context, _ string) ([]service.WatchlistItem, error) {
	return f.items, f.err
}
func (f *fakeWatchlistService) Add(_ context.Context, userID, instrumentID string) error {
	f.addUser, f.addID = userID, instrumentID
	return f.err
}
func (f *fakeWatchlistService) Remove(context.Context, string, string) error { return nil }

type fakeBarService struct {
	symbol    string
	timeframe string
	limit     int
	bars      []service.Bar
	err       error
}

func (f *fakeBarService) Get(_ context.Context, symbol, timeframe string, limit int) ([]service.Bar, error) {
	f.symbol, f.timeframe, f.limit = symbol, timeframe, limit
	return f.bars, f.err
}
func (f *fakeBarService) SyncBars(context.Context, service.BarSource, string, string, string, string) (int, error) {
	return 0, nil
}

type fakeSnapshotSource struct {
	quote service.Quote
	ok    bool
	err   error
}

func (f *fakeSnapshotSource) FetchSnapshot(context.Context, string) (service.Quote, bool, error) {
	return f.quote, f.ok, f.err
}

type fakeInstrumentService struct {
	resolveID string
	resolveOK bool
	err       error
}

func (f *fakeInstrumentService) Search(context.Context, string, int) ([]service.InstrumentHit, error) {
	return nil, nil
}
func (f *fakeInstrumentService) SyncAssets(context.Context, service.AssetSource) (int, error) {
	return 0, nil
}
func (f *fakeInstrumentService) ResolveInstrumentID(context.Context, string) (string, bool, error) {
	return f.resolveID, f.resolveOK, f.err
}

// --- tests -----------------------------------------------------------------

func TestSearchWorkspaceScopesToPrincipalAndEmitsCitations(t *testing.T) {
	t.Parallel()

	search := &fakeSearchService{resp: service.SearchResponse{Sections: []service.SearchSection{
		{Hits: []service.SearchHit{
			{Kind: "ticker", ID: "NVDA", Title: "NVIDIA"},
			{Kind: "entity", ID: "ent-1", Title: "Jensen Huang"},
		}},
	}}}
	tools := workspaceToolServer{search: search}

	_, out, err := tools.searchWorkspace(contextWithScopes(), nil, searchInput{Query: "nvda"})
	if err != nil {
		t.Fatalf("searchWorkspace error: %v", err)
	}
	if search.userID != "user-1" {
		t.Fatalf("search userID = %q, want user-1", search.userID)
	}
	if search.limit != 8 {
		t.Fatalf("default limit = %d, want 8", search.limit)
	}
	if len(out.Citations) != 2 || out.Citations[0].Kind != "ticker" || out.Citations[1].ID != "ent-1" {
		t.Fatalf("citations = %#v, want ticker NVDA + entity ent-1", out.Citations)
	}
}

func TestSearchWorkspaceRequiresPrincipalAndQuery(t *testing.T) {
	t.Parallel()

	tools := workspaceToolServer{search: &fakeSearchService{}}

	if _, _, err := tools.searchWorkspace(context.Background(), nil, searchInput{Query: "x"}); err == nil {
		t.Fatal("searchWorkspace without principal should error")
	}
	if _, _, err := tools.searchWorkspace(contextWithScopes(), nil, searchInput{}); err == nil {
		t.Fatal("searchWorkspace without query should error")
	}
}

func TestGetEntityScopesToPrincipalAndCites(t *testing.T) {
	t.Parallel()

	entities := &fakeEntityContextService{ec: service.EntityContext{
		Entity: service.EntityIdentity{ID: "ent-1", Name: "NVIDIA", Kind: "company"},
	}}
	tools := workspaceToolServer{entities: entities}

	_, out, err := tools.getEntity(contextWithScopes(), nil, getEntityInput{EntityID: "ent-1"})
	if err != nil {
		t.Fatalf("getEntity error: %v", err)
	}
	if entities.getUserID != "user-1" || entities.getID != "ent-1" {
		t.Fatalf("entity get scoped as (%q,%q), want (user-1, ent-1)", entities.getUserID, entities.getID)
	}
	if len(out.Citations) != 1 || out.Citations[0].Kind != "entity" || out.Citations[0].Title != "NVIDIA" {
		t.Fatalf("citations = %#v, want one entity citation", out.Citations)
	}
}

func TestGetArtifactExtractsTextAndCitesByKind(t *testing.T) {
	t.Parallel()

	artifacts := &fakeArtifactService{getResult: service.ArtifactResponse{
		ID: "app-1", Type: "app", Title: "Dashboard", Content: "shard body",
	}}
	tools := workspaceToolServer{artifacts: artifacts}

	_, out, err := tools.getArtifact(contextWithScopes(service.ScopeArtifactsRead), nil, getArtifactInput{ArtifactID: "app-1"})
	if err != nil {
		t.Fatalf("getArtifact error: %v", err)
	}
	if out.Text != "shard body" {
		t.Fatalf("text = %q, want shard body", out.Text)
	}
	if len(out.Citations) != 1 || out.Citations[0].Kind != "shard" {
		t.Fatalf("citations = %#v, want one shard citation", out.Citations)
	}
}

func TestGetQuoteDegradesWithoutSnapshots(t *testing.T) {
	t.Parallel()

	tools := workspaceToolServer{} // snapshots nil

	_, _, err := tools.getQuote(contextWithScopes(), nil, getQuoteInput{Symbol: "nvda"})
	var br service.BadRequest
	if err == nil || !errors.As(err, &br) {
		t.Fatalf("getQuote without snapshots error = %v, want bad request", err)
	}
}

func TestGetQuoteReturnsSnapshotWithCitation(t *testing.T) {
	t.Parallel()

	tools := workspaceToolServer{snapshots: &fakeSnapshotSource{quote: service.Quote{Last: 123.45}, ok: true}}

	_, out, err := tools.getQuote(contextWithScopes(), nil, getQuoteInput{Symbol: "nvda"})
	if err != nil {
		t.Fatalf("getQuote error: %v", err)
	}
	if out.Quote.Last != 123.45 {
		t.Fatalf("quote last = %v, want 123.45", out.Quote.Last)
	}
	if len(out.Citations) != 1 || out.Citations[0].ID != "NVDA" {
		t.Fatalf("citations = %#v, want uppercased NVDA ticker", out.Citations)
	}
}

func TestGetBarsUppercasesAndDefaultsLimit(t *testing.T) {
	t.Parallel()

	bars := &fakeBarService{}
	tools := workspaceToolServer{bars: bars}

	_, out, err := tools.getBars(contextWithScopes(), nil, getBarsInput{Symbol: "nvda"})
	if err != nil {
		t.Fatalf("getBars error: %v", err)
	}
	if bars.symbol != "NVDA" || bars.limit != 30 {
		t.Fatalf("bars fetched as (%q,%d), want (NVDA,30)", bars.symbol, bars.limit)
	}
	if out.Symbol != "NVDA" || len(out.Citations) != 1 || out.Citations[0].Kind != "ticker" {
		t.Fatalf("output = %#v, want NVDA + ticker citation", out)
	}
}

func TestAddToWatchlistResolvesSymbol(t *testing.T) {
	t.Parallel()

	instruments := &fakeInstrumentService{resolveID: "inst-1", resolveOK: true}
	watchlist := &fakeWatchlistService{}
	tools := workspaceToolServer{instruments: instruments, watchlist: watchlist}

	_, out, err := tools.addToWatchlist(contextWithScopes(), nil, addToWatchlistInput{Symbol: "nvda"})
	if err != nil {
		t.Fatalf("addToWatchlist error: %v", err)
	}
	if !out.OK || out.Symbol != "NVDA" {
		t.Fatalf("output = %#v, want ok NVDA", out)
	}
	if watchlist.addUser != "user-1" || watchlist.addID != "inst-1" {
		t.Fatalf("watchlist add = (%q,%q), want (user-1, inst-1)", watchlist.addUser, watchlist.addID)
	}
}

func TestAddToWatchlistUnknownSymbolIsNotAnError(t *testing.T) {
	t.Parallel()

	tools := workspaceToolServer{instruments: &fakeInstrumentService{}, watchlist: &fakeWatchlistService{}}

	_, out, err := tools.addToWatchlist(contextWithScopes(), nil, addToWatchlistInput{Symbol: "zzzz"})
	if err != nil {
		t.Fatalf("addToWatchlist error: %v", err)
	}
	if out.OK || out.Note == "" {
		t.Fatalf("output = %#v, want ok=false with a note", out)
	}
}

func TestDrawEdgeUsesPrincipalAsOwner(t *testing.T) {
	t.Parallel()

	entities := &fakeEntityContextService{}
	tools := workspaceToolServer{entities: entities}

	_, out, err := tools.drawEdge(contextWithScopes(), nil, drawEdgeInput{FromID: "a", ToID: "b", Rel: "competes_with"})
	if err != nil {
		t.Fatalf("drawEdge error: %v", err)
	}
	if !out.OK {
		t.Fatal("drawEdge output not ok")
	}
	if entities.drawn == nil || entities.drawn.OwnerUserID != "user-1" || entities.drawn.Rel != "competes_with" {
		t.Fatalf("drawn edge = %#v, want owner user-1 competes_with", entities.drawn)
	}
}

func TestDrawEdgeValidatesRequiredFields(t *testing.T) {
	t.Parallel()

	tools := workspaceToolServer{entities: &fakeEntityContextService{}}

	if _, _, err := tools.drawEdge(contextWithScopes(), nil, drawEdgeInput{FromID: "a"}); err == nil {
		t.Fatal("drawEdge without to_id/rel should error")
	}
}
