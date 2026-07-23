package mcpserver

import (
	"context"
	"strings"

	"aladin/backend_v2/internal/blocknote"
	"aladin/backend_v2/internal/service"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Workspace tools — the read/light-write surface the copilot (and any MCP
// client) uses to ground answers in the user's Aladin data: federated search,
// entities, insights, artifacts, watchlist, and market data. Ported from the
// in-process copilot tool dispatch when the copilot moved to the Claude Agent
// SDK sidecar; all scoping comes from the bearer principal on ctx.

// citationOut rides in tool outputs so a consumer (the copilot's Go stream
// translator) can accumulate grounding chips: kind routes the client nav
// (ticker → markets, entity → /entity/:id, page/shard → work pane).
type citationOut struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
}

// workspaceToolServer carries the deps the workspace tools need. snapshots and
// marketInfo may be nil (no market-data keys) — the dependent tools degrade with
// a clear "not configured" error.
type workspaceToolServer struct {
	search      service.SearchService
	entities    service.EntityContextService
	insights    service.InsightService
	artifacts   service.ArtifactService
	watchlist   service.WatchlistService
	bars        service.BarService
	snapshots   service.QuoteSnapshotSource
	marketInfo  service.MarketInfoService
	instruments service.InstrumentService
}

func registerWorkspaceTools(server *sdkmcp.Server, t workspaceToolServer) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "search",
		Description: "Federated search over the user's Aladin workspace: tickers, entities (companies/people), pages, and shards. Use this first to find the ids of things to look up. Hits ride back with citations.",
	}, t.searchWorkspace)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_entity",
		Description: "Fetch one entity's identity, its typed relationships (edges), and the verbatim material accreted under it. entity_id comes from a search hit of kind entity/company/person.",
	}, t.getEntity)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_insights",
		Description: "List engine-generated insights (discourse/bridges over the user's connected sources). Optionally filter by type or status.",
	}, t.getInsights)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_artifacts",
		Description: "List the user's artifacts as {id,title,type}. type is one of page, app (a shard — an agent-built interactive doc), link, file, voice. Use get_artifact to read one.",
	}, t.listArtifacts)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_artifact",
		Description: "Read one artifact by id — works for ANY kind: a page (returns its text), a shard/app (returns its content + metadata), a link, a file, or a voice note. For block-level page edits use get_page instead.",
	}, t.getArtifact)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_watchlist",
		Description: "List the tickers the user is currently tracking on the Markets surface.",
	}, t.getWatchlist)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_bars",
		Description: "OHLCV price history for a ticker symbol. timeframe is e.g. 1Day or 5Min; returns oldest→newest.",
	}, t.getBars)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_quote",
		Description: "Current last price + previous close for a ticker symbol (snapshot). Errors when live market data is not configured.",
	}, t.getQuote)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "add_to_watchlist",
		Description: "Add a ticker to the user's Markets watchlist by symbol.",
	}, t.addToWatchlist)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "draw_edge",
		Description: "Draw a typed relationship edge between two entities (e.g. rel \"competes_with\"). Additive.",
	}, t.drawEdge)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_news",
		Description: "Recent market news/headlines (Benzinga via Alpaca), newest first. Optionally filter to a comma-separated symbol list. Use this to explain WHY a stock is moving — a catalyst vs. a liquidity move.",
	}, t.getNews)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_movers",
		Description: "Today's top gainers and losers across the US market (consolidated tape). Answers \"what's moving today\" without naming a symbol first. Note: includes low-priced/low-float names with extreme % moves.",
	}, t.getMovers)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_most_actives",
		Description: "Today's highest-volume US stocks (most-actives screener). Use to gauge where liquidity and attention are concentrated.",
	}, t.getMostActives)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_account",
		Description: "The user's trading account summary: cash, equity, buying power, portfolio value. Read-only. `paper` flags whether this is a paper (simulated) account — caveat the numbers accordingly.",
	}, t.getAccount)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_positions",
		Description: "The user's open positions: symbol, qty, side, avg entry, market value, and unrealized P&L. Read-only — this reasons about ACTUAL exposure, not the abstract watchlist. Cannot place or modify orders.",
	}, t.getPositions)
}

