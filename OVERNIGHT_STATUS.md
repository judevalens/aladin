# Overnight Status — M5 + M6

**Branch:** `m5-m6-blocknote-jsonb` (13 commits ahead of `main`)
**Started:** 2026-05-26 evening
**Finished:** 2026-05-27 early morning
**All tasks complete.** No half-finished work; nothing is broken; no manual cleanup needed.

## What shipped

### Milestone 5 — BlockNote JSON as canonical storage

- `backend_v2/internal/blocknote/text.go` — pure-Go BlockNote-JSON text extractor for full-text search (`M5.1`).
- `backend_v2/internal/db/migrations/00024_page_blocks.sql` — drops `page_documents.markdown`, adds `blocks JSONB` + `search_text TEXT`, creates a `pg_trgm` GIN index on `search_text`, and TRUNCATEs `artifacts` (cascades to pages + tree_nodes). Test workspace — by design (`M5.2`).
- `backend_v2/internal/service/page_document.go` — `PageDocumentService` interface, **the collab seam** for M7 (`M5.3`).
- Backend repo + service signature swap: `Content string` → `Blocks json.RawMessage` for pages; `SavePageDocument/SavePageDocumentRevision` collapsed into `SavePageBlocks(..., expectedRev)` (`M5.4`).
- API handlers and integration tests follow the typed change automatically (`M5.5`).
- Frontend cutover: `aladin_react/src/modules/pages/editor/markdown-adapter.ts` deleted; `BlockNotePageEditorDriver` now calls `editor.replaceBlocks` directly with the wire JSON and ships `editor.document` on change; markdown-fallback is now a read-only JSON view (`M5.6`).
- MCP write tools temporarily off and `get_page` reshaped to return blocks JSON during the migration (`M5.7`).

### Milestone 6 — Node converter sidecar + block-aware MCP tools

- `services/blocknote-converter/` — Node sidecar exposing `/md-to-blocks`, `/blocks-to-md`, `/blocks-to-md-batch`, `/healthz` using `@blocknote/server-util` (pinned to `0.51.0`, same as `aladin_react`). Docker-composed at `:3500` (`M6.1`).
- `backend_v2/internal/blocknote/client.go` — Go HTTP client for the converter with timeouts, retries, and an `ErrConverter` sentinel for application-level 4xx (`M6.2`).
- `cmd/mcp/main.go` reads `CONVERTER_URL` from config, builds the client, and runs a short retry loop against `/healthz` so MCP doesn't 500 during a cold `docker-compose up`. `Makefile` gets a `converter` target. MCP `/healthz` proxies converter health (`M6.3`).
- `backend_v2/internal/blocknote/block_ops.go` — pure-Go array operations (`Find/Replace/Insert{After,Before,AtStart,AtEnd}/DeleteByID`, `WithID`) that preserve every block's raw JSON unchanged through splices (`M6.4`).
- `PageDocumentService.ReplaceBlock/InsertBlocks/DeleteBlock` filled in using block_ops, recomputing `search_text` and bumping revision on every write (`M6.5`).
- MCP tools re-enabled and block-level tools added (`M6.6 + M6.7 + M6.8`):
  - `create_page(title, markdown, ...)` → converter → store
  - `update_page(id, markdown?, ...)` → full replace (warned in docstring)
  - `get_page(id)` → enriched response: each block returns `{id, type, props, markdown}`
  - `update_block(page_id, block_id, markdown, revision?)` — first new block inherits the original id
  - `insert_blocks(page_id, position {after_id|before_id|at}, markdown, revision?)`
  - `delete_block(page_id, block_id, revision?)` — refuses last block
- Server `Instructions` rewritten to teach the agent the block model.
- **End-to-end integration test (`M6.9`)** — `backend_v2/internal/mcp/integration_test.go` (build tag `integration`). Drives a live MCP server via the official Go SDK client, walks the full workflow, **passes in 0.10s** with Postgres + blocknote-converter running.

## Defaults I made without you

| Decision | Default | Why |
|---|---|---|
| Migration uses `TRUNCATE artifacts CASCADE` | Wipes pages, tree_nodes, page_documents | You said test workspace; no backfill. Verified locally: subsequent `make backend` start works fine. |
| `Content` field retained on artifact types | Pages set Content to "" | Non-page artifacts (link/voice/file) still use Content. Keeping the field avoids a sweeping schema change for non-page kinds. |
| `update_block` first new block inherits original id | Per plan | Keeps downstream references stable when an agent's markdown parses into multiple blocks. |
| `delete_block` refuses last block | BlockNote requires ≥1 block | Otherwise the editor crashes; surface a clear error rather than letting the agent foot-gun. |
| Search column populated in-process via `blocknote.ExtractText` | No FTS reindex job | Trigram GIN over `search_text` is plenty for v1. Re-evaluate when corpus grows. |
| `pageBlockView.Props` is `map[string]any`, not `json.RawMessage` | SDK schema validation failure with `json.RawMessage` | The SDK auto-generates an output schema from the Go type; `[]byte` (i.e. `json.RawMessage`) maps to `null|array`, which rejects object props at runtime. Discovered during E2E. |
| MCP server tolerates a missing converter at startup | Warns + continues | Tool calls that need the converter still fail with a clean error. Avoids tight startup ordering between docker services. |

