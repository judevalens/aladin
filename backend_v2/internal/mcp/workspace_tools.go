package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"aladin/backend_v2/internal/blocknote"
	"aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/watchlist"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"sort"
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
	// Page anchors the citation inside the artifact (a document page). The client's
	// wormhole opens the source there; zero means "no anchor".
	Page int `json:"page,omitempty"`
}

// workspaceToolServer carries the deps the workspace tools need. snapshots and
// marketInfo may be nil (no market-data keys) — the dependent tools degrade with
// a clear "not configured" error.
type workspaceToolServer struct {
	search      service.SearchService
	entities    service.EntityContextService
	insights    service.InsightService
	artifacts   service.ArtifactService
	watchlist   watchlist.Service
	bars        service.BarService
	snapshots   service.QuoteSnapshotSource
	marketInfo  service.MarketInfoService
	alerts      service.AlertService
	instruments service.InstrumentService
	documents   service.DocumentService
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
		Description: "Read one artifact by id — works for ANY kind: a page (returns its text), a shard/app (returns its content + metadata), a link, a file, or a voice note. For an ingested PDF this returns its OUTLINE and page count but NOT its text — use search_document to find passages, then read_document to read around them. For block-level page edits use get_page instead.",
	}, t.getArtifact)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "search_document",
		Description: "Find passages inside ONE ingested document by keyword. Returns short snippets with page " +
			"numbers, not the document. This is the right first move for any question about a PDF — " +
			"get_artifact deliberately does not return the text, and reading page by page through a long " +
			"document wastes context. Follow a hit with read_document to read around it.",
	}, t.searchDocument)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "read_document",
		Description: "Read a page range from an ingested document (a PDF Aladin has extracted). " +
			"Use it to read AROUND a hit from search_document, or a range from the outline. Capped at 25 pages " +
			"per call — it is not a way to read a whole book. Text comes back with [pN] markers so you can cite pages.",
	}, t.readDocument)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_watchlist",
		Description: "List the tickers in a watchlist. Optionally pass a list name; omit for the user's default list.",
	}, t.getWatchlist)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_watchlists",
		Description: "List the user's watchlists (named instrument sets) with their item counts.",
	}, t.listWatchlists)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "create_watchlist",
		Description: "Create a new named watchlist. Returns its id.",
	}, t.createWatchlist)
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
		Description: "Add a ticker to a watchlist by symbol. Optionally pass a list name (created if it doesn't exist); omit for the user's default list.",
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
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "create_alert",
		Annotations: destructiveTool("Create alert"),
		Description: "Create a recurring price alert on a symbol. direction is \"above\" or \"below\", threshold is the price. It fires when the price crosses the level with confirming momentum, then self-re-arms after a genuine pullback (so it won't spam on jitter). The result surfaces as a notification. This asks the user to approve before it's created.",
	}, t.createAlert)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_alerts",
		Description: "List the user's price alerts (symbol, direction, threshold, armed/status, last fired).",
	}, t.listAlerts)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "delete_alert",
		Description: "Delete a price alert by id.",
	}, t.deleteAlert)
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

	// Set for an ingested document (a PDF we've read). The outline comes back in full
	// because it's small and it's how an agent decides WHAT to read; the text is
	// truncated, with `more` telling the model to call read_document for the rest.
	PageCount   int              `json:"page_count,omitempty"`
	Outline     []documentTOCOut `json:"outline,omitempty"`
	OutlineNote string           `json:"outline_note,omitempty"`
	More        string           `json:"more,omitempty"`
	// Non-empty when the file exists but couldn't be read (e.g. a scan needing OCR),
	// so the model says so instead of assuming the document is empty.
	Unreadable string `json:"unreadable,omitempty"`
}

type documentTOCOut struct {
	Title string `json:"title"`
	Level int    `json:"level"`
	Page  int    `json:"page"`
}

type searchDocumentInput struct {
	ArtifactID string `json:"artifact_id"`
	Query      string `json:"query"`
	Limit      int    `json:"limit,omitempty"`
}

type documentHitOut struct {
	Page    int    `json:"page"`
	Snippet string `json:"snippet"`
}

type searchDocumentOutput struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Query     string           `json:"query"`
	Hits      []documentHitOut `json:"hits"`
	Note      string           `json:"note,omitempty"`
	Citations []citationOut    `json:"citations,omitempty"`
}

