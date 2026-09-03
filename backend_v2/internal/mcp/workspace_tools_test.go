package mcpserver

import (
	"aladin/backend_v2/internal/alert"
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	"aladin/backend_v2/internal/entities"
	"aladin/backend_v2/internal/instrument"
	"aladin/backend_v2/internal/market"
	searchdomain "aladin/backend_v2/internal/search"
	"aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/watchlist"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- fakes -----------------------------------------------------------------

type fakeSearchService struct {
	userID string
	query  string
	limit  int
	resp   searchdomain.SearchResponse
	err    error
}

func (f *fakeSearchService) Search(_ context.Context, userID, query string, limit int) (searchdomain.SearchResponse, error) {
	f.userID, f.query, f.limit = userID, query, limit
	return f.resp, f.err
}

type fakeEntityContextService struct {
	getUserID string
	getID     string
	ec        entities.EntityContext
	drawn     *entities.DrawEdgeInput
	err       error
}

func (f *fakeEntityContextService) Get(_ context.Context, ownerUserID, entityID string) (entities.EntityContext, error) {
	f.getUserID, f.getID = ownerUserID, entityID
	return f.ec, f.err
}
func (f *fakeEntityContextService) DrawEdge(_ context.Context, in entities.DrawEdgeInput) error {
	copyIn := in
	f.drawn = &copyIn
	return f.err
}
func (f *fakeEntityContextService) MergeQueue(context.Context, int) ([]entities.MergeQueueItem, error) {
	return nil, nil
}
func (f *fakeEntityContextService) AcceptMerge(context.Context, string) error { return nil }
func (f *fakeEntityContextService) RejectMerge(context.Context, string) error { return nil }

type fakeWatchlistService struct {
	items       []watchlist.WatchlistItem
	lists       []watchlist.Watchlist
	listUser    string
	listID      string
	listsUser   string
	createUser  string
	addUser     string
	addListID   string
	addID       string
	resolveUser string
	resolved    string // the name passed to ResolveOrCreateByName
	createdWith string
	err         error
}

func (f *fakeWatchlistService) ListWatchlists(_ context.Context, userID string) ([]watchlist.Watchlist, error) {
	f.listsUser = userID
	return f.lists, f.err
}
func (f *fakeWatchlistService) CreateWatchlist(_ context.Context, userID, name string) (watchlist.Watchlist, error) {
	f.createUser = userID
	f.createdWith = name
	return watchlist.Watchlist{ID: "new-list", Name: name, Kind: "manual"}, f.err
}
func (f *fakeWatchlistService) RenameWatchlist(context.Context, string, string, string) error {
	return nil
}
func (f *fakeWatchlistService) DeleteWatchlist(context.Context, string, string) error { return nil }
func (f *fakeWatchlistService) ListItems(_ context.Context, userID, listID string) ([]watchlist.WatchlistItem, error) {
	f.listUser, f.listID = userID, listID
	return f.items, f.err
}
func (f *fakeWatchlistService) AddItem(_ context.Context, userID, listID, instrumentID string) error {
	f.addUser, f.addListID, f.addID = userID, listID, instrumentID
	return f.err
}
func (f *fakeWatchlistService) RemoveItem(context.Context, string, string, string) error { return nil }
func (f *fakeWatchlistService) ResolveOrCreateByName(_ context.Context, userID, name string) (string, error) {
	f.resolveUser = userID
	f.resolved = name
	if name == "" {
		return "default-list", f.err
	}
	return "resolved-" + name, f.err
}
func (f *fakeWatchlistService) ResolveInstruments(context.Context, string, string) ([]watchlist.WatchlistItem, error) {
	return f.items, f.err
}
func (f *fakeWatchlistService) List(context.Context, string) ([]watchlist.WatchlistItem, error) {
	return f.items, f.err
}
func (f *fakeWatchlistService) Add(context.Context, string, string) error    { return f.err }
func (f *fakeWatchlistService) Remove(context.Context, string, string) error { return nil }

type fakeBarService struct {
	symbol    string
	timeframe string
	limit     int
	bars      []market.Bar
	err       error
}

func (f *fakeBarService) Get(_ context.Context, symbol, timeframe string, limit int) ([]market.Bar, error) {
	f.symbol, f.timeframe, f.limit = symbol, timeframe, limit
	return f.bars, f.err
}
func (f *fakeBarService) GetAdjusted(ctx context.Context, symbol, timeframe string, limit int, _ market.AdjustMode) ([]market.Bar, error) {
	return f.Get(ctx, symbol, timeframe, limit)
}
func (f *fakeBarService) SyncCorporateActions(context.Context, market.CorporateActionSource, string, string, string) (int, error) {
	return 0, nil
}
func (f *fakeBarService) SyncBars(context.Context, market.BarSource, string, string, string, string) (int, error) {
	return 0, nil
}

type fakeSnapshotSource struct {
	quote market.Quote
	ok    bool
	err   error
}

func (f *fakeSnapshotSource) FetchSnapshot(context.Context, string) (market.Quote, bool, error) {
	return f.quote, f.ok, f.err
}

type fakeInstrumentService struct {
	resolveID string
	resolveOK bool
	err       error
}

func (f *fakeInstrumentService) Search(context.Context, string, int) ([]instrument.InstrumentHit, error) {
	return nil, nil
}
func (f *fakeInstrumentService) SyncAssets(context.Context, instrument.AssetSource) (int, error) {
	return 0, nil
}
func (f *fakeInstrumentService) ResolveInstrumentID(context.Context, string) (string, bool, error) {
	return f.resolveID, f.resolveOK, f.err
}

type fakeMarketInfo struct {
	news      []market.NewsArticle
	movers    market.MoversResult
	actives   []market.ActiveStock
	account   market.AccountSummary
	positions []market.PositionView
	newsSyms  string
	err       error
}

func (f *fakeMarketInfo) News(_ context.Context, symbols string, _ int) ([]market.NewsArticle, error) {
	f.newsSyms = symbols
	return f.news, f.err
}
func (f *fakeMarketInfo) Movers(context.Context, int) (market.MoversResult, error) {
	return f.movers, f.err
}
func (f *fakeMarketInfo) MostActives(context.Context, int) ([]market.ActiveStock, error) {
	return f.actives, f.err
}
func (f *fakeMarketInfo) Account(context.Context) (market.AccountSummary, error) {
	return f.account, f.err
}
func (f *fakeMarketInfo) Positions(context.Context) ([]market.PositionView, error) {
	return f.positions, f.err
}

type fakeAlertService struct {
	created   *alert.Alert
	warning   string
	createErr error
	list      []alert.Alert
	deleted   string
}

func (f *fakeAlertService) Create(_ context.Context, userID, symbol, direction string, threshold float64) (alert.CreateAlertResult, error) {
	if f.createErr != nil {
		return alert.CreateAlertResult{}, f.createErr
	}
	a := alert.Alert{ID: "al1", UserID: userID, Symbol: strings.ToUpper(symbol), Direction: direction, Threshold: threshold, Armed: true, Status: "active"}
	f.created = &a
	return alert.CreateAlertResult{Alert: a, Warning: f.warning}, nil
}
func (f *fakeAlertService) List(context.Context, string) ([]alert.Alert, error) {
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

	search := &fakeSearchService{resp: searchdomain.SearchResponse{Sections: []searchdomain.SearchSection{
		{Hits: []searchdomain.SearchHit{
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

	entities := &fakeEntityContextService{ec: entities.EntityContext{
		Entity: entities.EntityIdentity{ID: "ent-1", Name: "NVIDIA", Kind: "company"},
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

	tools := workspaceToolServer{snapshots: &fakeSnapshotSource{quote: market.Quote{Last: 123.45}, ok: true}}

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

func TestGetWatchlistScopesDefaultAndNamedListsToPrincipal(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name       string
		input      getWatchlistInput
		wantListID string
		wantName   string
	}{
		{name: "default", input: getWatchlistInput{}, wantListID: ""},
		{name: "named", input: getWatchlistInput{List: "Semis"}, wantListID: "resolved-Semis", wantName: "Semis"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			watchlist := &fakeWatchlistService{items: []watchlist.WatchlistItem{{InstrumentID: "inst-1", Symbol: "NVDA"}}}
			tools := workspaceToolServer{watchlist: watchlist}
			_, out, err := tools.getWatchlist(contextWithScopes(), nil, tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if len(out.Items) != 1 || watchlist.listUser != "user-1" || watchlist.listID != tt.wantListID {
				t.Fatalf("out=%#v list=(%q,%q), want user-1/%q", out, watchlist.listUser, watchlist.listID, tt.wantListID)
			}
			if watchlist.resolved != tt.wantName || (tt.wantName != "" && watchlist.resolveUser != "user-1") {
				t.Fatalf("resolve=(%q,%q), want user-1/%q", watchlist.resolveUser, watchlist.resolved, tt.wantName)
			}
		})
	}
}

func TestListWatchlistsScopesToPrincipal(t *testing.T) {
	t.Parallel()
	watchlist := &fakeWatchlistService{lists: []watchlist.Watchlist{{ID: "l1", Name: "Tech", ItemCount: 3}}}
	tools := workspaceToolServer{watchlist: watchlist}
	_, out, err := tools.listWatchlists(contextWithScopes(), nil, emptyInput{})
	if err != nil {
		t.Fatal(err)
	}
	if watchlist.listsUser != "user-1" || len(out.Watchlists) != 1 || out.Watchlists[0].ItemCount != 3 {
		t.Fatalf("user=%q output=%#v", watchlist.listsUser, out)
	}
}

func TestWatchlistWorkspaceToolsRequirePrincipal(t *testing.T) {
	t.Parallel()
	tools := workspaceToolServer{}
	ctx := context.Background()
	tests := []struct {
		name string
		call func() error
	}{
		{name: "get", call: func() error { _, _, err := tools.getWatchlist(ctx, nil, getWatchlistInput{}); return err }},
		{name: "list", call: func() error { _, _, err := tools.listWatchlists(ctx, nil, emptyInput{}); return err }},
		{name: "create", call: func() error {
			_, _, err := tools.createWatchlist(ctx, nil, createWatchlistInput{Name: "Tech"})
			return err
		}},
		{name: "add", call: func() error {
			_, _, err := tools.addToWatchlist(ctx, nil, addToWatchlistInput{Symbol: "NVDA"})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); !errors.Is(err, service.ErrUnauthenticated) {
				t.Fatalf("error = %v, want ErrUnauthenticated", err)
			}
		})
	}
}

func TestWatchlistToolSchemas(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "watchlist-contract", Version: "test"}, nil)
	registerWorkspaceTools(server, workspaceToolServer{})
	clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "watchlist-contract-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	listed, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]*sdkmcp.Tool, len(listed.Tools))
	for _, tool := range listed.Tools {
		byName[tool.Name] = tool
	}

	tests := []struct {
		name           string
		description    string
		inputProps     []string
		inputRequired  []string
		outputProps    []string
		outputRequired []string
	}{
		{name: "get_watchlist", description: "List the tickers in a watchlist. Optionally pass a list name; omit for the user's default list.", inputProps: []string{"list"}, outputProps: []string{"items"}, outputRequired: []string{"items"}},
		{name: "list_watchlists", description: "List the user's watchlists (named instrument sets) with their item counts.", outputProps: []string{"watchlists"}, outputRequired: []string{"watchlists"}},
		{name: "create_watchlist", description: "Create a new named watchlist. Returns its id.", inputProps: []string{"name"}, inputRequired: []string{"name"}, outputProps: []string{"id", "name"}, outputRequired: []string{"id", "name"}},
		{name: "add_to_watchlist", description: "Add a ticker to a watchlist by symbol. Optionally pass a list name (created if it doesn't exist); omit for the user's default list.", inputProps: []string{"list", "symbol"}, inputRequired: []string{"symbol"}, outputProps: []string{"citations", "list", "note", "ok", "symbol"}, outputRequired: []string{"ok", "symbol"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := byName[tt.name]
			if tool == nil {
				t.Fatalf("tool %q not registered", tt.name)
			}
			if tool.Description != tt.description {
				t.Fatalf("description = %q", tool.Description)
			}
			assertToolSchema(t, "input", tool.InputSchema, tt.inputProps, tt.inputRequired)
			assertToolSchema(t, "output", tool.OutputSchema, tt.outputProps, tt.outputRequired)
		})
	}
}

func assertToolSchema(t *testing.T, label string, raw any, wantProps, wantRequired []string) {
	t.Helper()
	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(encoded, &schema); err != nil {
		t.Fatal(err)
	}
	properties, _ := schema["properties"].(map[string]any)
	gotProps := make([]string, 0, len(properties))
	for key := range properties {
		gotProps = append(gotProps, key)
	}
	sort.Strings(gotProps)
	sort.Strings(wantProps)
	if !equalStrings(gotProps, wantProps) {
		t.Fatalf("%s properties = %v, want %v; schema=%s", label, gotProps, wantProps, encoded)
	}
	gotRequired := make([]string, 0)
	if required, ok := schema["required"].([]any); ok {
		for _, value := range required {
			gotRequired = append(gotRequired, value.(string))
		}
	}
	sort.Strings(gotRequired)
	sort.Strings(wantRequired)
	if !equalStrings(gotRequired, wantRequired) {
		t.Fatalf("%s required = %v, want %v; schema=%s", label, gotRequired, wantRequired, encoded)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestAddToWatchlistResolvesSymbol(t *testing.T) {
	t.Parallel()

	instruments := &fakeInstrumentService{resolveID: "inst-1", resolveOK: true}
	watchlist := &fakeWatchlistService{}
	tools := workspaceToolServer{instruments: instruments, watchlist: watchlist}

	// No list name → the default list.
	_, out, err := tools.addToWatchlist(contextWithScopes(), nil, addToWatchlistInput{Symbol: "nvda"})
	if err != nil {
		t.Fatalf("addToWatchlist error: %v", err)
	}
	if !out.OK || out.Symbol != "NVDA" {
		t.Fatalf("output = %#v, want ok NVDA", out)
	}
	if len(out.Citations) != 1 || out.Citations[0].Kind != "ticker" || out.Citations[0].ID != "NVDA" || out.Citations[0].Title != "NVDA" {
		t.Fatalf("citations = %#v, want NVDA ticker citation", out.Citations)
	}
	if watchlist.addUser != "user-1" || watchlist.addID != "inst-1" || watchlist.addListID != "default-list" {
		t.Fatalf("watchlist add = (%q,%q,%q), want (user-1, inst-1, default-list)", watchlist.addUser, watchlist.addID, watchlist.addListID)
	}
}

func TestAddToWatchlistNamedListResolvesOrCreates(t *testing.T) {
	t.Parallel()

	instruments := &fakeInstrumentService{resolveID: "inst-1", resolveOK: true}
	watchlist := &fakeWatchlistService{}
	tools := workspaceToolServer{instruments: instruments, watchlist: watchlist}

	_, out, err := tools.addToWatchlist(contextWithScopes(), nil, addToWatchlistInput{Symbol: "nvda", List: "Semis"})
	if err != nil {
		t.Fatalf("addToWatchlist error: %v", err)
	}
	if watchlist.resolved != "Semis" {
		t.Fatalf("ResolveOrCreateByName got %q, want Semis", watchlist.resolved)
	}
	if watchlist.addListID != "resolved-Semis" || out.List != "Semis" {
		t.Fatalf("added to list %q (out.List %q), want resolved-Semis / Semis", watchlist.addListID, out.List)
	}
}

func TestCreateWatchlistTool(t *testing.T) {
	t.Parallel()
	watchlist := &fakeWatchlistService{}
	tools := workspaceToolServer{watchlist: watchlist}
	_, out, err := tools.createWatchlist(contextWithScopes(), nil, createWatchlistInput{Name: "Shorts"})
	if err != nil {
		t.Fatalf("createWatchlist error: %v", err)
	}
	if watchlist.createUser != "user-1" || watchlist.createdWith != "Shorts" || out.Name != "Shorts" {
		t.Fatalf("created by %q with %q (out %q), want user-1/Shorts", watchlist.createUser, watchlist.createdWith, out.Name)
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

	mi := &fakeMarketInfo{news: []market.NewsArticle{
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

	mi := &fakeMarketInfo{positions: []market.PositionView{
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

func TestSummarizeBoardContentIncludesLinks(t *testing.T) {
	snapshot := `{"document":{"store":{
		"shape:l1":{"typeName":"shape","type":"aladin-link","props":{
			"url":"https://ssrn.com/momentum","title":"Momentum Crashes","domain":"ssrn.com",
			"description":"Momentum strategies crash in panic states.","status":"ready"}},
		"shape:l2":{"typeName":"shape","type":"aladin-link","props":{
			"url":"https://example.com/raw","title":"","domain":"example.com","status":"failed"}},
		"shape:t1":{"typeName":"shape","type":"aladin-task","props":{"text":"read §4"}}
	}}}`
	got := summarizeBoardContent(snapshot)
	if !strings.Contains(got, "2 link") {
		t.Fatalf("counts missing links: %q", got)
	}
	if !strings.Contains(got, "link: Momentum Crashes — https://ssrn.com/momentum [ssrn.com] :: Momentum strategies crash in panic states.") {
		t.Fatalf("unfurled link line missing: %q", got)
	}
	// A link that never unfurled still surfaces by URL — the agent can follow it.
	if !strings.Contains(got, "link: https://example.com/raw [example.com]") {
		t.Fatalf("bare link line missing: %q", got)
	}
}