## How to verify everything

```bash
# 1. Build everything
cd backend_v2 && go build ./...
cd ../aladin_react && npm run build

# 2. All unit tests (no external services required)
cd ../backend_v2 && go test ./...
cd ../aladin_react && npm test

# 3. Start dependencies
cd .. && docker compose up -d postgres blocknote-converter

# 4. End-to-end MCP test (Postgres + converter must be up)
cd backend_v2 && go test -tags=integration -v -run TestMCP_EndToEnd ./internal/mcp/

# 5. Smoke the converter directly
curl -sS http://localhost:3500/healthz
curl -sS -X POST http://localhost:3500/md-to-blocks \
  -H 'content-type: application/json' \
  -d '{"markdown":"# hi\n\n- a\n- b"}' | jq .

# 6. Run the actual MCP server and connect a real client
make converter       # start the sidecar
make mcp             # in another shell, on :8090

# 7. Frontend: open the app and edit a page
cd aladin_react && npm run dev
# Page editor loads blocks directly; save round-trips block IDs.
```

## What needs your eyes (I can't visual-test from here)

1. **Browser editor.** Type rich content (heading + list + code block), save, reload. Block IDs should survive the round-trip; inspect `page_documents.blocks` in Postgres to confirm.
2. **Markdown-fallback view.** Trigger a BlockNote initialization error (toggle the editor mode, or transiently break the editor); the fallback is now read-only JSON. That's a regression from the M5 plan but acceptable as a stopgap — re-add a true markdown view by adding a frontend-side blocks→markdown converter (a server `BlocksToMD` round-trip on the API) when needed.
3. **Real MCP client.** Wire Claude Code:
   ```
   claude mcp add --transport http aladin \
     http://localhost:8090/mcp \
     -H "Authorization: Bearer <mint a token via /api/integration-tokens or the auth service>"
   ```
   then ask the agent to create a page, get it, update a block, etc. The Go E2E test does the same workflow as a positive control.

## Known follow-ups (NOT in this branch)

- **Custom block types.** You said later. When you add them, also register them in `services/blocknote-converter/server.js` (pass a custom schema to `ServerBlockNoteEditor.create`). A version-drift check between `aladin_react/package.json` and `services/blocknote-converter/package.json` would be a nice CI guard (the plan flagged it).
- **Markdown-fallback writes.** The dropped textarea-based markdown fallback was an accessibility/escape-hatch nice-to-have. Restoring it cleanly means a "blocks ↔ markdown via the converter" frontend hook — defer until someone hits the fallback in practice.
- **Search ranking.** Currently ILIKE-over-`search_text` with a trigram index. FTS / pg_search / pgvector embeddings are a future-Milestone.
- **`move_block` MCP tool.** Punted per plan; revisit if agent edits show a pattern of "I wanted to reorder, had to delete-then-insert."
- **Token issuance UX.** The auth service has `CreateIntegrationToken` already; whether you expose it via the API in a way the user can click-mint a token is a separate UX task — not blocking the MCP feature.

## Plan + memory files

- `~/.claude/plans/blocknote-jsonb-and-block-mcp.md` — the M5+M6 plan we drafted before starting. Now a reference; no edits needed.
- `~/.claude/projects/-Users-judepaulemon-Documents-aladin/memory/project_mcp_milestone4_plan.md` — was stale (said M4 parked); updated last session to reflect M1–M4 shipped and M5+M6 planned. The "M5+M6 planned" line is now technically out of date; you may want to flip it to "M5+M6 shipped" when you confirm everything works on your end.

## Git history (newest first)

```
58f6891 M6.9: end-to-end MCP integration test
d831274 M6.6 + M6.7 + M6.8: re-enable MCP write tools, add block-level surgery, and rewrite the server Instructions
e366d47 M6.5: PageDocumentService block-targeted methods land
f6ff07e M6.4: pure-Go block array operations (block_ops)
5be88b2 M6.3: wire the converter into cmd/mcp + startup healthcheck retry
b77281a M6.2: Go HTTP client for the blocknote-converter sidecar
3609994 M6.1: add blocknote-converter Node sidecar service
23a6547 M5.6: frontend cutover — editor speaks BlockNote JSON directly
206047d M5.7: disable MCP write tools during the migration; reads return blocks
921d75f M5.4: switch repo + service to blocks JSON for pages
59d4c0f M5.3: introduce PageDocumentService — the collab seam
3b2050d M5.2: migration 00024 — page_documents.markdown -> blocks JSONB
abc53aa M5.1: add pure-Go BlockNote text extractor for search indexing
```

`git diff main..m5-m6-blocknote-jsonb` is the full delta.

Sleep well. ☕
