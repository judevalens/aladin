package service

import (
	"context"
	"encoding/json"
	"strings"

	"aladin/backend_v2/internal/llm"
)

// Shard (doc-surface) authoring tools — the copilot can create an interactive React "shard",
// write/edit its files (each write triggers a draft build so diagnostics ride back), and rebuild.
// All additive (drafts are reversible/rebuildable); publishing a shard live is a separate,
// gated action (not yet exposed). Reuses the exact services the MCP uses.

const copilotShardManifest = "anchors.json"

// starterIndexTSX / starterAnchorsJSON mirror the MCP's shard seed so a fresh shard builds.
const copilotStarterIndexTSX = `import { createRoot } from "react-dom/client";
import { Page, Section, Region } from "@aladin/kit";

function App() {
  return (
    <Page>
      <Section>
        <Region anchor="intro" kind="narrative">
          <h1 className="text-2xl font-display text-ink">New shard</h1>
          <p className="mt-2 text-ink-2">Composed from @aladin/kit.</p>
        </Region>
      </Section>
    </Page>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
`

const copilotStarterAnchorsJSON = `{
  "version": 1,
  "intent": "Describe what this shard is for — written so a cold agent could rebuild its idea.",
  "anchors": [
    { "id": "intro", "kind": "narrative", "route": "#/", "source": "index.tsx", "meaning": "The shard's intro region." }
  ]
}
`

// copilotProvenance tags agent-authored artifacts so they're attributable (mirrors the MCP's
// metadata.agent.source).
func copilotProvenance() map[string]any {
	return map[string]any{"agent": map[string]any{"source": "copilot"}}
}

