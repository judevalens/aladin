# CLAUDE.md — Aladin

Working notes for Claude. Read this first; it's the map of the repo, the commands
that actually work, and the conventions to honor. Keep it current as the stack moves.

## What Aladin is

A desktop research workspace. **Dual-store architecture:**

- **Frontend / desktop** — `aladin_react/` — Vite + React + TypeScript, wrapped in
  **Tauri v2** (Rust). Local **SQLite** holds the browser tree, artifact metadata, and
  legacy page content.
- **Backend** — `backend_v2/` — **Go 1.25** + **Postgres** (canonical store) + Redis
  (Asynq queue) + Neo4j (graph). Migrations are embedded **goose**, applied on boot.
- **Collab sidecar** — `services/blocknote/` — Node; Yjs/Hocuspocus realtime page
  editing. Page content lives in Postgres `page_ydoc` + webview IndexedDB.
- Legacy Kotlin/Compose UI lives in `aladin_ui/` — **not** the active surface; ignore
  unless explicitly asked.

## Layout

```
aladin_react/
  src/components/ui/      shadcn-style primitives (button, dialog, card, …)
  src/modules/<feature>/  feature code; <feature>/ui/ holds that feature's UI
  src/app/state/          Zustand slices (session, workspace)
  src/shared/             API types, realtime, shared libs
  src/lib/utils.ts        the cn() helper used by components/ui  ← import from "@/lib/utils"
  src/index.css           design tokens (Tailwind v4 + shadcn, OKLch)
backend_v2/
  cmd/{api,worker,mcp}    entrypoints (api :8000, mcp :8090)
  internal/               api → service → repo layering; db/migrations/*.sql (goose)
services/blocknote/       collab sidecar (converter :3500, collab :3501)
design/                   ui-design-spec.md (intent) + OVERHAUL.md (north-star)
```

> Note: a duplicate `cn()` exists at `src/shared/lib/utils.ts`. Components/ui use
> `@/lib/utils` — prefer that one; don't add a third.

## Run (use the SANDBOX stack — never the dev infra)

All automated testing/verification runs against the **isolated sandbox** stack
(`docker-compose.test.yml`, project `aladin-test`), so the real `aladin-*` dev
containers and their data are never touched until we deliberately migrate.

```bash
make test-db-up           # sandbox: pg :5444, neo4j :7475/:7688, redis :6380
make test-db-down         # stop it (ARGS=-v also drops sandbox volumes)

# Frontend dev server (proxies /api -> :8000):
cd aladin_react && npm install && npm run dev      # http://localhost:4173
npm run tauri:dev                                  # full desktop app
```

The real dev stack (`make db-up`, `make backend`) exists but is the user's; don't
start/stop it for routine verification — use the sandbox.

## Test / typecheck / lint

```bash
# Go — against the sandbox DB (auto-migrates; refuses the dev DB):
make test-go              # = go test ./... with TEST_DATABASE_URL -> :5444
cd backend_v2 && go vet ./...

# Frontend:
cd aladin_react && npm test          # vitest (unit)
npm run e2e                          # playwright (needs dev server on :4173)
node_modules/.bin/tsc --noEmit -p tsconfig.app.json   # typecheck
# (run `npm install` first in a fresh worktree; do NOT use bare `npx tsc` — it
#  resolves to an unrelated package. Use the local bin or `npx --no-install tsc`.)

# Collab sidecar + version-drift guard:
make blocknote-test
make check-blocknote-versions        # @blocknote/* + yjs must not drift across packages

# Tauri/Rust:
cd aladin_react/src-tauri && cargo clippy && cargo fmt
```

Go integration tests are behind `-tags=integration` and need the sidecar up; plain
`make test-go` runs the unit + DB suites.

## Design tokens — the rule that matters

The app is styled with **Tailwind v4 + shadcn**, tokens defined in
`aladin_react/src/index.css` using **OKLch** with light (`:root`) + dark (`.dark`)
variants. **Never hardcode hex/rgb/oklch in components** — use the semantic tokens:
`bg-background`, `text-foreground`, `bg-card`, `bg-muted`, `text-muted-foreground`,
`bg-primary`, `border-border`, `ring-ring`, `bg-destructive`, the `sidebar-*` family,
and `--radius` (`rounded-md/lg/xl`, …).

Design *intent* lives in `design/ui-design-spec.md` (written in an older `AladinColor`
vocabulary). `design/OVERHAUL.md` bridges that vocabulary to the live tokens — consult
it before design work.

## Component conventions

- **Primitives** go in `src/components/ui/`; build on **Base UI** (`@base-ui/react`)
  + **class-variance-authority** (`cva`) for variants. See `button.tsx` for the pattern.
- **Feature UI** goes in `src/modules/<feature>/ui/`.
- Compose classes with `cn()` from `@/lib/utils`.
- **No "screen"/container components** — a feature's UI calls its own hook directly;
  only introduce a container when state is genuinely lifted across siblings.
- For RxJS/observable bindings prefer **`useSyncExternalStore`** over
  `useState`+`useEffect`; seed the snapshot with `loading()` rather than switching
  primitives.

## Backend conventions (Go)

- **Clean layering:** `api → service → repo`. Service/logic classes sit behind an
  interface (concrete impl unexported); DI exposes **interfaces**, not concrete structs
  (e.g. `Sync()` returns `service.SyncService`, not `*repo.SyncRepo`). DTOs stay plain
  structs.
- Destructive DB tests use `dbtest.RequireTestDSN` and refuse to run unless
  `TEST_DATABASE_URL` is set and differs from `DATABASE_URL`.

## Working style here

- When a design doc is **LOCKED**, implement it exactly — no unrequested
  "improvements" or shortcuts; raise concerns as questions, stay scoped to the task.
- Per-feature implementation plans live in `~/.claude/plans/`.
- Reusable orchestration workflows live in `.claude/workflows/` (opt-in; run only
  when explicitly invoked).
