package service

import (
	"context"
	"encoding/json"
	"strings"

	"aladin/backend_v2/internal/blocknote"
	"aladin/backend_v2/internal/llm"
)

// Page-authoring tools — the copilot can create/edit BlockNote pages (the user's writing) via
// the same converter (markdown⇄blocks) + live-doc bridge the MCP uses. Additive edits run
// directly; full-document replace (update_page) and block deletion are destructive → the
// approval gate. Plus light additive writes: add_to_watchlist, draw_edge.

func pageToolDefs() []llm.ChatToolDef {
	return []llm.ChatToolDef{
		{
			Name:        "create_page",
			Description: "Create a page (a BlockNote document — the user's writing/notes; distinct from a shard). Body is markdown; the server converts it to blocks. Returns the new page id.",
			Parameters: objSchema(map[string]any{
				"title":    strProp("The page title."),
				"markdown": strProp("Optional page body as markdown."),
				"summary":  strProp("Optional one-line summary."),
				"folderId": strProp("Optional folder id."),
			}, "title"),
		},
		{
			Name:        "get_page_blocks",
			Description: "Read a page's blocks as {id, text} — the precursor to surgical edits (update_block/delete_block target a block id). Use this to find block ids before editing.",
			Parameters:  objSchema(map[string]any{"pageId": strProp("The page id.")}, "pageId"),
		},
		{
			Name:        "insert_blocks",
			Description: "Insert markdown as new blocks into a page. position is optional: {afterId} / {beforeId} / {at:\"start\"|\"end\"} (default end).",
			Parameters: objSchema(map[string]any{
				"pageId":   strProp("The page id."),
				"markdown": strProp("Markdown for the new content."),
				"afterId":  strProp("Insert after this block id."),
				"beforeId": strProp("Insert before this block id."),
				"at":       strProp("\"start\" or \"end\"."),
			}, "pageId", "markdown"),
		},
		{
			Name:        "update_block",
			Description: "Replace a single block by id with new markdown (surgical; the first produced block keeps the original id). Prefer this over update_page for precise edits.",
			Parameters: objSchema(map[string]any{
				"pageId":   strProp("The page id."),
				"blockId":  strProp("The block id to replace."),
				"markdown": strProp("Replacement markdown."),
			}, "pageId", "blockId", "markdown"),
		},
		{
			Name:        "update_page",
			Description: "Replace a page's ENTIRE body with new markdown, and/or update its title/summary/folder. Destructive — wipes existing block ids — so the user is asked to approve. Prefer update_block/insert_blocks for surgical edits.",
			Parameters: objSchema(map[string]any{
				"pageId":   strProp("The page id."),
				"markdown": strProp("Optional new full-document markdown (replaces all blocks)."),
				"title":    strProp("Optional new title."),
				"summary":  strProp("Optional new summary."),
			}, "pageId"),
		},
		{
			Name:        "delete_block",
			Description: "Delete a block from a page by id. Destructive — the user approves. Refuses to remove the last block.",
			Parameters: objSchema(map[string]any{
				"pageId":  strProp("The page id."),
				"blockId": strProp("The block id to delete."),
			}, "pageId", "blockId"),
		},
		{
			Name:        "add_to_watchlist",
			Description: "Add a ticker to the user's Markets watchlist by symbol.",
			Parameters:  objSchema(map[string]any{"symbol": strProp("Ticker symbol, e.g. NVDA.")}, "symbol"),
		},
		{
			Name:        "draw_edge",
			Description: "Draw a typed relationship edge between two entities (e.g. rel \"competes_with\"). Additive.",
			Parameters: objSchema(map[string]any{
				"fromId": strProp("Source entity id."),
				"toId":   strProp("Target entity id."),
				"rel":    strProp("Relationship type, e.g. competes_with, supplies, owns."),
				"why":    strProp("Optional short rationale."),
			}, "fromId", "toId", "rel"),
		},
	}
}

