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
- **Copilot agent sidecar** — `services/copilot-agent/` — Node; runs the Claude
  Agent SDK per copilot turn (needs `ANTHROPIC_API_KEY`), consuming tools from the
  Go MCP server (:8090) with the caller's bearer and streaming NDJSON back to the
  Go API, which republishes `copilot.*` events to the dock.

## Layout

```
aladin_react/
  src/components/ui/      shadcn-style primitives (button, dialog, card, …)
  src/modules/<feature>/  feature code; <feature>/ui/ holds that feature's UI
  src/app/state/          Zustand slices (session, workspace)
  src/shared/             API types, realtime, shared libs
  src/lib/utils.ts        the cn() helper used by components/ui  ← import from "@/lib/utils"
  src/index.css           design tokens (Tailwind v4; Aladin Dark/Soft via data-theme)
backend_v2/
  cmd/{api,worker,mcp}    entrypoints (api :8000, mcp :8090)
  internal/               api → service → repo layering; db/migrations/*.sql (goose)
services/blocknote/       collab sidecar (converter :3500, collab :3501)
services/copilot-agent/   copilot agent sidecar (Claude Agent SDK, :3550)
design/                   UI_ARCHITECTURE.md (frontend onboarding map: shell, tokens,
                          conventions, traps) + TRADING_PRD.md (north star) + screens/
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

The app ships a **dark-minimal IDE** look in two themes — **Dark** (default) + **Soft** —
driven by `data-theme` on `<html>` (see `src/app/state/theme-slice.ts`). Tokens live in
`aladin_react/src/index.css` (Tailwind v4 inline `@theme`). **Never hardcode hex/rgb in
components** — use the Aladin tokens:
- surfaces: `bg-rail`/`bg-panel`/`bg-bg`/`bg-chrome`/`bg-field`/`bg-card`/`bg-raise`/`bg-explorer`
- ink ramp: `text-ink` / `text-ink-2` / `text-ink-3` / `text-ink-4`
- accent + lines: `bg-amber`, `bg-amber-soft`, `border-amber-line`, `border-line`, `border-line-2`
- semantic hues: `text-for` (supports), `text-against` (counters), `text-catalyst`, `text-echo`
- fonts: `font-display` (Space Grotesk) · `font-mono` (JetBrains Mono) · `font-sans` (system)
- radii `rounded-chip/card/modal`; shadows `shadow-panel/modal/toast`
shadcn's own tokens (`bg-background`, `text-foreground`, `border-border`, …) are mapped
onto these, so restyled primitives inherit the theme.

**Before any UI work read `design/UI_ARCHITECTURE.md`** — the current frontend map: the
shell layout, the `artifact.kind` switch that renders surfaces, the auth-free `/spike/*`
routes for iterating without a login, the token rules, the drill rule, and the traps that
have actually bitten. `design/screens/` holds reference renders; `design/TRADING_PRD.md` is
the product north star. (`DESIGN_SPEC.md`/`BROWSER_SPEC.md` were removed — a Tailwind v3
config and a shadcn map that no longer match the app; what was still true moved into
UI_ARCHITECTURE.md.)

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