type readDocumentInput struct {
	ArtifactID string `json:"artifact_id"`
	FromPage   int    `json:"from_page,omitempty"`
	ToPage     int    `json:"to_page,omitempty"`
}

type readDocumentOutput struct {
	ID        string        `json:"id"`
	Title     string        `json:"title"`
	FromPage  int           `json:"from_page"`
	ToPage    int           `json:"to_page"`
	PageCount int           `json:"page_count"`
	Text      string        `json:"text"`
	More      string        `json:"more,omitempty"`
	Citations []citationOut `json:"citations,omitempty"`
}

type getWatchlistInput struct {
	List string `json:"list,omitempty"` // optional list name; empty = default
}
type getWatchlistOutput struct {
	Items []watchlist.WatchlistItem `json:"items"`
}

type listWatchlistsOutput struct {
	Watchlists []watchlist.Watchlist `json:"watchlists"`
}
type createWatchlistInput struct {
	Name string `json:"name"`
}
type createWatchlistOutput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
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
	List   string `json:"list,omitempty"` // optional list name (created if new); empty = default
}
type addToWatchlistOutput struct {
	OK        bool          `json:"ok"`
	Symbol    string        `json:"symbol"`
	List      string        `json:"list,omitempty"`
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
	// A board's Content is a tldraw snapshot (projected by the sync room) — summarize its
	// structure instead of dumping shape JSON at the model.
	if art.Type == "board" {
		body = summarizeBoardContent(art.Content)
	}
	out := getArtifactOutput{
		ID: art.ID, Title: art.Title, Type: art.Type,
		Text: body, Summary: art.Summary, Metadata: art.Metadata,
		Citations: []citationOut{{Kind: citationKindForArtifact(art.Type), ID: art.ID, Title: art.Title}},
	}

	// An ingested file (a PDF we've read) reports that it IS readable and how to read it
	// — it does not hand back the text.
	//
	// Returning a slice of the document here was a mistake: get_artifact is called to
	// find out WHAT something is, often across several artifacts at once, and every
	// caller was paying for a wall of text it never asked for. Worse, the slice was the
	// first pages — the title page and front matter, i.e. the least useful part of any
	// document. Reading is now always a deliberate act: search_document to find the
	// relevant passage, read_document to expand around it.
	if art.Type == "file" && t.documents != nil {
		if doc, derr := t.documents.Get(ctx, art.ID, false); derr == nil {
			out.PageCount = doc.PageCount
			out.Outline, out.OutlineNote = capOutline(doc.Sections)
			// Most PDFs ship no bookmarks. Segmentation recovers a structure anyway
			// (INGESTION_PRD §11), so fall back to it rather than reporting no outline
			// for a document that plainly has sections.
			if len(out.Outline) == 0 {
				if tree, terr := t.documents.Outline(ctx, art.ID); terr == nil {
					out.Outline, out.OutlineNote = capOutline(flattenChunkTitles(tree, 0))
					if len(out.Outline) > 0 && out.OutlineNote == "" {
						out.OutlineNote = "Outline recovered by layout analysis — this PDF carries no bookmarks of its own."
					}
				}
			}
			switch doc.Status {
			case "ready":
				out.Text = ""
				out.More = fmt.Sprintf(
					"This is a readable %d-page document. Its text is NOT included here. "+
						"Use search_document(artifact_id=%q, query=...) to find relevant passages, "+
						"then read_document(artifact_id=%q, from_page, to_page) to read around a hit.",
					doc.PageCount, art.ID, art.ID)
			case "pending", "ingesting":
				out.Unreadable = "This document is still being read; try again shortly."
			default:
				// A scan needing OCR, or a broken file. Say so — an agent that assumes
				// "no text" means "nothing to say" will confidently answer about nothing.
				out.Unreadable = doc.Error
				if out.Unreadable == "" {
					out.Unreadable = "This file could not be read (status: " + doc.Status + ")."
				}
			}
		}
	}
	return nil, out, nil
}

// flattenChunkTitles walks the recovered tree into the same levelled sequence an
// embedded outline would produce, so both sources render identically to the model.
func flattenChunkTitles(chunks []service.DocumentChunk, depth int) []service.DocumentSection {
	out := []service.DocumentSection{}
	for _, chunk := range chunks {
		if chunk.Kind == "section" && strings.TrimSpace(chunk.Title) != "" {
			out = append(out, service.DocumentSection{Title: chunk.Title, Level: depth, Page: chunk.PageFrom})
		}
		// Blocks are leaves with no heading; descend past them for nested sections.
		next := depth
		if chunk.Kind == "section" {
			next = depth + 1
		}
		out = append(out, flattenChunkTitles(chunk.Children, next)...)
	}
	return out
}

