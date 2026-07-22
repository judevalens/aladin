package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"aladin/backend_v2/internal/blocknote"
	"aladin/backend_v2/internal/llm"
)

// toolDefs is the read-only tool surface exposed to the agent. Each maps to a service
// already wired in DI; the schemas are what the model sees when deciding what to call.
func (s *defaultCopilotService) toolDefs() []llm.ChatToolDef {
	defs := []llm.ChatToolDef{
		{
			Name:        "search",
			Description: "Federated search over the user's Aladin workspace: tickers, entities (companies/people), pages, and shards. Use this first to find the ids of things to look up.",
			Parameters: objSchema(map[string]any{
				"query": strProp("The search query."),
				"limit": intProp("Max results (default 8)."),
			}, "query"),
		},
		{
			Name:        "get_entity",
			Description: "Fetch one entity's identity, its typed relationships (edges), and the verbatim material accreted under it. entityId comes from a search hit of kind entity/company/person.",
			Parameters: objSchema(map[string]any{
				"entityId": strProp("The entity id."),
			}, "entityId"),
		},
		{
			Name:        "get_insights",
			Description: "List engine-generated insights (discourse/bridges over the user's connected sources). Optionally filter by type or status.",
			Parameters: objSchema(map[string]any{
				"limit":  intProp("Max insights (default 10)."),
				"type":   strProp("Optional insight type filter."),
				"status": strProp("Optional status filter (e.g. pending)."),
			}),
		},
		{
			Name:        "list_artifacts",
			Description: "List the user's artifacts as {id,title,type}. type is one of page, app (a shard — an agent-built interactive doc), link, file, voice. Use get_artifact to read one.",
			Parameters: objSchema(map[string]any{
				"type": strProp("Optional type filter: page | app | link | file | voice."),
			}),
		},
		{
			Name:        "get_artifact",
			Description: "Read one artifact by id — works for ANY kind: a page (returns its blocks), a shard/app (returns its content + metadata), a link, a file, or a voice note. Use this whenever an artifact is the current surface.",
			Parameters: objSchema(map[string]any{
				"artifactId": strProp("The artifact id."),
			}, "artifactId"),
		},
		{
			Name:        "get_page",
			Description: "Read one page's content (BlockNote blocks) by id. For non-page artifacts (shards, links, files) use get_artifact instead.",
			Parameters: objSchema(map[string]any{
				"pageId": strProp("The page (artifact) id."),
			}, "pageId"),
		},
		{
			Name:        "get_watchlist",
			Description: "List the tickers the user is currently tracking on the Markets surface.",
			Parameters:  objSchema(map[string]any{}),
		},
		{
			Name:        "get_browser_tree",
			Description: "The user's full workspace tree: folders and the artifacts (pages, shards, links, files) inside them.",
			Parameters:  objSchema(map[string]any{}),
		},
		{
			Name:        "list_folders",
			Description: "List folders. Omit parentId for root folders, or pass a folder id for its child folders.",
			Parameters: objSchema(map[string]any{
				"parentId": strProp("Optional parent folder id."),
			}),
		},
		{
			Name:        "search_pages",
			Description: "Search the user's pages by title, summary, and body text. Returns matching pages.",
			Parameters: objSchema(map[string]any{
				"query": strProp("The search query."),
				"limit": intProp("Max results (default 10)."),
			}, "query"),
		},
		{
			Name:        "get_bars",
			Description: "OHLCV price history for a ticker symbol. timeframe is e.g. 1Day or 5Min; returns oldest→newest.",
			Parameters: objSchema(map[string]any{
				"symbol":    strProp("Ticker symbol, e.g. NVDA."),
				"timeframe": strProp("Bar timeframe (default 1Day)."),
				"limit":     intProp("Number of bars (default 30)."),
			}, "symbol"),
		},
	}
	if s.Snapshots != nil {
		defs = append(defs, llm.ChatToolDef{
			Name:        "get_quote",
			Description: "Current last price + previous close for a ticker symbol (snapshot).",
			Parameters: objSchema(map[string]any{
				"symbol": strProp("Ticker symbol, e.g. NVDA."),
			}, "symbol"),
		})
	}
	// Shard authoring — only when the doc-surface services are wired.
	if s.DocStore != nil && s.ShardBuild != nil {
		defs = append(defs, shardToolDefs(s.Preview != nil)...)
	}
	return defs
}