// shardToolDefs returns the shard tools; hasPreview gates the headless preview loop.
func shardToolDefs(hasPreview bool) []llm.ChatToolDef {
	defs := []llm.ChatToolDef{
		{
			Name:        "create_shard",
			Description: "Create a new shard (an interactive React app rendered in a sandboxed iframe — distinct from a page/note). Seeds index.tsx composed from @aladin/kit. Then write_shard_file to author it and build_shard to compile. Returns the new shard's id.",
			Parameters: objSchema(map[string]any{
				"title":    strProp("The shard title."),
				"summary":  strProp("Optional one-line summary of what the shard is."),
				"folderId": strProp("Optional folder id to create it in."),
			}, "title"),
		},
		{
			Name:        "list_shard_files",
			Description: "List files in a shard's directory (relative path optional; defaults to the shard root).",
			Parameters: objSchema(map[string]any{
				"shardId": strProp("The shard (artifact) id."),
				"path":    strProp("Optional relative subdirectory."),
			}, "shardId"),
		},
		{
			Name:        "read_shard_file",
			Description: "Read a file from a shard's directory (e.g. index.tsx).",
			Parameters: objSchema(map[string]any{
				"shardId": strProp("The shard id."),
				"path":    strProp("File path, e.g. index.tsx."),
			}, "shardId", "path"),
		},
		{
			Name:        "write_shard_file",
			Description: "Write (create or overwrite) a file in a shard (index.tsx, components, css). A draft build runs automatically and its diagnostics come back in `build` — read them to confirm it compiles; the user sees the draft update live. Author shards with @aladin/kit + Tailwind/Aladin token classes.",
			Parameters: objSchema(map[string]any{
				"shardId": strProp("The shard id."),
				"path":    strProp("File path to write, e.g. index.tsx."),
				"content": strProp("Full file contents."),
			}, "shardId", "path", "content"),
		},
		{
			Name:        "edit_shard_file",
			Description: "Edit a shard file by exact string replacement. old_string must appear exactly once (include surrounding context) unless replaceAll is true. Triggers a draft build; prefer this over write_shard_file for surgical changes.",
			Parameters: objSchema(map[string]any{
				"shardId":    strProp("The shard id."),
				"path":       strProp("File path to edit."),
				"oldString":  strProp("Exact text to replace."),
				"newString":  strProp("Replacement text."),
				"replaceAll": map[string]any{"type": "boolean", "description": "Replace all occurrences (default false)."},
			}, "shardId", "path", "oldString", "newString"),
		},
		{
			Name:        "build_shard",
			Description: "Build a shard (draft channel) and return the build log. On failure, read the log, fix the files with write/edit_shard_file, and build again.",
			Parameters: objSchema(map[string]any{
				"shardId": strProp("The shard id."),
			}, "shardId"),
		},
		{
			Name:        "delete_shard_file",
			Description: "Delete a file from a shard. Destructive — the user is asked to approve before it runs.",
			Parameters: objSchema(map[string]any{
				"shardId": strProp("The shard id."),
				"path":    strProp("File path to delete."),
			}, "shardId", "path"),
		},
		{
			Name:        "publish_shard",
			Description: "Publish a shard — build the published bundle and make it live at /content/{id}/. Destructive (user approves). Build/preview it first; verify routes mount with the preview tools before publishing.",
			Parameters: objSchema(map[string]any{
				"shardId": strProp("The shard id."),
				"summary": strProp("Optional summary recorded on publish."),
			}, "shardId"),
		},
	}
	if hasPreview {
		defs = append(defs,
			llm.ChatToolDef{
				Name:        "preview_open",
				Description: "Build a shard and open it in a live headless browser tab (kept across calls). Returns whether React mounted, the URL, and any console/exceptions. Preview before publishing; re-open after edits.",
				Parameters: objSchema(map[string]any{
					"shardId": strProp("The shard id."),
					"channel": strProp("draft (default) or published."),
				}, "shardId"),
			},
			llm.ChatToolDef{
				Name:        "preview_navigate",
				Description: "Navigate the open preview to an in-app hash route (e.g. #/section). Returns the post-navigation state.",
				Parameters: objSchema(map[string]any{
					"shardId": strProp("The shard id."),
					"route":   strProp("Hash route, e.g. #/section."),
				}, "shardId", "route"),
			},
			llm.ChatToolDef{
				Name:        "preview_snapshot",
				Description: "Return the current preview view's text + a compact DOM outline. Use to verify what actually rendered.",
				Parameters:  objSchema(map[string]any{"shardId": strProp("The shard id.")}, "shardId"),
			},
			llm.ChatToolDef{
				Name:        "preview_eval",
				Description: "Evaluate a JavaScript expression in the open preview and return its JSON value (inspect state/DOM).",
				Parameters: objSchema(map[string]any{
					"shardId": strProp("The shard id."),
					"expr":    strProp("JS expression, e.g. document.querySelectorAll('button').length."),
				}, "shardId", "expr"),
			},
			llm.ChatToolDef{
				Name:        "preview_click",
				Description: "Click the first element matching a CSS selector in the open preview, then return the resulting state.",
				Parameters: objSchema(map[string]any{
					"shardId":  strProp("The shard id."),
					"selector": strProp("CSS selector."),
				}, "shardId", "selector"),
			},
			llm.ChatToolDef{
				Name:        "preview_console",
				Description: "Return the console log lines + uncaught exceptions from the open preview.",
				Parameters:  objSchema(map[string]any{"shardId": strProp("The shard id.")}, "shardId"),
			},
			llm.ChatToolDef{
				Name:        "preview_close",
				Description: "Close the live preview tab for a shard and free its resources.",
				Parameters:  objSchema(map[string]any{"shardId": strProp("The shard id.")}, "shardId"),
			},
			llm.ChatToolDef{
				Name:        "preview_restart",
				Description: "Force-restart the headless preview renderer (tears down all preview tabs) to recover a wedged renderer.",
				Parameters:  objSchema(map[string]any{}),
			},
		)
	}
	return defs
}