// maxOutlineEntries bounds the outline too. A 400-page book can carry hundreds of
// bookmarks, and an outline that fills the context is the same mistake as text that
// does — just quieter.
const maxOutlineEntries = 60

// capOutline keeps the top of the tree, which is the part you navigate by, and says so
// when it drops the rest rather than silently presenting a partial contents page as
// complete.
func capOutline(sections []service.DocumentSection) ([]documentTOCOut, string) {
	if len(sections) <= maxOutlineEntries {
		out := make([]documentTOCOut, 0, len(sections))
		for _, section := range sections {
			out = append(out, documentTOCOut(section))
		}
		return out, ""
	}
	// Prefer shallower entries — chapters over sub-sub-sections.
	for depth := 0; depth < 4; depth++ {
		kept := make([]documentTOCOut, 0, maxOutlineEntries)
		for _, section := range sections {
			if section.Level <= depth {
				kept = append(kept, documentTOCOut(section))
			}
		}
		if len(kept) > 0 && len(kept) <= maxOutlineEntries {
			return kept, fmt.Sprintf(
				"Outline truncated to %d top-level entries (of %d total) to keep it readable.",
				len(kept), len(sections))
		}
	}
	trimmed := make([]documentTOCOut, 0, maxOutlineEntries)
	for _, section := range sections[:maxOutlineEntries] {
		trimmed = append(trimmed, documentTOCOut(section))
	}
	return trimmed, fmt.Sprintf("Outline truncated to the first %d of %d entries.", maxOutlineEntries, len(sections))
}

// readDocumentBudget bounds a targeted read. Generous, because the caller asked for this
// text on purpose and named a range — but still bounded, since an unbounded read is how
// one document eats a context window.
const readDocumentBudget = 20000

// maxReadPages stops "read the whole book" being expressible as one call. A reader that
// wants more asks again with the next range, which at least keeps the cost visible.
const maxReadPages = 25

// joinPages renders pages as text with [pN] markers, up to a character budget. The
// markers are what let a model cite "p. 42" instead of gesturing at the document.
func joinPages(pages []service.DocumentPage, budget int) (string, bool) {
	var builder strings.Builder
	truncated := false
	for _, page := range pages {
		if strings.TrimSpace(page.Text) == "" {
			continue
		}
		chunk := fmt.Sprintf("[p%d]\n%s\n\n", page.Page, page.Text)
		if budget > 0 && builder.Len()+len(chunk) > budget {
			truncated = true
			break
		}
		builder.WriteString(chunk)
	}
	return strings.TrimSpace(builder.String()), truncated
}

func (t workspaceToolServer) readDocument(ctx context.Context, _ *sdkmcp.CallToolRequest, in readDocumentInput) (*sdkmcp.CallToolResult, readDocumentOutput, error) {
	if strings.TrimSpace(in.ArtifactID) == "" {
		return nil, readDocumentOutput{}, service.BadRequest("artifact_id is required")
	}
	if t.documents == nil {
		return nil, readDocumentOutput{}, service.BadRequest("document reading is not configured")
	}
	art, err := t.artifacts.Get(ctx, in.ArtifactID)
	if err != nil {
		return nil, readDocumentOutput{}, err
	}
	doc, err := t.documents.Get(ctx, in.ArtifactID, false)
	if err != nil {
		return nil, readDocumentOutput{}, err
	}
	if doc.Status != "ready" {
		return nil, readDocumentOutput{}, service.BadRequest("this document has no readable text: " + doc.Error)
	}

	from, to := in.FromPage, in.ToPage
	if from <= 0 {
		from = 1
	}
	if to <= 0 || to > doc.PageCount {
		to = doc.PageCount
	}
	// "Read the whole book" must not be expressible as one call. Capping the span keeps
	// the cost of going deeper visible instead of accidental.
	capped := false
	if to-from+1 > maxReadPages {
		to = from + maxReadPages - 1
		capped = true
	}

	// Only this range leaves the database — the rest of the document is never loaded.
	pages, err := t.documents.Pages(ctx, in.ArtifactID, from, to)
	if err != nil {
		return nil, readDocumentOutput{}, err
	}
	text, truncated := joinPages(pages, readDocumentBudget)

	out := readDocumentOutput{
		ID: art.ID, Title: art.Title, FromPage: from, ToPage: to,
		PageCount: doc.PageCount, Text: text,
		Citations: []citationOut{{Kind: citationKindForArtifact(art.Type), ID: art.ID, Title: art.Title, Page: from}},
	}
	switch {
	case truncated:
		out.More = "Truncated at the size limit — request a narrower page range to read the rest."
	case capped:
		out.More = fmt.Sprintf("Capped at %d pages; continue from page %d.", maxReadPages, to+1)
	}
	return nil, out, nil
}

