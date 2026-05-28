# M8 Status — Collaborative page editing (Yjs + Hocuspocus)

Branches: `m8a-collab-foundation` (foundation) → `m8b-collab-online` (collab).
Plan: `~/.claude/plans/yjs-collab-pages.md`.

**M8a + M8b are code-complete.** M8b's browser behavior is **not yet verified** — that part needs you (collab is inherently a two-client, in-browser thing). M8c (MCP bridge) and M8d (cleanup) are not started.

## What shipped

### M8a — Foundation (branch `m8a-collab-foundation`)
| | |
|---|---|
| M8a.1 | Migration 00025: `page_ydoc(page_id, state BYTEA, updated_at)` + `page_documents.last_collab_commit_at`. Wipes page data (test workspace). |
| M8a.2 | `services/blocknote-converter` → `services/blocknote`, restructured into `src/{handlers,services,middleware}` with an error boundary (handler errors → 500, never crash; process-level uncaught → exit + Docker restart). Converter parity kept. |
| M8a.3 | docker-compose service/container renamed (`aladin-blocknote`); Makefile `blocknote` / `blocknote-test` / `check-blocknote-versions` / `nuke-local-db` targets. Go client unchanged. |
| M8a.4 | `scripts/check-blocknote-versions.sh` — fails on @blocknote/yjs/hocuspocus version drift between aladin_react and services/blocknote (compares lockfile-resolved versions). |
| (side) | `make nuke-local-db` — wipes the Tauri local SQLite when a schema change breaks the local cache. |

### M8b — Collab online (branch `m8b-collab-online`)
| | |
|---|---|
| M8b.9 | `GET /api/auth/resolve` — bearer/session → principal JSON. Hocuspocus auth hook calls it (keeps Hocuspocus decoupled from the auth schema). Go-tested. |
| M8b.1 | Hocuspocus mounted in `services/blocknote`, persisting full Y.Docs to `page_ydoc` via stock `@hocuspocus/extension-database` (debounced 2s/10s). Auth via `/api/auth/resolve`. |
| collab test | `test/collab.test.js` (COLLAB_IT-gated): two clients converge + state persists to `page_ydoc`. Passes 241ms against live Postgres. |
| M8b.3/.4/.5 | Frontend editor → Y.Doc collab: HocuspocusProvider + y-indexeddb (local-first) + awareness (cursors). M7 load/save bypassed for content. |

## The crossws finding (corrects the M8b.1 commit message)

The M8b.1 commit message blames a "ws@7 vs ws@8 version mismatch." **That's wrong** — here's the accurate account:

Hocuspocus 4.x doesn't use the `ws` library directly; it depends on **`crossws`** (a cross-runtime WebSocket abstraction). Its `handleConnection(socket, request)` expects a **crossws peer**, not a raw `ws` socket. My first attempt hand-rolled a `ws` `WebSocketServer` + `handleConnection(rawWsSocket)` — the socket wasn't a crossws peer, so Hocuspocus never drove the message loop (`onAuthenticate` never fired, no sync). The ws@7 I briefly saw came from `express-ws` and vanished when I removed it.

The fix is the **idiomatic** Hocuspocus 4.x mount: use the `@hocuspocus/server` `Server` wrapper, which owns its own HTTP+WS server (via crossws) on its own port. Not a hack — the recommended integration.

## Architecture: one process, two ports

- **:3500** — Express converter (md↔blocks HTTP). Unchanged from M6.
- **:3501** — Hocuspocus collab (WebSocket). Its own server.

They do **not** talk to each other today. They share a process only to set up M8c: the MCP admin bridge will be an HTTP endpoint that calls `collab.hocuspocus.openDirectConnection(pageId)` in-process to apply agent edits to the live Y.Doc. (Noted scalability limit: `openDirectConnection` is single-instance; horizontal scale needs `@hocuspocus/extension-redis` or a WS-client bridge. Deferred — Aladin is single-instance. Bridge stays in-process for now per decision.)

## Durability & consistency (recap)

- CRDT = convergence, not consensus. Every actor edits its own view; Yjs merges deterministically. No corruption; semantic staleness is possible (same as any collaborator).
- Durability lives on the **clients** (Y.Doc + y-indexeddb), not the server debounce. The 2s/10s `store` debounce is write-efficiency; clients re-push gaps on reconnect.
- `page_documents.blocks` is a JSON **projection** (search / list / cold MCP reads), refreshed from the Y.Doc — that's M8.7 (in M8c), not built yet. So search/list won't reflect collab edits until M8c lands.

## What needs your eyes (browser — I can't headless this)

Run everything, then:
1. **Two-tab collab**: open the same page in two windows, type in one, see it in the other live.
2. **Awareness**: see the other window's cursor + your email label.
3. **Offline**: kill `services/blocknote`, edit, reload the app → edit survives (y-indexeddb); restart the service → it syncs up.
4. **Auth**: confirm the provider authenticates — the Tauri app's token (desktop session) must reach Hocuspocus's `onAuthenticate` → `/api/auth/resolve`. If pages don't sync, check the token is non-empty and the Go API is up.

## How to run

```
docker compose up -d postgres blocknote        # converter :3500 + collab :3501
make backend                                    # Go API :8000 (serves /api/auth/resolve)
cd aladin_react && npm run tauri dev            # desktop app
# verify: go test ./..., npm test, COLLAB_IT=1 node --test --test-force-exit services/blocknote/test/collab.test.js
```

## Known caveats / gaps

- **React strict-mode (dev)** may double-create the Y.Doc/provider (useState initializer side effect) → a double connection in dev only. Harmless; if it bothers, guard creation with a ref or disable strict mode for the editor subtree.
- **Web (non-desktop) collab auth** is a gap: the provider token comes from the desktop session store; web uses cookies that won't cross to :3501. Desktop is the target; web collab auth is a follow-up.
- **No JSON projection yet** (M8.7 is M8c). `get_page`/`search_pages` read stale `page_documents.blocks` until then. The MCP write tools still use the M7 path until M8c routes them through the bridge.
- **M7 content path still present** (page-repo.savePage, usePageState drafts, page session services). Bypassed by the editor but not deleted — that's M8d, after a dogfood window.
- **Commit message inaccuracy**: M8b.1's message says "ws version mismatch"; the real cause is crossws (above). Left as-is; this doc is the correct record.

## Git history

```
# m8b-collab-online (on top of m8a-collab-foundation)
b4fbe7f M8b.3/.4/.5: frontend editor → collaborative Y.Doc (Hocuspocus + IndexedDB)
52c53b1 M8b.1: mount Hocuspocus collab in services/blocknote (+ convergence test)
9891157 M8b.9: GET /api/auth/resolve — token → principal for Hocuspocus
# m8a-collab-foundation (on top of m7-local-first-pages)
30a3232 add `make nuke-local-db`
471e93a M8a.4: Yjs/BlockNote version-drift CI gate
1980ab9 M8a.3: rename docker-compose service/container; add Makefile targets
61616d2 M8a.2: rename services/blocknote-converter → services/blocknote (layered)
8a7b707 M8a.1: migration 00025 — page_ydoc storage + projection column
```

## Next

- **You**: browser-verify M8b (checklist above).
- **Then M8c**: MCP bridge (`/admin/apply` → `openDirectConnection`), route MCP write tools through it, the `ArtifactService.Update` page-blocks guard, and the M8.7 JSON projection.
- **Then M8d**: delete the M7 content path after a dogfood window.