// --- inputs / outputs ------------------------------------------------------

type searchInput struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}
type searchOutput struct {
	Results   service.SearchResponse `json:"results"`
	Citations []citationOut          `json:"citations,omitempty"`
}

type getEntityInput struct {
	EntityID string `json:"entity_id"`
}
type getEntityOutput struct {
	Entity    service.EntityContext `json:"entity"`
	Citations []citationOut         `json:"citations,omitempty"`
}

type getInsightsInput struct {
	Limit  int    `json:"limit,omitempty"`
	Type   string `json:"type,omitempty"`
	Status string `json:"status,omitempty"`
}
type getInsightsOutput struct {
	Insights map[string]any `json:"insights"`
}

type listArtifactsInput struct {
	Type string `json:"type,omitempty"`
}
type artifactSummaryOut struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}
type listArtifactsOutput struct {
	Artifacts []artifactSummaryOut `json:"artifacts"`
}

type getArtifactInput struct {
	ArtifactID string `json:"artifact_id"`
}
type getArtifactOutput struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	Summary   *string        `json:"summary,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Citations []citationOut  `json:"citations,omitempty"`
}

type getWatchlistOutput struct {
	Items []service.WatchlistItem `json:"items"`
}

type getBarsInput struct {
	Symbol    string `json:"symbol"`
	Timeframe string `json:"timeframe,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}
type getBarsOutput struct {
	Symbol    string        `json:"symbol"`
	Bars      []service.Bar `json:"bars"`
	Citations []citationOut `json:"citations,omitempty"`
}

type getQuoteInput struct {
	Symbol string `json:"symbol"`
}
type getQuoteOutput struct {
	Quote     service.Quote `json:"quote"`
	Citations []citationOut `json:"citations,omitempty"`
}

type addToWatchlistInput struct {
	Symbol string `json:"symbol"`
}
type addToWatchlistOutput struct {
	OK        bool          `json:"ok"`
	Symbol    string        `json:"symbol"`
	Note      string        `json:"note,omitempty"`
	Citations []citationOut `json:"citations,omitempty"`
}

type drawEdgeInput struct {
	FromID string `json:"from_id"`
	ToID   string `json:"to_id"`
	Rel    string `json:"rel"`
	Why    string `json:"why,omitempty"`
}
type drawEdgeOutput struct {
	OK bool `json:"ok"`
}

type getNewsInput struct {
	Symbols string `json:"symbols,omitempty"` // comma-separated; empty = general market news
	Limit   int    `json:"limit,omitempty"`
}
type getNewsOutput struct {
	News      []service.NewsArticle `json:"news"`
	Citations []citationOut         `json:"citations,omitempty"`
}

type topInput struct {
	Top int `json:"top,omitempty"`
}
type getMoversOutput struct {
	Movers service.MoversResult `json:"movers"`
}
type getMostActivesOutput struct {
	MostActives []service.ActiveStock `json:"mostActives"`
}

type getAccountOutput struct {
	Account service.AccountSummary `json:"account"`
}
type getPositionsOutput struct {
	Positions []service.PositionView `json:"positions"`
	Citations []citationOut          `json:"citations,omitempty"`
}

// --- handlers --------------------------------------------------------------