func (t workspaceToolServer) searchDocument(ctx context.Context, _ *sdkmcp.CallToolRequest, in searchDocumentInput) (*sdkmcp.CallToolResult, searchDocumentOutput, error) {
	if strings.TrimSpace(in.ArtifactID) == "" {
		return nil, searchDocumentOutput{}, service.BadRequest("artifact_id is required")
	}
	if t.documents == nil {
		return nil, searchDocumentOutput{}, service.BadRequest("document search is not configured")
	}
	art, err := t.artifacts.Get(ctx, in.ArtifactID)
	if err != nil {
		return nil, searchDocumentOutput{}, err
	}
	hits, err := t.documents.Search(ctx, in.ArtifactID, in.Query, in.Limit)
	if err != nil {
		return nil, searchDocumentOutput{}, err
	}
	out := searchDocumentOutput{
		ID: art.ID, Title: art.Title, Query: in.Query,
		Citations: []citationOut{{Kind: citationKindForArtifact(art.Type), ID: art.ID, Title: art.Title}},
	}
	for _, hit := range hits {
		out.Hits = append(out.Hits, documentHitOut{Page: hit.Page, Snippet: hit.Snippet})
	}
	if len(hits) > 0 {
		// The chip lands on the best hit's page — the reader opens where the answer is.
		out.Citations[0].Page = hits[0].Page
	}
	if len(out.Hits) == 0 {
		out.Note = "No matches. Try different wording — this is keyword search over the document's text, not semantic."
	} else {
		out.Note = "Snippets only. Use read_document with a page from a hit to read around it."
	}
	return nil, out, nil
}

func (t workspaceToolServer) getWatchlist(ctx context.Context, _ *sdkmcp.CallToolRequest, in getWatchlistInput) (*sdkmcp.CallToolResult, getWatchlistOutput, error) {
	principal, err := service.RequirePrincipal(ctx)
	if err != nil {
		return nil, getWatchlistOutput{}, err
	}
	listID := "" // default list
	if strings.TrimSpace(in.List) != "" {
		if listID, err = t.watchlist.ResolveOrCreateByName(ctx, principal.UserID, in.List); err != nil {
			return nil, getWatchlistOutput{}, err
		}
	}
	items, err := t.watchlist.ListItems(ctx, principal.UserID, listID)
	if err != nil {
		return nil, getWatchlistOutput{}, err
	}
	return nil, getWatchlistOutput{Items: items}, nil
}

func (t workspaceToolServer) listWatchlists(ctx context.Context, _ *sdkmcp.CallToolRequest, _ emptyInput) (*sdkmcp.CallToolResult, listWatchlistsOutput, error) {
	principal, err := service.RequirePrincipal(ctx)
	if err != nil {
		return nil, listWatchlistsOutput{}, err
	}
	lists, err := t.watchlist.ListWatchlists(ctx, principal.UserID)
	if err != nil {
		return nil, listWatchlistsOutput{}, err
	}
	return nil, listWatchlistsOutput{Watchlists: lists}, nil
}

