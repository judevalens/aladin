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

func shardToolDefs() []llm.ChatToolDef {
	return []llm.ChatToolDef{
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
	}
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
	}
	return "", nil, false, nil
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