func (t workspaceToolServer) searchWorkspace(ctx context.Context, _ *sdkmcp.CallToolRequest, in searchInput) (*sdkmcp.CallToolResult, searchOutput, error) {
	principal, err := service.RequirePrincipal(ctx)
	if err != nil {
		return nil, searchOutput{}, err
	}
	if strings.TrimSpace(in.Query) == "" {
		return nil, searchOutput{}, service.BadRequest("query is required")
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 8
	}
	resp, err := t.search.Search(ctx, principal.UserID, in.Query, limit)
	if err != nil {
		return nil, searchOutput{}, err
	}
	var cites []citationOut
	for _, sec := range resp.Sections {
		for _, h := range sec.Hits {
			cites = append(cites, citationOut{Kind: h.Kind, ID: h.ID, Title: h.Title})
		}
	}
	return nil, searchOutput{Results: resp, Citations: cites}, nil
}

func (t workspaceToolServer) getEntity(ctx context.Context, _ *sdkmcp.CallToolRequest, in getEntityInput) (*sdkmcp.CallToolResult, getEntityOutput, error) {
	principal, err := service.RequirePrincipal(ctx)
	if err != nil {
		return nil, getEntityOutput{}, err
	}
	if strings.TrimSpace(in.EntityID) == "" {
		return nil, getEntityOutput{}, service.BadRequest("entity_id is required")
	}
	ec, err := t.entities.Get(ctx, principal.UserID, in.EntityID)
	if err != nil {
		return nil, getEntityOutput{}, err
	}
	return nil, getEntityOutput{
		Entity:    ec,
		Citations: []citationOut{{Kind: "entity", ID: ec.Entity.ID, Title: ec.Entity.Name}},
	}, nil
}

func (t workspaceToolServer) getInsights(ctx context.Context, _ *sdkmcp.CallToolRequest, in getInsightsInput) (*sdkmcp.CallToolResult, getInsightsOutput, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}
	res, err := t.insights.List(ctx, service.InsightListParams{Limit: limit, Type: in.Type, Status: in.Status})
	if err != nil {
		return nil, getInsightsOutput{}, err
	}
	return nil, getInsightsOutput{Insights: res}, nil
}

func (t workspaceToolServer) listArtifacts(ctx context.Context, _ *sdkmcp.CallToolRequest, in listArtifactsInput) (*sdkmcp.CallToolResult, listArtifactsOutput, error) {
	items, err := t.artifacts.List(ctx, service.ArtifactListParams{})
	if err != nil {
		return nil, listArtifactsOutput{}, err
	}
	filter := strings.TrimSpace(in.Type)
	out := make([]artifactSummaryOut, 0, len(items))
	for _, it := range items {
		if filter != "" && it.Type != filter {
			continue
		}
		out = append(out, artifactSummaryOut{ID: it.ID, Title: it.Title, Type: it.Type})
	}
	return nil, listArtifactsOutput{Artifacts: out}, nil
}

func (t workspaceToolServer) getArtifact(ctx context.Context, _ *sdkmcp.CallToolRequest, in getArtifactInput) (*sdkmcp.CallToolResult, getArtifactOutput, error) {
	if strings.TrimSpace(in.ArtifactID) == "" {
		return nil, getArtifactOutput{}, service.BadRequest("artifact_id is required")
	}
	art, err := t.artifacts.Get(ctx, in.ArtifactID)
	if err != nil {
		return nil, getArtifactOutput{}, err
	}
	// Hand the model readable text, not raw BlockNote JSON: a page's body is its
	// blocks (extract the inline text); other kinds carry plain text in Content.
	body := art.Content
	if len(art.Blocks) > 0 {
		if text, terr := blocknote.ExtractText(art.Blocks); terr == nil && strings.TrimSpace(text) != "" {
			body = text
		}
	}
	return nil, getArtifactOutput{
		ID: art.ID, Title: art.Title, Type: art.Type,
		Text: body, Summary: art.Summary, Metadata: art.Metadata,
		Citations: []citationOut{{Kind: citationKindForArtifact(art.Type), ID: art.ID, Title: art.Title}},
	}, nil
}

