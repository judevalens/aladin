package mcpserver

import (
	"context"
	"errors"
	"strings"
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

type fakeMarketInfo struct {
	news      []service.NewsArticle
	movers    service.MoversResult
	actives   []service.ActiveStock
	account   service.AccountSummary
	positions []service.PositionView
	newsSyms  string
	err       error
}

func (f *fakeMarketInfo) News(_ context.Context, symbols string, _ int) ([]service.NewsArticle, error) {
	f.newsSyms = symbols
	return f.news, f.err
}
func (f *fakeMarketInfo) Movers(context.Context, int) (service.MoversResult, error) {
	return f.movers, f.err
}
func (f *fakeMarketInfo) MostActives(context.Context, int) ([]service.ActiveStock, error) {
	return f.actives, f.err
}
func (f *fakeMarketInfo) Account(context.Context) (service.AccountSummary, error) {
	return f.account, f.err
}
func (f *fakeMarketInfo) Positions(context.Context) ([]service.PositionView, error) {
	return f.positions, f.err
}

type fakeAlertService struct {
	created   *service.Alert
	warning   string
	createErr error
	list      []service.Alert
	deleted   string
}

func (f *fakeAlertService) Create(_ context.Context, userID, symbol, direction string, threshold float64) (service.CreateAlertResult, error) {
	if f.createErr != nil {
		return service.CreateAlertResult{}, f.createErr
	}
	a := service.Alert{ID: "al1", UserID: userID, Symbol: strings.ToUpper(symbol), Direction: direction, Threshold: threshold, Armed: true, Status: "active"}
	f.created = &a
	return service.CreateAlertResult{Alert: a, Warning: f.warning}, nil
}
func (f *fakeAlertService) List(context.Context, string) ([]service.Alert, error) {
	return f.list, nil
}
func (f *fakeAlertService) Delete(_ context.Context, _, id string) error {
	f.deleted = id
	return nil
}
func (f *fakeAlertService) Pause(context.Context, string, string) error { return nil }

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

func TestGetNewsUppercasesFilterAndCitesSymbols(t *testing.T) {
	t.Parallel()

	mi := &fakeMarketInfo{news: []service.NewsArticle{
		{ID: 1, Headline: "Tesla sinks", Symbols: []string{"TSLA", "AMZN"}},
		{ID: 2, Headline: "More", Symbols: []string{"TSLA"}}, // dup ticker collapses
	}}
	tools := workspaceToolServer{marketInfo: mi}

	_, out, err := tools.getNews(contextWithScopes(), nil, getNewsInput{Symbols: "tsla"})
	if err != nil {
		t.Fatalf("getNews error: %v", err)
	}
	if mi.newsSyms != "TSLA" {
		t.Fatalf("news filter = %q, want uppercased TSLA", mi.newsSyms)
	}
	if len(out.Citations) != 2 {
		t.Fatalf("citations = %#v, want distinct TSLA + AMZN", out.Citations)
	}
}

func TestMarketInfoToolsDegradeWithoutConfig(t *testing.T) {
	t.Parallel()

	tools := workspaceToolServer{} // marketInfo nil
	var br service.BadRequest
	if _, _, err := tools.getNews(contextWithScopes(), nil, getNewsInput{}); !errors.As(err, &br) {
		t.Fatalf("getNews without config = %v, want bad request", err)
	}
	if _, _, err := tools.getPositions(contextWithScopes(), nil, emptyInput{}); !errors.As(err, &br) {
		t.Fatalf("getPositions without config = %v, want bad request", err)
	}
	if _, _, err := tools.getAccount(contextWithScopes(), nil, emptyInput{}); !errors.As(err, &br) {
		t.Fatalf("getAccount without config = %v, want bad request", err)
	}
}

func TestCreateAlertScopesToPrincipalAndCites(t *testing.T) {
	t.Parallel()

	as := &fakeAlertService{warning: "already above"}
	tools := workspaceToolServer{alerts: as}

	_, out, err := tools.createAlert(contextWithScopes(), nil, createAlertInput{Symbol: "nvda", Direction: "above", Threshold: 215})
	if err != nil {
		t.Fatalf("createAlert error: %v", err)
	}
	if as.created == nil || as.created.UserID != "user-1" || as.created.Symbol != "NVDA" {
		t.Fatalf("created alert = %#v, want NVDA for user-1", as.created)
	}
	if out.Warning != "already above" {
		t.Fatalf("warning not propagated: %q", out.Warning)
	}
	if len(out.Citations) != 1 || out.Citations[0].ID != "NVDA" {
		t.Fatalf("citations = %#v, want NVDA ticker", out.Citations)
	}
}

func TestAlertToolsDegradeWithoutConfig(t *testing.T) {
	t.Parallel()

	tools := workspaceToolServer{} // alerts nil
	var br service.BadRequest
	if _, _, err := tools.createAlert(contextWithScopes(), nil, createAlertInput{Symbol: "x", Direction: "above", Threshold: 1}); !errors.As(err, &br) {
		t.Fatalf("createAlert without config = %v, want bad request", err)
	}
	if _, _, err := tools.listAlerts(contextWithScopes(), nil, emptyInput{}); !errors.As(err, &br) {
		t.Fatalf("listAlerts without config = %v, want bad request", err)
	}
}

func TestGetPositionsCitesEachHolding(t *testing.T) {
	t.Parallel()

	mi := &fakeMarketInfo{positions: []service.PositionView{
		{Symbol: "NVDA", Qty: "99", UnrealizedPL: "107.84"},
	}}
	tools := workspaceToolServer{marketInfo: mi}

	_, out, err := tools.getPositions(contextWithScopes(), nil, emptyInput{})
	if err != nil {
		t.Fatalf("getPositions error: %v", err)
	}
	if len(out.Positions) != 1 || len(out.Citations) != 1 || out.Citations[0].ID != "NVDA" {
		t.Fatalf("output = %#v, want one NVDA position + citation", out)
	}
}