// runShardTool dispatches a shard-authoring tool. ok=false means the name isn't a shard tool.
func (s *defaultCopilotService) runShardTool(ctx context.Context, name, args string) (string, []Citation, bool, error) {
	switch name {
	case "create_shard":
		var a struct {
			Title    string  `json:"title"`
			Summary  string  `json:"summary"`
			FolderID *string `json:"folderId"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		if strings.TrimSpace(a.Title) == "" {
			return "", nil, true, BadRequest("title is required")
		}
		payload := ArtifactPayload{Type: "app", Title: a.Title, FolderID: a.FolderID, Metadata: copilotProvenance()}
		if s := strings.TrimSpace(a.Summary); s != "" {
			payload.Summary = &s
		}
		created, err := s.Artifacts.Create(ctx, payload)
		if err != nil {
			return "", nil, true, err
		}
		id := created.Artifact.ID
		if _, err := s.DocStore.EnsurePageDir(ctx, id); err != nil {
			_, _ = s.Artifacts.Delete(ctx, id)
			return "", nil, true, err
		}
		if err := s.DocStore.WriteFile(ctx, id, "index.tsx", []byte(copilotStarterIndexTSX)); err != nil {
			_, _ = s.Artifacts.Delete(ctx, id)
			return "", nil, true, err
		}
		if err := s.DocStore.WriteFile(ctx, id, copilotShardManifest, []byte(copilotStarterAnchorsJSON)); err != nil {
			_, _ = s.Artifacts.Delete(ctx, id)
			return "", nil, true, err
		}
		res := jsonString(map[string]any{"id": id, "title": created.Artifact.Title})
		return res, []Citation{{Kind: "shard", ID: id, Title: created.Artifact.Title}}, true, nil

	case "list_shard_files":
		var a struct {
			ShardID string `json:"shardId"`
			Path    string `json:"path"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		if err := s.requireShard(ctx, a.ShardID); err != nil {
			return "", nil, true, err
		}
		entries, err := s.DocStore.ListDir(ctx, a.ShardID, a.Path)
		if err != nil {
			return "", nil, true, err
		}
		return jsonString(map[string]any{"entries": entries}), nil, true, nil

	case "read_shard_file":
		var a struct {
			ShardID string `json:"shardId"`
			Path    string `json:"path"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		if err := s.requireShard(ctx, a.ShardID); err != nil {
			return "", nil, true, err
		}
		data, err := s.DocStore.ReadFile(ctx, a.ShardID, a.Path)
		if err != nil {
			return "", nil, true, err
		}
		return jsonString(map[string]any{"content": string(data)}), nil, true, nil

	case "write_shard_file":
		var a struct {
			ShardID string `json:"shardId"`
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		if err := s.requireShard(ctx, a.ShardID); err != nil {
			return "", nil, true, err
		}
		if strings.TrimSpace(a.Path) == "" {
			return "", nil, true, BadRequest("path is required")
		}
		if err := s.DocStore.WriteFile(ctx, a.ShardID, a.Path, []byte(a.Content)); err != nil {
			return "", nil, true, err
		}
		return jsonString(map[string]any{"ok": true, "path": a.Path, "build": s.draftBuild(ctx, a.ShardID)}), nil, true, nil

	case "edit_shard_file":
		var a struct {
			ShardID    string `json:"shardId"`
			Path       string `json:"path"`
			OldString  string `json:"oldString"`
			NewString  string `json:"newString"`
			ReplaceAll bool   `json:"replaceAll"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		if err := s.requireShard(ctx, a.ShardID); err != nil {
			return "", nil, true, err
		}
		if strings.TrimSpace(a.Path) == "" || a.OldString == "" {
			return "", nil, true, BadRequest("path and oldString are required")
		}
		data, err := s.DocStore.ReadFile(ctx, a.ShardID, a.Path)
		if err != nil {
			return "", nil, true, err
		}
		content := string(data)
		count := strings.Count(content, a.OldString)
		if count == 0 {
			return "", nil, true, BadRequest("oldString not found in " + a.Path)
		}
		if count > 1 && !a.ReplaceAll {
			return "", nil, true, BadRequest("oldString is ambiguous; add context or set replaceAll")
		}
		if a.ReplaceAll {
			content = strings.ReplaceAll(content, a.OldString, a.NewString)
		} else {
			content = strings.Replace(content, a.OldString, a.NewString, 1)
		}
		if err := s.DocStore.WriteFile(ctx, a.ShardID, a.Path, []byte(content)); err != nil {
			return "", nil, true, err
		}
		return jsonString(map[string]any{"ok": true, "path": a.Path, "replacements": count, "build": s.draftBuild(ctx, a.ShardID)}), nil, true, nil

	case "build_shard":
		var a struct {
			ShardID string `json:"shardId"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		if err := s.requireShard(ctx, a.ShardID); err != nil {
			return "", nil, true, err
		}
		return jsonString(map[string]any{"build": s.draftBuild(ctx, a.ShardID)}), nil, true, nil

	case "delete_shard_file":
		var a struct {
			ShardID string `json:"shardId"`
			Path    string `json:"path"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		if err := s.requireShard(ctx, a.ShardID); err != nil {
			return "", nil, true, err
		}
		if strings.TrimSpace(a.Path) == "" {
			return "", nil, true, BadRequest("path is required")
		}
		if err := s.DocStore.DeleteFile(ctx, a.ShardID, a.Path); err != nil {
			return "", nil, true, err
		}
		return jsonString(map[string]any{"ok": true, "deleted": a.Path, "build": s.draftBuild(ctx, a.ShardID)}), nil, true, nil

	case "publish_shard":
		var a struct {
			ShardID string `json:"shardId"`
			Summary string `json:"summary"`
		}
		_ = json.Unmarshal([]byte(args), &a)
		if err := s.requireShard(ctx, a.ShardID); err != nil {
			return "", nil, true, err
		}
		res, err := s.ShardBuild.Build(ctx, a.ShardID, ChannelPublished)
		if err != nil {
			return "", nil, true, err
		}
		if !res.OK {
			return jsonString(map[string]any{"ok": false, "log": res.Log, "note": "build failed — not published"}), nil, true, nil
		}
		if summary := strings.TrimSpace(a.Summary); summary != "" {
			_, _ = s.Artifacts.Update(ctx, a.ShardID, ArtifactPatch{Summary: &summary})
		}
		_, _ = s.ShardBuild.Build(ctx, a.ShardID, ChannelDraft) // reconcile draft state
		return jsonString(map[string]any{"ok": true, "servedUrl": "/content/" + a.ShardID + "/"}), []Citation{{Kind: "shard", ID: a.ShardID}}, true, nil

	case "preview_open", "preview_navigate", "preview_snapshot", "preview_eval", "preview_click", "preview_console", "preview_close", "preview_restart":
		return s.runPreviewTool(ctx, name, args)
	}
	return "", nil, false, nil
}

// runPreviewTool drives the headless preview loop. Degrades cleanly ("renderer unavailable")
// when no Chrome is present on the host.
func (s *defaultCopilotService) runPreviewTool(ctx context.Context, name, args string) (string, []Citation, bool, error) {
	if s.Preview == nil {
		return `{"error":"preview not available"}`, nil, true, nil
	}
	var a struct {
		ShardID  string `json:"shardId"`
		Channel  string `json:"channel"`
		Route    string `json:"route"`
		Expr     string `json:"expr"`
		Selector string `json:"selector"`
	}
	_ = json.Unmarshal([]byte(args), &a)
	if name == "preview_restart" {
		if err := s.Preview.Reset(ctx); err != nil {
			return "", nil, true, err
		}
		return `{"ok":true}`, nil, true, nil
	}
	if err := s.requireShard(ctx, a.ShardID); err != nil {
		return "", nil, true, err
	}
	var (
		st  PreviewState
		err error
	)
	switch name {
	case "preview_open":
		channel := ChannelDraft
		if strings.EqualFold(strings.TrimSpace(a.Channel), "published") {
			channel = ChannelPublished
		}
		st, err = s.Preview.Open(ctx, a.ShardID, channel)
	case "preview_navigate":
		st, err = s.Preview.Navigate(ctx, a.ShardID, a.Route)
	case "preview_snapshot":
		st, err = s.Preview.Snapshot(ctx, a.ShardID)
	case "preview_eval":
		st, err = s.Preview.Eval(ctx, a.ShardID, a.Expr)
	case "preview_click":
		st, err = s.Preview.Click(ctx, a.ShardID, a.Selector)
	case "preview_console":
		st, err = s.Preview.Console(ctx, a.ShardID)
	case "preview_close":
		if cerr := s.Preview.Close(ctx, a.ShardID); cerr != nil {
			return "", nil, true, cerr
		}
		return `{"ok":true}`, nil, true, nil
	}
	if err != nil {
		return "", nil, true, err
	}
	return jsonString(st), nil, true, nil
}

// draftBuild runs a synchronous draft build and returns a compact {ok, log} for the model to
// read. A build error is folded into ok:false rather than failing the write (the file IS written).
func (s *defaultCopilotService) draftBuild(ctx context.Context, shardID string) map[string]any {
	res, err := s.ShardBuild.Build(ctx, shardID, ChannelDraft)
	if err != nil {
		return map[string]any{"ok": false, "log": err.Error()}
	}
	return map[string]any{"ok": res.OK, "log": res.Log}
}

// requireShard verifies the id is an existing shard (app artifact) owned by the caller.
func (s *defaultCopilotService) requireShard(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return BadRequest("shardId is required")
	}
	art, err := s.Artifacts.Get(ctx, id)
	if err != nil {
		return err
	}
	if art.Type != "app" {
		return BadRequest("that artifact is not a shard")
	}
	return nil
}