func (t workspaceToolServer) getWatchlist(ctx context.Context, _ *sdkmcp.CallToolRequest, _ emptyInput) (*sdkmcp.CallToolResult, getWatchlistOutput, error) {
	principal, err := service.RequirePrincipal(ctx)
	if err != nil {
		return nil, getWatchlistOutput{}, err
	}
	items, err := t.watchlist.List(ctx, principal.UserID)
	if err != nil {
		return nil, getWatchlistOutput{}, err
	}
	return nil, getWatchlistOutput{Items: items}, nil
}

func (t workspaceToolServer) getBars(ctx context.Context, _ *sdkmcp.CallToolRequest, in getBarsInput) (*sdkmcp.CallToolResult, getBarsOutput, error) {
	sym := strings.ToUpper(strings.TrimSpace(in.Symbol))
	if sym == "" {
		return nil, getBarsOutput{}, service.BadRequest("symbol is required")
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 30
	}
	bars, err := t.bars.Get(ctx, sym, in.Timeframe, limit)
	if err != nil {
		return nil, getBarsOutput{}, err
	}
	return nil, getBarsOutput{
		Symbol:    sym,
		Bars:      bars,
		Citations: []citationOut{{Kind: "ticker", ID: sym, Title: sym}},
	}, nil
}

func (t workspaceToolServer) getQuote(ctx context.Context, _ *sdkmcp.CallToolRequest, in getQuoteInput) (*sdkmcp.CallToolResult, getQuoteOutput, error) {
	if t.snapshots == nil {
		return nil, getQuoteOutput{}, service.BadRequest("live quotes are not configured")
	}
	sym := strings.ToUpper(strings.TrimSpace(in.Symbol))
	if sym == "" {
		return nil, getQuoteOutput{}, service.BadRequest("symbol is required")
	}
	q, ok, err := t.snapshots.FetchSnapshot(ctx, sym)
	if err != nil {
		return nil, getQuoteOutput{}, err
	}
	if !ok {
		return nil, getQuoteOutput{}, service.BadRequest("no snapshot available for " + sym)
	}
	return nil, getQuoteOutput{
		Quote:     q,
		Citations: []citationOut{{Kind: "ticker", ID: sym, Title: sym}},
	}, nil
}

func (t workspaceToolServer) addToWatchlist(ctx context.Context, _ *sdkmcp.CallToolRequest, in addToWatchlistInput) (*sdkmcp.CallToolResult, addToWatchlistOutput, error) {
	principal, err := service.RequirePrincipal(ctx)
	if err != nil {
		return nil, addToWatchlistOutput{}, err
	}
	sym := strings.ToUpper(strings.TrimSpace(in.Symbol))
	if sym == "" {
		return nil, addToWatchlistOutput{}, service.BadRequest("symbol is required")
	}
	id, ok, err := t.instruments.ResolveInstrumentID(ctx, sym)
	if err != nil {
		return nil, addToWatchlistOutput{}, err
	}
	if !ok {
		return nil, addToWatchlistOutput{OK: false, Symbol: sym, Note: "unknown symbol " + sym}, nil
	}
	if err := t.watchlist.Add(ctx, principal.UserID, id); err != nil {
		return nil, addToWatchlistOutput{}, err
	}
	return nil, addToWatchlistOutput{
		OK:        true,
		Symbol:    sym,
		Citations: []citationOut{{Kind: "ticker", ID: sym, Title: sym}},
	}, nil
}

func (t workspaceToolServer) drawEdge(ctx context.Context, _ *sdkmcp.CallToolRequest, in drawEdgeInput) (*sdkmcp.CallToolResult, drawEdgeOutput, error) {
	principal, err := service.RequirePrincipal(ctx)
	if err != nil {
		return nil, drawEdgeOutput{}, err
	}
	if strings.TrimSpace(in.FromID) == "" || strings.TrimSpace(in.ToID) == "" || strings.TrimSpace(in.Rel) == "" {
		return nil, drawEdgeOutput{}, service.BadRequest("from_id, to_id, and rel are required")
	}
	if err := t.entities.DrawEdge(ctx, service.DrawEdgeInput{
		OwnerUserID: principal.UserID,
		FromID:      in.FromID,
		ToID:        in.ToID,
		Rel:         in.Rel,
		Why:         in.Why,
	}); err != nil {
		return nil, drawEdgeOutput{}, err
	}
	return nil, drawEdgeOutput{OK: true}, nil
}

