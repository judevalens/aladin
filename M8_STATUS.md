# M8 Status — Collaborative page editing (Yjs + Hocuspocus)

Branches: `m8a-collab-foundation` (foundation) → `m8b-collab-online` (collab).
Plan: `~/.claude/plans/yjs-collab-pages.md`.

**M8a + M8b + M8c are code-complete and tested.** M8b's browser behavior is **verified** — you confirmed live editing + persistence across a full server+client restart; collab broadcast + the projection are now regression-tested in `test/collab.test.js`. **M8c (MCP collab bridge) shipped**: agent edits route through the live Y.Doc, broadcast to open editors, and project to `page_documents`. M8d (cleanup) is not started.

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

### M8c — MCP collab bridge (branch `m8b-collab-online`)
| | |
|---|---|
| M8c.1/.2 | `services/blocknote` admin bridge: `POST /admin/apply` (block op → live Y.Doc via `server.hocuspocus.openDirectConnection` + `connection.transact`) and `GET /admin/page/:id` (fresh materialized read). Shared-secret guarded (`X-Admin-Secret`, default `local-dev-admin-secret`, fail-closed). Ops: replace_all / replace_block / insert_blocks / delete_block. |
| M8c.3 | M8.7 JSON projection: debounced `onChange` (≤1/500ms AND ≤1/50 commits, + 5s sweep for restart-orphaned timers) materializes Y.Doc → `page_documents.blocks` + `search_text` + bumped `revision`. Fire-and-forget — a projection failure never blocks the WS loop or crashes the broker. |
| M8c.4 | Seam guards in `ArtifactService`: Create **ignores** page `blocks` (empty doc; content arrives via the editor or a bridge replace_all); Update **refuses** page `blocks`/`content` (BadRequest). Closes the M5/M6 direct-write leak that would have clobbered the Y.Doc. |
| M8c.5/.6 | Go: `blocknote.Client` gains `ApplyOperation`/`GetPage` (same base URL, admin-secret header) behind a `Bridge` interface. MCP write tools (`update_block`/`insert_blocks`/`delete_block`/`update_page`) route through the bridge; `create_page` = Create-empty + bridge replace_all; `get_page` reads **fresh** via the bridge; `list_pages`/`search_pages` still read the projection. Write responses carry the affected blocks' markdown (read-after-own-write fix). |
| tests | `test/collab.test.js` — new bridge-op test: a server-side op broadcasts to two live clients + the projection lands (passes, COLLAB_IT). Go unit tests updated (`tools_test.go`, `artifacts_test.go`); `integration_test.go` rewired for the bridge (manual, `-tags integration` — **TRUNCATEs, don't run against live data**). Full `go test ./...`, hermetic `npm test`, and the version-drift gate are green. |

**Verified end-to-end (HTTP probe + collab test):** replace_all → 2 blocks; insert_blocks keeps prior block ids stable; both live clients observe a server-side bridge op; `page_documents` projection reflects edits with correct `search_text`.

**Bridge limitations (accepted, documented):**
- **Coarse ops** — server-util has no fine-grained "apply one op preserving history" call; every op is a full-fragment rebuild. Untouched block **ids survive** (verified), but a concurrent human edit landing *during* an agent write can be dropped wholesale — it's a whole-document clobber, not a block-level merge. Fine for low-frequency agent edits, single instance.
- **No agent cursor** — `DirectConnection` exposes only `transact`/`disconnect` (no awareness API), so the agent gets no presence cursor. Deferred.
- **`get_page` hard-depends on the sidecar** (reads the live Y.Doc; no projection fallback).

### Adversarial review (M8c.9)

Two independent adversarial agents reviewed M8c (architecture + clean-code). **Fixed:**
- **Seam leak closed**: `PATCH /api/pages` (`PageService.Save`) wrote blocks directly, bypassing the M8c guard — it's now refused (the path is dormant; usePageState is orphaned; fully removed in M8d).
- `insert_blocks` now **rejects** an ambiguous position (after_id + before_id) and an unknown `at` value instead of silently appending in the wrong place.
- `op` is validated **before** opening a Y.Doc connection (no needless load/teardown on a bad op).
- Removed dead `revision` fields from the block tool inputs/outputs — they did nothing (CRDT has no optimistic-concurrency CAS); leaving them misled agents.
- Projection knobs are env-configurable: `PROJECTION_DEBOUNCE_MS` / `PROJECTION_MAX_COMMITS` / `PROJECTION_SWEEP_MS`.
- Loud startup **warning** when the admin bridge runs on the default shared secret.
- New hermetic unit tests: `withFirstBlockID` + `applyInsertPosition` (Go), the admin-secret middleware (Node).

**Deferred / accepted (with rationale):**
- The admin bridge defaults to a known shared secret on `0.0.0.0:3500` — fine for single-machine localhost, but **set `BLOCKNOTE_ADMIN_SHARED_SECRET` before any reachable deployment** (now warned at startup). A loopback-only bind was rejected because the Docker topology reaches the sidecar over the published port.
- `blocksToSearchText` doesn't read table-cell text → table content is invisible to `search_pages`. Follow-up if pages start using tables.
- `toolServer.pages` / the whole `PageDocumentService` (~215 lines) + the M7 page_content path are orphaned-but-present — removed in M8d.
- A bare `get_page` on a never-edited page materializes an empty `page_ydoc` row (benign — the artifact exists, so it's just the empty doc the first edit would have created).
- Both reviewers' verdict: the core mechanism is sound (persistence is durable before `applyOperation` returns; failure isolation is good; CRDT guarantees hold) — safe to dogfood on a single trusted local machine.

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

- **You**: end-to-end MCP smoke — with the sidecar (`make blocknote`) + API (`make backend`) up, point Claude Code/Codex at the MCP server, create/edit a page via the tools, and watch it appear live in an open editor. (Optional: the `-tags integration` E2E — but it TRUNCATEs page data.)
- **Then M8d**: delete the M7 content path after a dogfood window — `page-repo.savePage`, `usePageState` drafts, Rust `page_content`, the `PATCH /api/pages` handler, and the now-dead `toolServer.pages` / `PageDocumentService` wiring.
- **Later (M12)**: `artifacts.content` → per-type canonical content tables (generalize the pages pattern; see `~/.claude/plans/yjs-collab-pages.md`).
