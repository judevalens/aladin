# UI Architecture — onboarding map for the frontend

> **Read this before changing any UI.** It describes what the app *is today*, verified against
> the code, not what an older design package proposed. Written for an agent arriving cold —
> especially one doing design/UI work.
>
> Companions: `screens/` (reference renders of the original static views — Home, Folders,
> Signals, Sources, Graph; useful for the visual register, but they predate the trading pivot
> so treat them as mood, not spec) · `../CLAUDE.md` (commands, conventions, backend) ·
> `TRADING_PRD.md` (product intent).

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
- **Browser pane** — `browser-pane-ui.tsx`. The folder/artifact tree, governed by **the
  drill rule** below.
- **Miller popup** — `miller-columns.tsx`, `460` tall by up to `864` wide (`COLUMN_WIDTH * 4`
  — exactly four columns). Anchored to the clicked row's rect and clamped to the viewport.
- **Work pane** — `work-pane-ui.tsx`. Tab strip + breadcrumb, and **a switch on
  `activeArtifact.kind`** that picks the viewer. This is the extension point: a new artifact
  kind is a new branch here, not a new route.
- **Copilot dock** — `modules/copilot/ui/copilot-dock-ui.tsx`. The agent lives here, always.
  **Do not build a second chat surface**; that mistake is documented in the Tutor spikes.
- **Terminal dock** — collapsible, bottom of the work pane.

### The drill rule (agents get this wrong)

`MAX_INLINE = 2` in `browser-pane-ui.tsx`. A folder expands **inline** only while
`depth < 2`, so the tree shows three visible levels (0, 1, 2). A folder at `depth >= 2` does
**not** expand — clicking it **drills**, opening the Miller popup seeded with that folder's
ancestor path. This is what stops indentation marching off the left edge.

```
onClick(node):
  folder && depth >= MAX_INLINE  → openMiller(node.id, rowRect)   // drill
  folder                         → toggle(node.id) in openSet     // expand inline
  leaf                           → open it as a tab
```

Leaves always open in the work pane; only inline folders toggle. If you change the tree,
preserve this — flattening it to "expand everything inline" is the obvious-looking change and
it breaks at depth 4.

### The tab strip grouping invariant

Tabs belonging to one research folder must stay **contiguous**. Membership is *derived* from
the tab's `contextId` — never stored, never managed — so grouping is purely an ordering
concern, computed in `use-workspace-state.ts` (~line 383): research tabs cluster by
`contextId` in first-seen order, loose artifact tabs sort after all groups, and the last tab
of a group gets a heavier right divider (`border-r-2 border-r-line-2`). There is no per-group
colour — this app has one accent.

The strip is a **status display, not a navigation surface**: `scrollbar-hidden` is deliberate,
and there is intentionally no overflow chevron. Anything that needs to *navigate* to a tab
belongs in a separate surface.

The full rules live in `RESEARCH_SURFACE_PRD.md` **§12** — the `§12` comments in
`work-pane-ui.tsx` and `use-workspace-state.ts` point there, *not* at a section of this file.

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
  theme.css              DESIGN TOKENS — the @theme inline block. THIS is the token file.
  index.css              the 7 non-default [data-theme] blocks + base layer