func (t workspaceToolServer) getNews(ctx context.Context, _ *sdkmcp.CallToolRequest, in getNewsInput) (*sdkmcp.CallToolResult, getNewsOutput, error) {
	if t.marketInfo == nil {
		return nil, getNewsOutput{}, service.BadRequest("market data is not configured")
	}
	items, err := t.marketInfo.News(ctx, strings.ToUpper(strings.TrimSpace(in.Symbols)), in.Limit)
	if err != nil {
		return nil, getNewsOutput{}, err
	}
	// Cite the distinct symbols the news touches, so the answer can link tickers.
	seen := map[string]bool{}
	var cites []citationOut
	for _, n := range items {
		for _, sym := range n.Symbols {
			if sym != "" && !seen[sym] {
				seen[sym] = true
				cites = append(cites, citationOut{Kind: "ticker", ID: sym, Title: sym})
			}
		}
	}
	return nil, getNewsOutput{News: items, Citations: cites}, nil
}

func (t workspaceToolServer) getMovers(ctx context.Context, _ *sdkmcp.CallToolRequest, in topInput) (*sdkmcp.CallToolResult, getMoversOutput, error) {
	if t.marketInfo == nil {
		return nil, getMoversOutput{}, service.BadRequest("market data is not configured")
	}
	m, err := t.marketInfo.Movers(ctx, in.Top)
	if err != nil {
		return nil, getMoversOutput{}, err
	}
	return nil, getMoversOutput{Movers: m}, nil
}

func (t workspaceToolServer) getMostActives(ctx context.Context, _ *sdkmcp.CallToolRequest, in topInput) (*sdkmcp.CallToolResult, getMostActivesOutput, error) {
	if t.marketInfo == nil {
		return nil, getMostActivesOutput{}, service.BadRequest("market data is not configured")
	}
	m, err := t.marketInfo.MostActives(ctx, in.Top)
	if err != nil {
		return nil, getMostActivesOutput{}, err
	}
	return nil, getMostActivesOutput{MostActives: m}, nil
}

func (t workspaceToolServer) getAccount(ctx context.Context, _ *sdkmcp.CallToolRequest, _ emptyInput) (*sdkmcp.CallToolResult, getAccountOutput, error) {
	if t.marketInfo == nil {
		return nil, getAccountOutput{}, service.BadRequest("trading account is not configured")
	}
	acc, err := t.marketInfo.Account(ctx)
	if err != nil {
		return nil, getAccountOutput{}, err
	}
	return nil, getAccountOutput{Account: acc}, nil
}

func (t workspaceToolServer) getPositions(ctx context.Context, _ *sdkmcp.CallToolRequest, _ emptyInput) (*sdkmcp.CallToolResult, getPositionsOutput, error) {
	if t.marketInfo == nil {
		return nil, getPositionsOutput{}, service.BadRequest("trading account is not configured")
	}
	ps, err := t.marketInfo.Positions(ctx)
	if err != nil {
		return nil, getPositionsOutput{}, err
	}
	cites := make([]citationOut, 0, len(ps))
	for _, p := range ps {
		cites = append(cites, citationOut{Kind: "ticker", ID: p.Symbol, Title: p.Symbol})
	}
	return nil, getPositionsOutput{Positions: ps, Citations: cites}, nil
}

// citationKindForArtifact maps an artifact type to the citation kind the client
// routes on. "app" is a shard; every other artifact kind opens in the work
// pane, which the client's nav does for "page".
func citationKindForArtifact(t string) string {
	if t == "app" {
		return "shard"
	}
	return "page"
}
