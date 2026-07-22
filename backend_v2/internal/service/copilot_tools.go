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

// citationKindForArtifact maps an artifact type to the citation kind the client routes on.
// "app" is a shard; every artifact kind opens in the work pane, which the client's nav does
// for both "page" and "shard", so non-shard artifacts route as "page".
func citationKindForArtifact(t string) string {
	if t == "app" {
		return "shard"
	}
	return "page"
}
