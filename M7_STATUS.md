# M7 Status — Local-first page content

**Branch:** `m7-local-first-pages` (8 commits ahead of `m5-m6-blocknote-jsonb`)
**All 8 sub-milestones complete.** All builds green, all tests pass, M6 E2E integration test still passes against live Postgres + converter.

## What shipped

| Sub-task | What it does |
|---|---|
| **M7.1** | SQLite schema V8 — `page_content` table (id PK→artifacts, blocks TEXT, revision, sync columns). |
| **M7.2** | `db_get_page_content` / `db_upsert_page_content` Tauri commands. Local upsert queues an outbox mutation. |
| **M7.3** | `api/pages.rs` Rust client. `PageContentRepo` is now an `OutboxProcessor`: outbox `page_content.update` → `PATCH /api/pages/{id}`. |
| **M7.4** | `ArtifactCreateApiInput` sends `blocks: []` for page artifacts (no longer the dropped `content: <markdown>`); fixes the post-M5 "blank page" bug from the sync engine. |
| **M7.5** | `page-repo.ts` swaps `client.fetch` for `invoke`. Cold-cache pull command `db_pull_page_content` for first-time access on a new machine. |
| **M7.6** | Dead `createArtifact`/`renameArtifact` on `ArtifactApi` removed — every caller already went via Tauri. Dual-write surface eliminated. |
| **M7.7** | Realtime: `PageSnapshotPayload` updated to carry `blocks`. New `PageContentEventSubscriber` ingests `page.updated` events, upserts local page_content, emits `PageContentChanged` DataEvent. Frontend `PageDocumentService.applyExternalUpdate` pushes server-side edits into the editor stream. |
| **M7.8** | Build + test sweep. |

## Verification — what passed

```bash
# Go side
cd backend_v2 && go test ./...                                # ok
go test -tags=integration -run TestMCP_EndToEnd ./internal/mcp/  # passes against live Postgres + converter

# Rust shell
cd aladin_react/src-tauri && cargo build                       # clean, no errors

# Frontend
cd aladin_react && npx tsc -b && npm test && npm run build     # clean
```

## What needs your eyes (UI-level checks I can't do)

1. **Editor end-to-end** — Open Aladin, create a page, type, save. Pull `/Users/judepaulemon/Library/Application Support/com.aladin.react/aladin.sqlite` open in a viewer and confirm a `page_content` row exists with the blocks JSON.
2. **Sync push** — Confirm the page also lands in Postgres: `psql ... -c "SELECT id, blocks FROM page_documents;"` should show the same blocks.
3. **MCP edit roundtrip** — With the editor open on a page, have an MCP agent call `update_block` from Claude Code. The local SQLite should pick up the change via the `page.updated` realtime event; whether the editor *visibly* reconciles depends on a known gap (see below).
4. **Offline write** — Stop Postgres (`docker compose stop postgres`), edit a page in Aladin (local save works, outbox fills up), restart Postgres, watch the outbox drain.
5. **Cold-cache pull** — Delete local SQLite, relaunch, open a page that exists only in Postgres → `db_pull_page_content` should fetch and seed.

## Known gaps / follow-ups (NOT in M7)

- **Editor-session reconciliation against external updates.** `PageDocumentService.applyExternalUpdate` pushes the new record into the document stream, but `PageSessionService` only calls `initialize()` once per session, so a mid-edit MCP push won't redraw the editor. **Don't engineer around this.** M8 (Yjs + Hocuspocus collab) dissolves it: with the editor bound to a Y.Doc, remote edits CRDT-merge automatically and the per-session reconciliation step disappears. Leave the gap; let Y.Doc absorb it.
- **Sync outbox replay for the initial `create_artifact` path.** Today `db_create_artifact` writes locally + queues an artifact-create mutation. The mutation's `content` field is now ignored by the Go API for pages (M7.4 sends `blocks: []`). If a user creates a page AND types content before the artifact-create mutation drains, the order is: artifact created in Postgres with empty blocks → then the page_content mutation flushes the actual blocks via `PATCH /api/pages/{id}`. This works (eventual consistency) but the intermediate state shows up briefly on other clients. M8 territory or earlier if it bothers anyone.
- **`PageSnapshotPayload.blocks` parse from RFC3339 timestamps.** The Rust side falls back to `SystemTime::now()` for `updated_at` because the shell intentionally avoids `chrono`. The result is correct ordering for the *local* clock; if the Go and local clocks drift, an old event could win or lose. Matches the existing behavior in browser.rs.
- **M5.6 markdown-fallback view** is read-only JSON post-M5. Restoring a true markdown view requires the frontend to know how to render blocks → markdown. Flagged in OVERNIGHT_STATUS.md; still flagged.

## Git history (newest first)

```
c24e72e M7.7: realtime plumbing for page content (server push → local editor)
7fedccf M7.6: kill the artifact dual-write — remove dead direct createArtifact
7703a73 M7.5: frontend page-repo invokes Tauri instead of fetching the Go API
a332afb M7.4: ArtifactCreateApiInput now sends blocks for page artifacts
b55a718 M7.3: Rust sync engine for page content (api/pages.rs + outbox dispatch)
069a024 M7.2: Tauri commands for page content (local-first read + save)
b8590d4 M7.1: SQLite schema V8 — page_content table
```

`git diff m5-m6-blocknote-jsonb..m7-local-first-pages` is the full M7 delta.
`git diff main..m7-local-first-pages` shows M5+M6+M7 together.