// runTool dispatches a single tool call, returning the JSON result string the model reads
// plus any citations it should surface. userID scopes the user-scoped services.
func (s *defaultCopilotService) runTool(ctx context.Context, userID, name, args string) (string, []Citation, error) {
	switch name {
	case "search":
		var a struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		if a.Limit <= 0 {
			a.Limit = 8
		}
		resp, err := s.Search.Search(ctx, userID, a.Query, a.Limit)
		if err != nil {
			return "", nil, err
		}
		var cites []Citation
		for _, sec := range resp.Sections {
			for _, h := range sec.Hits {
				cites = append(cites, Citation{Kind: h.Kind, ID: h.ID, Title: h.Title})
			}
		}
		return jsonString(resp), cites, nil

	case "get_entity":
		var a struct {
			EntityID string `json:"entityId"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		ec, err := s.Entities.Get(ctx, userID, a.EntityID)
		if err != nil {
			return "", nil, err
		}
		return jsonString(ec), []Citation{{Kind: "entity", ID: ec.Entity.ID, Title: ec.Entity.Name}}, nil

	case "get_insights":
		var a struct {
			Limit  int    `json:"limit"`
			Type   string `json:"type"`
			Status string `json:"status"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		if a.Limit <= 0 {
			a.Limit = 10
		}
		res, err := s.Insights.List(ctx, InsightListParams{Limit: a.Limit, Type: a.Type, Status: a.Status})
		if err != nil {
			return "", nil, err
		}
		return jsonString(res), nil, nil

	case "list_artifacts":
		var a struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		items, err := s.Artifacts.List(ctx, ArtifactListParams{})
		if err != nil {
			return "", nil, err
		}
		filter := strings.TrimSpace(a.Type)
		out := make([]map[string]string, 0)
		for _, it := range items {
			if filter != "" && it.Type != filter {
				continue
			}
			out = append(out, map[string]string{"id": it.ID, "title": it.Title, "type": it.Type})
		}
		return jsonString(map[string]any{"artifacts": out}), nil, nil

	case "get_artifact":
		var a struct {
			ArtifactID string `json:"artifactId"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		art, err := s.Artifacts.Get(ctx, a.ArtifactID)
		if err != nil {
			return "", nil, err
		}
		// Hand the model readable text, not raw BlockNote JSON: a page's body is its
		// blocks (extract the inline text); other kinds carry plain text in Content.
		body := art.Content
		if len(art.Blocks) > 0 {
			if text, terr := blocknote.ExtractText(art.Blocks); terr == nil && strings.TrimSpace(text) != "" {
				body = text
			}
		}
		out := jsonString(map[string]any{
			"id": art.ID, "title": art.Title, "type": art.Type,
			"text": body, "summary": art.Summary, "metadata": art.Metadata,
		})
		return out, []Citation{{Kind: citationKindForArtifact(art.Type), ID: art.ID, Title: art.Title}}, nil

	case "get_page":
		var a struct {
			PageID string `json:"pageId"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		doc, err := s.Pages.Get(ctx, a.PageID)
		if err != nil {
			return "", nil, err
		}
		// Extract readable text from the page's blocks — raw BlockNote JSON is unreadable
		// to the model and gets truncated before any real content survives.
		text, _ := blocknote.ExtractText(doc.Blocks)
		out := jsonString(map[string]any{"id": doc.ID, "title": doc.Title, "text": text})
		return out, []Citation{{Kind: "page", ID: doc.ID, Title: doc.Title}}, nil

	case "get_watchlist":
		items, err := s.Watchlist.List(ctx, userID)
		if err != nil {
			return "", nil, err
		}
		return jsonString(map[string]any{"items": items}), nil, nil

	case "get_browser_tree":
		tree, err := s.Artifacts.BrowserTree(ctx)
		if err != nil {
			return "", nil, err
		}
		return jsonString(map[string]any{"tree": tree}), nil, nil

	case "list_folders":
		var a struct {
			ParentID string `json:"parentId"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		var parent *string
		if p := strings.TrimSpace(a.ParentID); p != "" {
			parent = &p
		}
		folders, err := s.Artifacts.ListFolders(ctx, parent)
		if err != nil {
			return "", nil, err
		}
		return jsonString(map[string]any{"folders": folders}), nil, nil

	case "search_pages":
		var a struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		if a.Limit <= 0 {
			a.Limit = 10
		}
		pages, err := s.Artifacts.SearchPages(ctx, PageSearchParams{Query: a.Query, Limit: a.Limit})
		if err != nil {
			return "", nil, err
		}
		cites := make([]Citation, 0, len(pages))
		for _, p := range pages {
			cites = append(cites, Citation{Kind: citationKindForArtifact(p.Type), ID: p.ID, Title: p.Title})
		}
		return jsonString(map[string]any{"pages": pages}), cites, nil

	case "get_bars":
		var a struct {
			Symbol    string `json:"symbol"`
			Timeframe string `json:"timeframe"`
			Limit     int    `json:"limit"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		if a.Limit <= 0 {
			a.Limit = 30
		}
		sym := strings.ToUpper(strings.TrimSpace(a.Symbol))
		bars, err := s.Bars.Get(ctx, sym, a.Timeframe, a.Limit)
		if err != nil {
			return "", nil, err
		}
		return jsonString(map[string]any{"symbol": sym, "bars": bars}), []Citation{{Kind: "ticker", ID: sym, Title: sym}}, nil

	case "get_quote":
		if s.Snapshots == nil {
			return `{"error":"live quotes not configured"}`, nil, nil
		}
		var a struct {
			Symbol string `json:"symbol"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		sym := strings.ToUpper(strings.TrimSpace(a.Symbol))
		q, ok, err := s.Snapshots.FetchSnapshot(ctx, sym)
		if err != nil {
			return "", nil, err
		}
		if !ok {
			return `{"error":"no snapshot available"}`, nil, nil
		}
		return jsonString(q), []Citation{{Kind: "ticker", ID: sym, Title: sym}}, nil

	default:
		if res, cites, ok, err := s.runShardTool(ctx, name, args); ok {
			return res, cites, err
		}
		return "", nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// --- tiny JSON-schema builders ------------------------------------------------

func objSchema(props map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func strProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func intProp(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}

const maxSurfaceContextChars = 1800

// surfaceContext builds a compact "current context" block from what the user is looking at, so
// the agent can answer about the current surface WITHOUT a tool round-trip. Best-effort: any
// missing field or fetch error yields an empty (skipped) block.
func (s *defaultCopilotService) surfaceContext(ctx context.Context, userID string, surface CopilotSurface) string {
	switch strings.TrimSpace(surface.Kind) {
	case "ticker":
		sym := strings.ToUpper(strings.TrimSpace(surface.Symbol))
		if sym == "" || s.Snapshots == nil {
			return ""
		}
		q, ok, err := s.Snapshots.FetchSnapshot(ctx, sym)
		if err != nil || !ok {
			return ""
		}
		return fmt.Sprintf("Current context — the user is viewing ticker %s. Latest snapshot: last %.2f, previous close %.2f, change %.2f%%.",
			sym, q.Last, q.PrevClose, q.ChangePct)

	case "artifact", "page", "shard":
		if strings.TrimSpace(surface.ID) == "" {
			return ""
		}
		art, err := s.Artifacts.Get(ctx, surface.ID)
		if err != nil {
			return ""
		}
		body := strings.TrimSpace(art.Content)
		if len(art.Blocks) > 0 {
			if text, terr := blocknote.ExtractText(art.Blocks); terr == nil && strings.TrimSpace(text) != "" {
				body = strings.TrimSpace(text)
			}
		}
		header := fmt.Sprintf("Current context — the user is viewing the %s %q.", artifactNoun(art.Type), art.Title)
		if body == "" {
			return header
		}
		return capText(header+" Its content:\n"+body, maxSurfaceContextChars)

	case "entity":
		if strings.TrimSpace(surface.ID) == "" {
			return ""
		}
		ec, err := s.Entities.Get(ctx, userID, surface.ID)
		if err != nil {
			return ""
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Current context — the user is viewing the entity %q (%s).", ec.Entity.Name, ec.Entity.Kind)
		if g := strings.TrimSpace(ec.Entity.Gist); g != "" {
			fmt.Fprintf(&b, " %s", g)
		}
		rels := make([]string, 0, 5)
		for _, e := range ec.Edges {
			if len(rels) >= 5 {
				break
			}
			if strings.TrimSpace(e.To) != "" {
				rels = append(rels, e.To)
			}
		}
		if len(rels) > 0 {
			fmt.Fprintf(&b, " Related: %s.", strings.Join(rels, ", "))
		}
		return capText(b.String(), maxSurfaceContextChars)

	case "markets":
		items, err := s.Watchlist.List(ctx, userID)
		if err != nil || len(items) == 0 {
			return ""
		}
		syms := make([]string, 0, len(items))
		for _, it := range items {
			syms = append(syms, it.Symbol)
		}
		return capText("Current context — the user is on the Markets surface. Watchlist: "+strings.Join(syms, ", ")+".", maxSurfaceContextChars)
	}
	return ""
}

// toolLabel is the human-facing phrase shown while a tool runs (a "Searching…" affordance).
func toolLabel(name string) string {
	switch name {
	case "search":
		return "Searching your workspace"
	case "get_entity":
		return "Looking up an entity"
	case "get_insights":
		return "Reading your insights"
	case "list_artifacts":
		return "Listing your artifacts"
	case "get_artifact":
		return "Reading an artifact"
	case "get_page":
		return "Reading a page"
	case "get_watchlist":
		return "Checking your watchlist"
	case "get_browser_tree", "list_folders":
		return "Browsing your workspace"
	case "search_pages":
		return "Searching your pages"
	case "get_bars":
		return "Reading price history"
	case "get_quote":
		return "Fetching a live quote"
	case "create_shard":
		return "Creating a shard"
	case "list_shard_files", "read_shard_file":
		return "Reading shard files"
	case "write_shard_file", "edit_shard_file":
		return "Writing shard code"
	case "build_shard":
		return "Building the shard"
	case "delete_shard_file":
		return "Deleting a shard file"
	case "publish_shard":
		return "Publishing the shard"
	case "preview_open", "preview_navigate", "preview_snapshot", "preview_eval", "preview_click", "preview_console", "preview_close", "preview_restart":
		return "Previewing the shard"
	default:
		return "Working"
	}
}

func artifactNoun(t string) string {
	switch t {
	case "app":
		return "shard"
	case "page":
		return "page"
	default:
		return "artifact"
	}
}

// citationKindForArtifact maps an artifact type to the citation kind the client routes on.
// "app" is a shard; every artifact kind opens in the work pane, which the client's nav does
// for both "page" and "shard", so non-shard artifacts route as "page".
func citationKindForArtifact(t string) string {
	if t == "app" {
		return "shard"
	}
	return "page"
}