// runPageTool dispatches a page-authoring / light-write tool. ok=false ⇒ not a page tool.
func (s *defaultCopilotService) runPageTool(ctx context.Context, userID, name, args string) (string, []Citation, bool, error) {
	switch name {
	case "create_page":
		var a struct {
			Title    string `json:"title"`
			Markdown string `json:"markdown"`
			Summary  string `json:"summary"`
			FolderID string `json:"folderId"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		if strings.TrimSpace(a.Title) == "" {
			return "", nil, true, BadRequest("title is required")
		}
		payload := ArtifactPayload{Type: "page", Title: a.Title, Metadata: copilotProvenance()}
		if f := strings.TrimSpace(a.FolderID); f != "" {
			payload.FolderID = &f
		}
		if sm := strings.TrimSpace(a.Summary); sm != "" {
			payload.Summary = &sm
		}
		created, err := s.Artifacts.Create(ctx, payload)
		if err != nil {
			return "", nil, true, err
		}
		id := created.Artifact.ID
		if strings.TrimSpace(a.Markdown) != "" {
			blocks, cerr := s.Converter.MDToBlocks(ctx, a.Markdown)
			if cerr != nil {
				return "", nil, true, cerr
			}
			if _, berr := s.Bridge.ApplyOperation(ctx, blocknote.BridgeOp{PageID: id, Op: "replace_all", Blocks: blocks, Agent: copilotBridgeAgent()}); berr != nil {
				return "", nil, true, berr
			}
		}
		return jsonString(map[string]any{"id": id, "title": created.Artifact.Title}), []Citation{{Kind: "page", ID: id, Title: created.Artifact.Title}}, true, nil

	case "get_page_blocks":
		var a struct {
			PageID string `json:"pageId"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		page, err := s.Bridge.GetPage(ctx, a.PageID)
		if err != nil {
			return "", nil, true, err
		}
		parsed, err := blocknote.ParseBlocks(page.Blocks)
		if err != nil {
			return "", nil, true, err
		}
		out := make([]map[string]string, 0, len(parsed))
		for _, b := range parsed {
			text, _ := blocknote.ExtractText(json.RawMessage("[" + string(b.Raw) + "]"))
			out = append(out, map[string]string{"id": b.ID, "text": text})
		}
		return jsonString(map[string]any{"blocks": out}), nil, true, nil

	case "insert_blocks":
		var a struct {
			PageID   string `json:"pageId"`
			Markdown string `json:"markdown"`
			AfterID  string `json:"afterId"`
			BeforeID string `json:"beforeId"`
			At       string `json:"at"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		blocks, err := s.Converter.MDToBlocks(ctx, a.Markdown)
		if err != nil {
			return "", nil, true, err
		}
		op := blocknote.BridgeOp{PageID: a.PageID, Op: "insert_blocks", Blocks: blocks, Agent: copilotBridgeAgent()}
		switch {
		case strings.TrimSpace(a.AfterID) != "":
			op.BlockID = a.AfterID
			op.Placement = "after"
		case strings.TrimSpace(a.BeforeID) != "":
			op.BlockID = a.BeforeID
			op.Placement = "before"
		case strings.EqualFold(strings.TrimSpace(a.At), "start"):
			zero := 0
			op.Position = &zero
		}
		res, err := s.Bridge.ApplyOperation(ctx, op)
		if err != nil {
			return "", nil, true, err
		}
		return jsonString(map[string]any{"ok": true, "insertedBlockIds": res.AffectedBlockIDs}), nil, true, nil

	case "update_block":
		var a struct {
			PageID   string `json:"pageId"`
			BlockID  string `json:"blockId"`
			Markdown string `json:"markdown"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		blocks, err := s.Converter.MDToBlocks(ctx, a.Markdown)
		if err != nil {
			return "", nil, true, err
		}
		blocks = withFirstBlockID(blocks, a.BlockID)
		res, err := s.Bridge.ApplyOperation(ctx, blocknote.BridgeOp{PageID: a.PageID, Op: "replace_block", BlockID: a.BlockID, Blocks: blocks, Agent: copilotBridgeAgent()})
		if err != nil {
			return "", nil, true, err
		}
		return jsonString(map[string]any{"ok": true, "replacedBlockCount": len(res.AffectedBlockIDs)}), nil, true, nil

	case "update_page":
		var a struct {
			PageID   string  `json:"pageId"`
			Markdown *string `json:"markdown"`
			Title    *string `json:"title"`
			Summary  *string `json:"summary"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		if strings.TrimSpace(a.PageID) == "" {
			return "", nil, true, BadRequest("pageId is required")
		}
		if a.Title != nil || a.Summary != nil {
			if _, err := s.Artifacts.Update(ctx, a.PageID, ArtifactPatch{Title: a.Title, Summary: a.Summary}); err != nil {
				return "", nil, true, err
			}
		}
		if a.Markdown != nil {
			blocks, err := s.Converter.MDToBlocks(ctx, *a.Markdown)
			if err != nil {
				return "", nil, true, err
			}
			if _, err := s.Bridge.ApplyOperation(ctx, blocknote.BridgeOp{PageID: a.PageID, Op: "replace_all", Blocks: blocks, Agent: copilotBridgeAgent()}); err != nil {
				return "", nil, true, err
			}
		}
		return jsonString(map[string]any{"ok": true, "id": a.PageID}), []Citation{{Kind: "page", ID: a.PageID}}, true, nil

	case "delete_block":
		var a struct {
			PageID  string `json:"pageId"`
			BlockID string `json:"blockId"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		if _, err := s.Bridge.ApplyOperation(ctx, blocknote.BridgeOp{PageID: a.PageID, Op: "delete_block", BlockID: a.BlockID, Agent: copilotBridgeAgent()}); err != nil {
			return "", nil, true, err
		}
		return jsonString(map[string]any{"ok": true, "deleted": a.BlockID}), nil, true, nil

	case "add_to_watchlist":
		var a struct {
			Symbol string `json:"symbol"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		sym := strings.ToUpper(strings.TrimSpace(a.Symbol))
		if sym == "" {
			return "", nil, true, BadRequest("symbol is required")
		}
		id, ok, err := s.Instruments.ResolveInstrumentID(ctx, sym)
		if err != nil {
			return "", nil, true, err
		}
		if !ok {
			return jsonString(map[string]any{"ok": false, "note": "unknown symbol " + sym}), nil, true, nil
		}
		if err := s.Watchlist.Add(ctx, userID, id); err != nil {
			return "", nil, true, err
		}
		return jsonString(map[string]any{"ok": true, "symbol": sym}), []Citation{{Kind: "ticker", ID: sym, Title: sym}}, true, nil

	case "draw_edge":
		var a struct {
			FromID string `json:"fromId"`
			ToID   string `json:"toId"`
			Rel    string `json:"rel"`
			Why    string `json:"why"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		if err := s.Entities.DrawEdge(ctx, DrawEdgeInput{OwnerUserID: userID, FromID: a.FromID, ToID: a.ToID, Rel: a.Rel, Why: a.Why}); err != nil {
			return "", nil, true, err
		}
		return jsonString(map[string]any{"ok": true}), nil, true, nil
	}
	return "", nil, false, nil
}

func copilotBridgeAgent() *blocknote.BridgeAgent {
	return &blocknote.BridgeAgent{Name: "copilot"}
}

// withFirstBlockID overrides the id of the first block in a blocks array so a replacement keeps
// the target block's id (the markdown may parse into multiple blocks). Returns input unchanged
// if it can't be parsed as a non-empty array.
func withFirstBlockID(blocks json.RawMessage, id string) json.RawMessage {
	var arr []map[string]json.RawMessage
	if err := json.Unmarshal(blocks, &arr); err != nil || len(arr) == 0 {
		return blocks
	}
	idJSON, err := json.Marshal(id)
	if err != nil {
		return blocks
	}
	arr[0]["id"] = idJSON
	out, err := json.Marshal(arr)
	if err != nil {
		return blocks
	}
	return out
}