func (t workspaceToolServer) createWatchlist(ctx context.Context, _ *sdkmcp.CallToolRequest, in createWatchlistInput) (*sdkmcp.CallToolResult, createWatchlistOutput, error) {
	principal, err := service.RequirePrincipal(ctx)
	if err != nil {
		return nil, createWatchlistOutput{}, err
	}
	w, err := t.watchlist.CreateWatchlist(ctx, principal.UserID, in.Name)
	if err != nil {
		return nil, createWatchlistOutput{}, err
	}
	return nil, createWatchlistOutput{ID: w.ID, Name: w.Name}, nil
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
	// Resolve the target list by name (creating it if new); empty → the user's default list.
	listID, err := t.watchlist.ResolveOrCreateByName(ctx, principal.UserID, in.List)
	if err != nil {
		return nil, addToWatchlistOutput{}, err
	}
	if err := t.watchlist.AddItem(ctx, principal.UserID, listID, id); err != nil {
		return nil, addToWatchlistOutput{}, err
	}
	return nil, addToWatchlistOutput{
		OK:        true,
		Symbol:    sym,
		List:      strings.TrimSpace(in.List),
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

type createAlertInput struct {
	Symbol    string  `json:"symbol"`
	Direction string  `json:"direction"` // above | below
	Threshold float64 `json:"threshold"`
}
type createAlertOutput struct {
	Alert     service.Alert `json:"alert"`
	Warning   string        `json:"warning,omitempty"`
	Citations []citationOut `json:"citations,omitempty"`
}
type listAlertsOutput struct {
	Alerts []service.Alert `json:"alerts"`
}
type deleteAlertInput struct {
	ID string `json:"id"`
}
type deleteAlertOutput struct {
	OK bool `json:"ok"`
}

func (t workspaceToolServer) createAlert(ctx context.Context, _ *sdkmcp.CallToolRequest, in createAlertInput) (*sdkmcp.CallToolResult, createAlertOutput, error) {
	if t.alerts == nil {
		return nil, createAlertOutput{}, service.BadRequest("alerts are not configured")
	}
	principal, err := service.RequirePrincipal(ctx)
	if err != nil {
		return nil, createAlertOutput{}, err
	}
	res, err := t.alerts.Create(ctx, principal.UserID, in.Symbol, in.Direction, in.Threshold)
	if err != nil {
		return nil, createAlertOutput{}, err
	}
	return nil, createAlertOutput{
		Alert:     res.Alert,
		Warning:   res.Warning,
		Citations: []citationOut{{Kind: "ticker", ID: res.Alert.Symbol, Title: res.Alert.Symbol}},
	}, nil
}

func (t workspaceToolServer) listAlerts(ctx context.Context, _ *sdkmcp.CallToolRequest, _ emptyInput) (*sdkmcp.CallToolResult, listAlertsOutput, error) {
	if t.alerts == nil {
		return nil, listAlertsOutput{}, service.BadRequest("alerts are not configured")
	}
	principal, err := service.RequirePrincipal(ctx)
	if err != nil {
		return nil, listAlertsOutput{}, err
	}
	items, err := t.alerts.List(ctx, principal.UserID)
	if err != nil {
		return nil, listAlertsOutput{}, err
	}
	return nil, listAlertsOutput{Alerts: items}, nil
}

func (t workspaceToolServer) deleteAlert(ctx context.Context, _ *sdkmcp.CallToolRequest, in deleteAlertInput) (*sdkmcp.CallToolResult, deleteAlertOutput, error) {
	if t.alerts == nil {
		return nil, deleteAlertOutput{}, service.BadRequest("alerts are not configured")
	}
	principal, err := service.RequirePrincipal(ctx)
	if err != nil {
		return nil, deleteAlertOutput{}, err
	}
	if err := t.alerts.Delete(ctx, principal.UserID, in.ID); err != nil {
		return nil, deleteAlertOutput{}, err
	}
	return nil, deleteAlertOutput{OK: true}, nil
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

// summarizeBoardContent renders a board's tldraw snapshot as a line of structure — shape
// counts by kind and the texts a model could act on (tasks, cards, excerpts, links, ink
// labels) — instead of the raw record JSON, which is noise at best and context-flooding
// at worst. Parsing lives in service.ParseBoardContent, the SAME parser the content-index
// projector uses, so what the copilot reads and what search retrieves can never drift.
func summarizeBoardContent(content string) string {
	parsed := service.ParseBoardContent(content)
	if len(parsed.Counts) == 0 {
		return "an empty board"
	}
	kinds := make([]string, 0, len(parsed.Counts))
	for kind := range parsed.Counts {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	parts := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		parts = append(parts, fmt.Sprintf("%d %s", parsed.Counts[kind], kind))
	}
	lines := make([]string, 0, len(parsed.Lines))
	for _, line := range parsed.Lines {
		lines = append(lines, line.Text)
	}
	sort.Strings(lines)
	summary := "board with " + strings.Join(parts, ", ")
	if len(lines) > 0 {
		summary += "\n" + strings.Join(lines, "\n")
	}
	return summary
}
