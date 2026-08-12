# UI Architecture — onboarding map for the frontend

> **Read this before changing any UI.** It describes what the app *is today*, verified against
> the code, not what an older design package proposed. Written for an agent arriving cold —
> especially one doing design/UI work.
>
> Companions: `DESIGN_SPEC.md` (tokens, component map, visual spec) · `BROWSER_SPEC.md`
> (the Folders browser, pixel-precise) · `screens/` (reference renders) · `../CLAUDE.md`
> (commands, conventions, backend). **Do not start from `README.md`** — see §9.

---

## 1. What this app is

A **desktop research workspace** for one user (a technically strong engineer, novice trader),
shipped as a **Tauri v2** app wrapping a Vite + React + TypeScript frontend. The visual
register is a **dark-minimal IDE**: dense, quiet, keyboard-friendly, no marketing surface.

It is **not** a knowledge-graph product. That was the original framing and it was abandoned;
the current north star is `TRADING_PRD.md` (a personal algo/swing-trading research workspace).
Some UI still reflects the older framing — see §8.

Two hosts, and the difference matters when you test:

| host | how | what works |
|---|---|---|
| **Tauri desktop app** | `npm run tauri:dev`, or `make prod-app` for a release build | everything — local SQLite, `invoke` commands, collab |
| **plain browser** | `npm run dev` → `http://localhost:4173` | UI only. Tauri APIs throw (`transformCallback` errors are expected), `/api` proxies to `localhost:8000` |

## 2. How to actually see the UI (start here)

The authenticated shell needs a login and a running backend. **For UI work you usually don't
need either** — there are auth-free `/spike/*` routes that mount surfaces on mock data:

```bash
cd aladin_react && npm install && npm run dev     # http://localhost:4173
```

| route | surface |
|---|---|
| `/spike/markets` | the trading surface on placeholder data |
| `/spike/editor` | the BlockNote page editor, no collab |
| `/spike/entity-context`, `/spike/entities-index`, `/spike/entities-inbox`, `/spike/entities-home` | the entity surfaces |
| `/spike/tutor`, `-read`, `-notebook`, `-kinds`, `-purpose` | five explorations of the Tutor surface (design record — see `TUTOR_PRD.md`) |
| `/spike/sandbox` | the Shard (Doc Surface) sandbox |

**This is the intended way to iterate on visuals.** A spike is a real component tree with
mock data, standalone and no-auth, registered in `../aladin_react/src/app/router.tsx`. Adding
one is cheap and is the normal way to propose a new surface here.

Console noise to ignore in the browser: `Cannot read properties of undefined (reading
'transformCallback')` (Tauri host absent) and `/api` 502s (no dev backend). Neither indicates
a UI fault.

## 3. The shell

One shell wraps every authenticated surface —
`../aladin_react/src/modules/workspace/ui/workspace-shell-ui.tsx`:

```
┌──────┬────────────────────────┬──────────────────────────────┬──────────────┐
│ rail │  browser pane          │  work pane                   │ copilot dock │
│ 7    │  336px (sm: 368px)     │  flex-1, tabbed              │  (right)     │
│ icons│  the tree / a surface  │  switches on artifact.kind   │              │
│      │                        ├──────────────────────────────┤              │
│      │                        │  terminal dock (bottom)      │              │
└──────┴────────────────────────┴──────────────────────────────┴──────────────┘
```

- **Rail** — 7 destinations, defined as a literal array at the top of the shell file:
  Home · Markets · Folders · Insights · Entities · Sources · Graph.
- **Browser pane** — `browser-pane-ui.tsx`. The folder/artifact tree. `BROWSER_SPEC.md` is
  the precise spec, including the drill rule (3 levels, then an 864×460 Miller-column popup —
  `miller-columns.tsx`).
- **Work pane** — `work-pane-ui.tsx`. Tab strip + breadcrumb, and **a switch on
  `activeArtifact.kind`** that picks the viewer. This is the extension point: a new artifact
  kind is a new branch here, not a new route.
- **Copilot dock** — `modules/copilot/ui/copilot-dock-ui.tsx`. The agent lives here, always.
  **Do not build a second chat surface**; that mistake is documented in the Tutor spikes.
- **Terminal dock** — collapsible, bottom of the work pane.

### The kind switch (how a thing gets rendered)

`work-pane-ui.tsx` maps kind → viewer:

| `artifact.kind` | viewer |
|---|---|
| `note` | `PageEditorUI` (BlockNote + Yjs collab) |
| `app` | `DocSurfaceKeepAlive` — a **Shard**: agent-authored React in a sandboxed iframe |
| `file` | `FileArtifactPaneUI` → the document viewer for ingested PDFs (pages, outline), download card otherwise |
| `link`, `voice` | their own artifact panes |
| *(tab kind)* `research` | `ResearchPaneUI` — a synthetic tab, not an artifact |

## 4. Where code lives

```
src/
  app/
    router.tsx           routes, including every /spike/*
    state/               9 Zustand slices + store.ts (workspace, copilot, market, theme, …)
    composition/         DI: repos + services constructed once, provided by context
  components/ui/         PRIMITIVES ONLY — 15 of them (button, dialog, popover, tooltip, …)
  modules/<feature>/
    ui/                  that feature's components
    hooks/               that feature's hooks
  shared/                api types, realtime, flow helpers (Loadable/observables)
  repos/                 data access (Tauri `invoke` or HTTP)
  index.css              DESIGN TOKENS (Tailwind v4 inline @theme)
```

18 modules: `artifacts auth copilot doc-surface documents entities graph insights markets
notifications pages pipeline research sources terminal tickers tutor workspace`.