```

18 modules: `artifacts auth copilot doc-surface documents entities graph insights markets
notifications pages pipeline research sources terminal tickers tutor workspace`.

**Primitives** are built on **Base UI** (`@base-ui/react`) + **class-variance-authority**,
restyled to Aladin tokens. Read `components/ui/button.tsx` for the pattern. This is *not*
stock shadcn — never ship the default shadcn look.

## 5. Styling rules (a change violating these will be rejected)

1. **Never hardcode a colour.** No hex, no `rgb()`. Use the tokens in **`src/theme.css`**
   (the `@theme inline` block; `src/index.css` holds the per-theme overrides):
   - surfaces `bg-rail bg-panel bg-bg bg-chrome bg-field bg-card bg-raise bg-explorer`
   - ink ramp `text-ink text-ink-2 text-ink-3 text-ink-4`
   - accent + lines `bg-amber bg-amber-soft border-amber-line border-line border-line-2`
   - semantic `text-for` (supports) `text-against` (counters) `text-catalyst` `text-echo`
   - fonts `font-display` (Space Grotesk) `font-mono` (JetBrains Mono) `font-sans`
   - radii `rounded-chip/card/modal`; shadows `shadow-panel/modal/toast`
2. **Eight themes**, not two: `dark` (default) · `soft` · `cool` · `contrast` · `linear` ·
   `apple-dark` · `apple-light` · `light`. Driven by `data-theme` on `<html>`
   (`app/state/theme-slice.ts`); `dark` lives in `theme.css`, the other seven override in
   `index.css`. **One component tree**, never two builds and never a per-theme branch.
   Verify in `light` and `linear` before merge — those two flip the accent *and* its
   foreground, so they catch hardcoded on-accent text that `dark` hides.
3. **Compose classes with `cn()` from `@/lib/utils`** — the only one. It used to be
   duplicated in `shared/lib/utils.ts` (38 importers vs 12); that copy is gone, and the file
   now holds only the date/string formatters. Don't add a second.
4. **Amber is the only accent — spend it.** It marks the one thing that needs attention on a
   surface. Two amber elements competing means neither reads.
5. **Avoid the clichés this app deliberately rejects**: no gradients, no emoji, no
   rounded-corner-plus-left-accent card. Calm and minimal.
6. **`whitespace-nowrap shrink-0` on header meta rows.** Leaving whitespace at its default
   wrapped them; this bit once already.
7. **Never hardcode a size.** Type is one of six steps — `text-meta` (10) `text-small` (12)
   `text-body` (13) `text-lead` (15) `text-title` (22) `text-display` (30). Line-height rides
   with the step. An arbitrary `text-[13.5px]` is a bug, not a preference.
8. **Radius follows container size, not taste** — `rounded-tap` (5, hit boxes ≤20px) ·
   `rounded-chip` (7) · `rounded-control` (9, buttons/rail cells/inputs/tree rows) ·
   `rounded-card` (12) · `rounded-modal` (14).
9. **Icons go through `<Icon>`** (`components/ui/icon.tsx`): `size="inline|default|rail"` and
   `mark` for chevrons/checks/×. Never set `strokeWidth` or a raw pixel size at a call site —
   that freedom produced 13 stroke weights across 62 files. **One carve-out:** an
   *illustration* (an empty-state or preview-card glyph at 20px+) is not chrome and may stay
   raw — but it must carry a comment saying so, because the grep below will flag it. Two exist
   today: `miller-columns.tsx` (26px preview) and `copilot-dock-ui.tsx` (20px empty state).
10. **Section labels go through `<Eyebrow>`** (`components/ui/eyebrow.tsx`) — `tone="loud"`
   for the one that needs attention, `as="span" | "label" | "h2" | …` when the layout needs
   that element. The old `.eyebrow` class is **deleted**; there is no other eyebrow.
11. **`font-display` (Space Grotesk) is for NAMES, not labels.** Below `text-lead` it belongs
   only on things that identify something — entity names, ticker symbols, document titles, the
   amber monogram — where it aids scanning. Section headings and pane labels under 15px use
   system sans; at `text-lead` and above, display font as usual. (Owner's call, 2026-08-12:
   Space Grotesk at 13px in a dense list read as a different app from the shell.)
12. **Spacing is the 4px step scale.** Arbitrary px is *layout* only (pane widths, the 38px
   rail cell), and layout is rare.
13. **Don't invent metrics.** No progress rings, mastery scores, or zeroed stat tiles for data
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
| `screens/` | reference renders; pre-pivot, so visual register only |
| `TRADING_PRD.md` | **the north star** — read for product intent |
| `RESEARCH_SURFACE_PRD.md` | LOCKED; the research folder pattern, and the best statement of the surface register |
| `INGESTION_PRD.md`, `SHARD_MODEL.md` | current for those subsystems |
| `TUTOR_PRD.md` | draft rev 3, design record; **nothing built** |

**Deleted, and why** — all recoverable from git history:

| removed | reason |
|---|---|
| `DESIGN_SPEC.md` | Tailwind **v3** config format (app is v4 inline `@theme`), a shadcn component map (app uses `@base-ui/react`), and specs for a Home dashboard, graph modal and "Ask-my-graph" that do not exist. Token names + don'ts salvaged into §5; `src/index.css` is the real source of truth. |
| `BROWSER_SPEC.md` | It was *accurate* — its numbers are in the code — so the intent-carrying parts were salvaged first: the drill rule (§3) and the Miller dimensions. Git history has the finer pixel detail. |
| `PRD.md` | The original product plan, built on the abandoned knowledge-graph framing (entities/theses/claims, Ask-my-graph, "graph persistence is the moat"). Superseded by `TRADING_PRD.md`. |
| `LEARN_PRD.md` | Superseded by `TUTOR_PRD.md`, which replaced the same surface. |
| `CURATION_UX_PRD.md` | Curation UX for the knowledge-graph product; referenced by no code. |
| `PORTFOLIO_PRD.md` | Deferred surface, but it **constrained T2** — so its three load-bearing rules (sleeve-relative target weights, attribution derived-never-stored, the residual is the product) were moved into `TRADING_PRD.md` §7 T2 before deletion. |
| `README.md` | An index to the above. Once `PRD`/`DESIGN_SPEC`/`BROWSER_SPEC` were gone it pointed at nothing. |

## 10. Checklist before you finish a UI change

```bash
cd aladin_react
node_modules/.bin/tsc --noEmit -p tsconfig.app.json   # NOT bare `npx tsc`
npm test
make check-tokens                                     # from repo root, if you touched theme.css
```

**Then grep your diff** — a rule with a check attached is the only kind that survives. Run
these against the changed files, not the whole tree:

```bash
# rule 1  colour — the `| grep -v` is required: rgb(var(--token)) is the legitimate form
grep -nE '#[0-9a-fA-F]{3,8}|rgba?\(' <files> | grep -vE 'rgba?\(var\(--'
grep -nE 'text-\[[0-9.]+(px|rem)\]' <files>           # rule 7  type
grep -nE 'rounded-\[[0-9]+px\]|rounded-(sm|md|lg)\b' <files>   # rule 8  radius
# rule 9  icons — capitalized tag only: <line>/<path>/<circle> in an SVG are not icons
grep -nE '<[A-Z][A-Za-z0-9]*(\s+[^<>]*)?(strokeWidth=|size=\{[0-9]+\})' <files>
grep -nE 'font-display' <files> | grep -E 'text-(meta|small|body)'   # rule 11 names-only
grep -nE '\b(p|px|py|m|mb|mt|gap)-\[[0-9]+px\]' <files>        # rule 12 spacing
```

The tree is not clean on rules 7–11 yet — the migration is staged (see the design-system
handoff). These greps are for *your diff*, so new code stops adding to the pile.

Then **look at it** — `npm run dev` and open the surface (a `/spike/*` route if it needs no
auth), in both themes. Grep your diff for `#` hex literals and `rgb(`. If you added a
component, confirm it's in the right place: primitive → `components/ui/`, everything else →
`modules/<feature>/ui/`.