**Primitives** are built on **Base UI** (`@base-ui/react`) + **class-variance-authority**,
restyled to Aladin tokens. Read `components/ui/button.tsx` for the pattern. This is *not*
stock shadcn — never ship the default shadcn look.

## 5. Styling rules (a change violating these will be rejected)

1. **Never hardcode a colour.** No hex, no `rgb()`. Use the tokens in `src/index.css`:
   - surfaces `bg-rail bg-panel bg-bg bg-chrome bg-field bg-card bg-raise bg-explorer`
   - ink ramp `text-ink text-ink-2 text-ink-3 text-ink-4`
   - accent + lines `bg-amber bg-amber-soft border-amber-line border-line border-line-2`
   - semantic `text-for` (supports) `text-against` (counters) `text-catalyst` `text-echo`
   - fonts `font-display` (Space Grotesk) `font-mono` (JetBrains Mono) `font-sans`
   - radii `rounded-chip/card/modal`; shadows `shadow-panel/modal/toast`
2. **Two themes**, Dark (default) and Soft, driven by `data-theme` on `<html>`
   (`app/state/theme-slice.ts`). Tokens handle it — if you use tokens you get both for free.
3. **Compose classes with `cn()` from `@/lib/utils`.** Note there is a duplicate `cn()` at
   `shared/lib/utils.ts`; prefer `@/lib/utils` and don't add a third.
4. **Amber is the only accent — spend it.** It marks the one thing that needs attention on a
   surface. Two amber elements competing means neither reads.
5. **Don't invent metrics.** No progress rings, mastery scores, or zeroed stat tiles for data
   that doesn't exist. An empty state names what belongs there in one sentence. Silence is the
   correct rendering of "fine". (`RESEARCH_SURFACE_PRD.md` §2 is explicit.)

## 6. Component conventions

- **No "screen"/container components.** A feature's UI calls its own hook directly. Introduce
  a container only when state is genuinely lifted across siblings.
- **Feature UI** goes in `modules/<feature>/ui/`; only **primitives** go in `components/ui/`.
- **For observable/RxJS bindings prefer `useSyncExternalStore`** over `useState`+`useEffect`,
  and seed the snapshot with `loading()` rather than switching primitives. The helper is
  `shared/flow/use-observable-state.ts` (`useObservableState`).

## 7. Data flow — the rule that matters most

**Reactivity rides the sync spine, not invalidation.** The server owns state; changes arrive
as outbox frames → `data_event` → `DataEvents` → the local store → `useSyncExternalStore`.

**Forbidden**, and this is a standing rule rather than a preference:

- REST-list + `reload()` refetching after a mutation
- optimistic local patching
- an "invalidation" Subject that components subscribe to in order to re-fetch

If a surface needs to stay live, it subscribes to the synced local model. Copy the existing
`signal`/tree paths. The client sync engine is in **Rust** (`src-tauri/src/sync/`), and the
local SQLite is a **rebuildable replica** — it self-heals (`Db::open_or_recover`), so never
treat it as authoritative.

## 8. Traps that have actually bitten

- **`useSyncExternalStore` requires a *cached* `getSnapshot` value, not merely a pure
  function.** `snapshot: () => []` returns a fresh array each call, never `Object.is`-equal,
  so React loops forever → *"Maximum update depth exceeded"* (React #185). This crashed the
  property-filter dialog on open. Regression test:
  `src/test/property-filter-store.test.tsx`. Hoist constants to module scope.
- **`/home` and `/folders` render the same component.** The Home briefing dashboard the old
  PRD describes does not exist. Don't assume a route's name reflects a distinct surface.
- **`/graph` is a placeholder.** The rail entry exists; the destination says so.
- **`modules/pipeline` is not mounted anywhere.** A complete vertical slice, imported by
  nothing. Don't take its presence as evidence it's live.
- **Tests live in `src/test/`**, not colocated — the vitest `include` is
  `src/test/**/*.test.ts(x)`. A colocated test file simply never runs.
- **The desktop app is a separate build from the backend release.** Frontend changes reach
  prod only via `make prod-app`; `make prod-update` ships the backend. See `../PROD.md`.

## 9. Which docs to trust

| doc | status |
|---|---|
| **this file** | current, verified against code |
| `DESIGN_SPEC.md` | current for tokens/components/visual spec |
| `BROWSER_SPEC.md` | current for the Folders browser |
| `TRADING_PRD.md` | **the north star** — read for product intent |
| `RESEARCH_SURFACE_PRD.md` | LOCKED; the research folder pattern, and the best statement of the surface register |
| `INGESTION_PRD.md`, `SHARD_MODEL.md` | current for those subsystems |
| `TUTOR_PRD.md` | draft rev 3, design record; **nothing built** |
| `README.md` | **HISTORICAL.** The original handoff package. Opens "Aladin is a personal knowledge graph", describes a `claims` layer that has since been deleted, and calls graph persistence "the #1 priority; it's the moat" — all superseded by the trading pivot. Useful only as an archive. |
| `PRD.md`, `LEARN_PRD.md`, `CURATION_UX_PRD.md`, `PORTFOLIO_PRD.md` | aspirational or superseded; referenced by no code |

## 10. Checklist before you finish a UI change

```bash
cd aladin_react
node_modules/.bin/tsc --noEmit -p tsconfig.app.json   # NOT bare `npx tsc`
npm test
```

Then **look at it** — `npm run dev` and open the surface (a `/spike/*` route if it needs no
auth), in both themes. Grep your diff for `#` hex literals and `rgb(`. If you added a
component, confirm it's in the right place: primitive → `components/ui/`, everything else →
`modules/<feature>/ui/`.
