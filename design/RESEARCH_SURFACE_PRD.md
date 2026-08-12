# PRD — The Research Bench

> **Audience:** a design agent prototyping Aladin's research surface.
> **Date:** 2026-07-31. Revision 5 — no strategy template; strategy code is a directory of files, IDE-shaped (§7, §20). Revision 4 — closes the two design opens (§15 Overview, §13 switcher). Rev 3 (2026-07-28) superseded rev 1 (which invented a `thesis` object) and rev 2 (which framed Aladin as a wrapper around someone else's backtest kernel). Both were wrong; see §0.
>
> Sections marked **LOCKED** are settled. Sections marked **OPEN** are not — don't design past them without asking.
>
> Companions: `TRADING_PRD.md` (engineering-facing north star; §5 and §7 still govern, §0/§3's "not a backtest engine" line does not) · `UI_ARCHITECTURE.md` (tokens + conventions, **authoritative for styling**) · `ENTITY_CONTEXT_PRD.md` (house style for hierarchy and provenance).

---

## 0. What changed, and why it matters

Two earlier framings are dead and should not be reintroduced:

1. **"Aladin is not a backtest engine, it wraps vectorbt/QuantConnect."** False. The author is writing the backtest platform. **Aladin is its control plane and UI.**
2. **"A thesis is the unit of research."** Invented. `TRADING_PRD.md` §2 puts `hypothesis` as a *column on strategies*. There is no object above a strategy. Everything named after "thesis" — the index, the lifecycle, the notebook — was scaffolding on a layer that doesn't exist.

Collapsing #2 removes most of rev 1: no thesis index, no 7-state lifecycle, no `falsification` field, no promotions table. **A strategy is the unit.**

---

# PART I — The product

## 1. What it is  **LOCKED**

Aladin is **an opinionated notebook for strategy research**, and the control plane for the trading engine the author is writing.

The notebook framing is exact and useful: a folder holding code, prose, and outputs, addressed as one unit, with kernel state visible in the chrome. But Aladin deliberately **inverts Jupyter's three defects**, and those inversions are the product:

| Jupyter | Aladin |
|---|---|
| Hidden state — cells run out of order, the notebook doesn't reproduce | Every run pins a code hash; runs are immutable |
| Re-run a cell and the previous output is gone | Every run is kept, forever |
| No notion of "armed" | Execution mode, live scanning, and broker truth are first-class |

**Guardrail:** any feature that makes Aladin *more* Jupyter-like — interactive re-run in place, mutable cells, ephemeral outputs — is the direction to refuse.

## 2. Who it's for  **LOCKED**

The author, alone. A novice trader with strong programming and math, learning markets by building the harness.

- No onboarding, no tutorials, no empty states explaining domain terms.
- Dense is fine. Mono type, tight numbers, IDE energy.
- Not a Bloomberg pastiche. No data-slop, no 40-widget dashboards.
- Multi-user, sharing, publishing: **out of scope.**

## 3. What the surface has to do  **LOCKED**

Stated in the author's words, and this is the scope:

1. Control a strategy — run it.
2. **See what data is coming through it.**
3. Result log.
4. A dashboard when it's live or paper trading.
5. Key metrics.
6. Past backtest results.
7. Glance at the code.
8. Group related research artifacts to that strategy — files, shards, pages, an imported white paper, a linked YouTube video, a pasted forum thread.
9. **All of it becomes a context artifact other agents can run over.**

Two notes on that list:

**#2 is the highest-value and least-specified item.** Everything else summarizes; that one teaches. When a backtest looks wrong the question is never "what's the Sharpe" — it's *what did the strategy see on that bar, and why didn't it enter?* It's also where lookahead and bad universes get caught. `TRADING_PRD.md` §1 says build the thing that teaches you the domain; this is that thing.

**#9 is why runs are kept forever** — not as a discipline, but because an agent reading only your winning runs will tell you what you want to hear.

## 4. What Aladin is not  **LOCKED**

- **Not the compute kernel.** Aladin renders the engine's inputs, progress, and results — never its internals. No profiler, no log tail, no queue gauges.
- **Not a broker.** Alpaca is the source of truth for positions. Aladin reconciles and never asserts over it.
- **Not social**, not low-latency, not a generic knowledge graph.
- **No auto-optimizer.** Sweeps *map* a surface for a human to read. An optimizer *picks*, which is overfitting with a progress bar. Hard line.

---

# PART II — The model

## 5. The research folder  **LOCKED**

The unit of work is a **research folder**: a tree node with `kind = 'research'` plus a 1:1 extension row holding strategy facts (manifest ref, code hash, universe, mode, run state).

This follows a pattern already in the repo twice — watchlists (one entity + a `kind` + a definition payload) and the trading entity model (thin `entities` + `kind` + hard 1:1 extension tables). One tree, one set of components, a discriminator.

**A research folder is not a folder with a label on it.** The kind buys four things a plain folder can't have:

- **Structural slots that exist at creation** — manifest, code ref, run log. Always present, empty or not.
- **Constrained creation.** `+` inside a research folder offers *launch a run · import a paper · add a note*, not the generic artifact menu. Opinion enforced where authoring happens is the only place it sticks.
- **Seeded typed properties.** Artifacts landing here get research-shaped property presets, not a blank key/value editor.
- **A stable address.** `research/pead-semis` is something an agent can be pointed at.

And it has **state** — running, armed, last run, in-flight sweep. Folders don't have state. That's the clearest tell it's a different kind.

**One research folder = one strategy**, including the case where no code exists yet (the extension row is just sparse). Comparing two strategies is a view *above* research folders, never a folder containing them.

## 6. Containment  **LOCKED**

**One primary home, many appearances.** The folder is where an artifact lives. The existing artifact-link machinery is how the same white paper *appears* in a second research context without being duplicated. Anything rendered outside its home is visibly foreign — dimmed or origin-tagged. Cross-research reference is possible but never accidental and never silent.

## 7. Strategies are pluggable  **LOCKED**

A strategy arrives from one of three places — **a git import, a local directory, or authored in Aladin** — and all three converge on **one representation**: a directory with a manifest plus code, identified by content hash.

**There is no template.** Rev 5 correction: "authored in Aladin" means you create files in the folder the way you would in an IDE — **strategy code is a directory of files, not a document**. Aladin scaffolds nothing; an empty strategy is an empty directory. The Code surface is therefore a file tree plus an editor pane (§20), not a single-file view, and nothing in the product should imply a starting template.

**The strategy declares its parameter schema; the manifest supplies the values.** The code says *I take `fast_ma: int`, `slow_ma: int`, `atr_mult: float`* with types, defaults, and bounds. Aladin introspects that and **renders the configure form and the sweep launcher generically**. This is what makes "pluggable" real — without it, every imported strategy needs hand-built UI.

- **Git imports pin a commit SHA, never a branch.** Otherwise upstream moves and the hash changes under runs that already happened. Updating is an explicit act producing a new version; old runs keep their old SHA.
- **One environment per strategy.** Two imported strategies will eventually want incompatible packages. Cheap to assume now, expensive to retrofit.

## 8. The execution contract  **LOCKED (with a staged rollout)**

**Event-driven is the primitive. Batch is an explicit opt-out.**

Event-driven makes the lookahead guarantee *structural* — the engine hands you bars ≤ T and there is nothing to get wrong. Batch makes it *aspirational*: you hand the strategy the whole array and trust it not to index forward, which Python can't prevent.

So the engine synthesizes batch by collecting and calling once, and **a strategy declaring `mode: batch` is visibly marked in the UI** as one where lookahead isn't structurally prevented. Don't forbid it; never let it hide. (The reverse — batch as primitive, event as a loop over it — loses the guarantee everywhere.)

**Ship event only.** Add batch against a real trigger (the first cross-sectional ranking strategy), not speculatively.

## 9. Code editing  **LOCKED**

`Aladin owns the manifest, not the math` is **obsolete**. Strategies will be editable in-app later via Monaco + LSP. Not now.

The invariant that protects reproducibility is editor-agnostic: **every run pins a code hash, and runs are immutable.** Where you type doesn't threaten that; hidden state does.

- **The dirty-buffer trap:** hash what actually executes, not what's on disk. Otherwise numbers get attributed to the wrong code, silently.
- Versions are created lazily **at run time**, not on every keystroke.
- **Design the code tab now as an editor that happens to be read-only** — same gutter, same line numbers, same layout — so enabling it later is a capability flip, not a redesign. Don't put controls where a minimap or breadcrumb bar will need to live.
- When LSP lands, it points at *that strategy's own environment*, so completions reflect the real engine API.

## 10. Executable code beside prose  **LOCKED**

Yes — with one boundary. Two different things get called "executable code":

- **The strategy.** Hashed, versioned, identical in backtest and live.
- **Analysis.** Plotting an equity curve against SMH, checking holding-period distribution, inspecting what the strategy saw on a bar. Doesn't feed live trading.

**Analysis cells can read runs, bars, and trades. They cannot define what trades.** The moment a cell could become a strategy, hidden state is back.

Three reasons this is cheap: pages are already block-based (a code cell is a block type, not a new artifact type); shards already do sandboxed execution; and the kernel is the engine itself, so a cell queries the identical data layer the backtest used.

**Cell outputs must record which run and version they read.** Otherwise the notebook re-runs six weeks later against newer data and the prose beside it describes something that no longer exists — the same disease in a different organ.

---

# PART III — Navigation  **LOCKED**

## 11. No new rail item

The research bench uses **the existing workspace shell**: the standard left browser pane, and the right work pane with tabs. No separate route, no second tree.

[`work-pane-ui.tsx`](../aladin_react/src/modules/workspace/ui/work-pane-ui.tsx) already has the tab strip, the breadcrumb bar, the side-pane toggle, and — the important part — a **switch on `activeArtifact.kind`** that picks the viewer. Research tabs are **new branches in that switch**: `overview`, `manifest`, `runs`, `run`, `code`, `inspect`.

**The one real refactor:** the tab model widens from a list of artifacts to a discriminated union carrying a `contextId`, since "Runs" and "Manifest" are views on the research extension row rather than rows in the artifact table.

**Structural slots are typed children in the normal tree.** Expanding a research folder in the browser pane gives you the notebook sidebar for free — one tree implementation, no parallel navigation.

**Discovery** is a lens/filter on the browser pane (`kind = research`), reusing the property-filter surface already shipped. Not a rail entry.

## 12. Tab grouping

**Grouping is an ordering invariant, not a mode.** All tabs live in one row; tabs belonging to the same research are always contiguous.

- **Membership is derived, not managed.** A tab is in a group because its artifact lives in that research folder. No add-to-group, no drag-between-groups, no naming or coloring a group. That deletes an entire surface of UI.
- **Group order is stable**, never most-recently-used — MRU re-sorts the row and moves every target.
- **No per-group color.** Aladin has one accent; hue-coding groups breaks the system. Ownership reads from a leading group label plus a heavier divider at each run's end.
- **Loose artifacts sit at the far right**, past every group, so groups anchor to the start.

**Collapse is an option layered on top**, not part of the model:

- **The active group is always expanded.** Collapse applies only to the others, which removes the whole class of "what if I collapse the group I'm in."
- **"Collapse others" is the focus affordance** — a thing the user asks for, not a mode the app imposes.
- **A collapsed group still shows run state.** The moment it can hide a running sweep, the deleted status strip has to come back.
- **No auto-collapse on overflow.** Tabs vanishing unasked is the surprise this model exists to avoid. Let the row scroll.
- Collapse state persists across sessions.

## 13. The quick switcher  **LOCKED**

**It lives in the left browser pane, not the tab row.** Tab-row width is the scarcest space in the shell, and the switcher is navigation — it belongs where navigation already lives.

**It is a button in the pane's header that opens the list as a popover.** Not a persistent *Research* section pinned above the tree. The pane's vertical space belongs to the tree, and the switcher is a thing you reach for, not a thing you read.

Behavior:

- It presents a **vertical list** — full names plus mode, run state, tab count, last touched. Four flask icons are indistinguishable; a list isn't.
- **Reuse the existing cmdk-based command primitive.** Arrow-key traversal and type-to-filter. If it's a button, anchor the list as a popover rather than a centered modal.
- **Selecting a research aligns its first tab to the left edge** of the tab area — not centered. Centering leaves unrelated tabs on both sides; left-aligning makes the group read as the thing you're in.
- **Selecting scrolls *and* activates** — that group's last-active tab, or its Overview the first time. Otherwise you jump and the content pane doesn't follow, which reads as broken.
- **Stable ordering + type-to-filter.** (If `⌘1`–`⌘9` direct jumps are wanted, ordering *must* stay stable — can't have that and MRU.)
- It's also where you **open** a research: one not currently in the row appears in the list, and selecting it adds its group.

**What left-pane placement costs:** the browser pane is collapsible and the tab row isn't. Two things therefore must not depend on it —

1. **A keyboard binding is mandatory**, not a nicety. With the pane hidden it's the only way to switch.
2. **Run-state awareness** can no longer ride the switcher. It doesn't need to: §12 puts the run dot on the tab group, so anything open in the row still shows state. The only gap is a research whose tabs were closed while a sweep was in flight — and that case belongs to the **notification system**, which already exists and is the natural consumer for run completion. Out of sight, and it tells you when it's done. Don't reintroduce a persistent global indicator for it.

## 14. No status strip

Cut. Run state and mode live as a **dashboard section inside the Overview artifact**, and the **tab group chip** carries the ambient indicator (§12). The strip was redundant the moment groups could show state. Anything out of the row falls to notifications (§13).

**One exception, deferred:** cancelling a sweep from the Overview is fine — it isn't urgent. A kill switch for real money that requires navigating to a tab is not. That's T5, it's last, and it should be revisited when execution lands.

## 15. The Overview artifact  **LOCKED**

The front page of every research folder, pinned first in its group, and the first thing an agent reads.

**It is a typed artifact with fixed structural sections — the strategy's control plane.** Not a page. A page can be emptied, which breaks the guarantee that there is always a manifest and always a run log; and the sections here are *live*, not authored.

It reads like a dashboard, and it is where you:

- **see and change run state** — mode, armed/offline, what's in flight;
- **launch** a known configuration, and **bring the strategy offline**;
- **see live numbers** when it's paper or live trading;
- **see aggregate stats across runs** — not one run's report card, the shape of all of them;
- **enter everything else** — manifest, runs, code, inspect.

One freeform prose region persists inside it, carrying the hypothesis (§16, and `TRADING_PRD.md` §2 puts `hypothesis` on the strategy). Structural sections cannot be deleted; the prose region can be empty.

**Boundary with §17 Control.** Overview *launches*; Control *composes*. Overview runs a configuration that already exists — the last one, the default one, the pinned one — as one action with no form. Anything that requires choosing parameters, widening a sweep range, or setting the held-out window is Control. If a control on Overview grows a form, it has moved to the wrong pane.

**Arm/offline is a real control on this surface**, which partially answers §14: the disarm sits on the pinned-first tab of the group rather than behind navigation. It does not fully answer it — §14's deferred exception stands until execution lands (T5).

---

# PART IV — The panes

Each is a branch in the kind-switch of §11.

## 16. Overview
The control plane: run state and arm/offline, one-action launch of a known config, live numbers, aggregate stats across runs, hypothesis prose, and the entry points to everything else. §15.

## 17. Control
Configure and launch — the composing surface, where Overview is the launching one (§15). The param form is **generated from the strategy's declared schema** (§7), so it works for any imported strategy. Three launch modes: single run, sweep (show the combination count as ranges widen — watching `3 × 8 × 12 = 288` appear is free honesty), and walk-forward.

**Held-out data is a required field, not an option.** Aladin owns the launcher, so it can simply refuse to consume all available history. Make it an ordinary, unavoidable part of the form — not a warning, not a checkbox, and never a modal that scolds. Good methodology should be the path of least resistance.

## 18. Inspect — what the strategy saw
The teaching surface (§3). Bars in, indicator values, the decision and its reason, for a chosen symbol and date. This is where lookahead, bad universes, and off-by-one entries get caught. **Design this with as much care as the sweep surface** — it earns its keep from the first run, whereas parameter surfaces only matter once you're sweeping.

## 19. Runs — the result log
Every run, kept forever. Immutable, citable, chronological by default. **Never sort best-first** — sorting is an explicit act.

The genuinely good part of the earlier mockups, worth keeping:

- **In-sample and held-out surfaces side by side**, shared color scale, linked cursor. The contrast *is* the argument.
- **`best OOS in sweep` shown next to the cell you picked.** One number, and more damning than any grid.
- **The neighborhood** of a selected cell — plateau means robust, spike means noise. Deterministic verdict computed from the neighborhood stats, never LLM-generated; the first time it's wrong the surface loses the authority that makes it worth having.
- **Universe resolved as-of** and the delisted-symbol count, per run. That's how survivorship stops being theoretical.

**On the sweep heatmap palette:** the token set has no sequential ramp. Propose one that survives every theme, stays legible for colorblind readers, and doesn't compete with amber. Encode magnitude with more than hue (cell size, value on hover) — and distinguish *a bad result* from *not yet run*, since partial sweeps are a real state.

## 20. Code
Read-only for now, **drawn as an editor** (§9) — and specifically as an **IDE-shaped surface: a file tree beside an editor pane**, because a strategy is a directory of multiple files (§7), not a document. Shows path, code hash, runtime, engine version.

## 21. Research artifacts
The captured material — pages, links, files, shards, imported papers, pasted threads, voice memos — living in the folder via the existing capture surface. Nothing new to build here structurally; it's the reason not to build a new grouper.

---

# PART V — Design system

`UI_ARCHITECTURE.md` §5 governs. Never hardcode a hex.

**Surfaces** `bg-rail` · `bg-panel` · `bg-bg` · `bg-chrome` · `bg-field` · `bg-card` · `bg-raise` · `bg-explorer`
**Ink** `text-ink` → `text-ink-2` → `text-ink-3` → `text-ink-4`
**Accent & lines** `bg-amber` · `bg-amber-soft` · `border-amber-line` · `border-line` · `border-line-2`
**Semantic** `text-for` (green) · `text-against` (red-clay) · `text-catalyst` (blue) · `text-echo` (violet, the user's own material)
**Radii** `rounded-chip` 7px · `rounded-card` 12px · `rounded-modal` 14px

Dark-theme reference: `--bg:#0d0d10`, `--panel:#0f0f12`, `--card:#121215`, `--ink:#eceaef`, `--amber:#c9925a`, `--for:#5cba8f`, `--against:#d8796b`. Themes swap under one component tree via `data-theme` — Dark, Soft, Cool, Contrast, Linear, Apple-dark, Light.

**Type.** Space Grotesk display, system sans body, **JetBrains Mono as a first-class UI font** — 9.5–11px, often uppercase with 0.4–0.8px tracking, for labels, counts, timestamps, hashes, tickers, params, metrics. Most small text here is mono. Numbers are tabular, consistently rounded, signed values colored `for`/`against`. A price, a Sharpe, and a code hash never render in body sans.

**Components.** Primitives in `src/components/ui/` on Base UI + `cva`, restyled — never default shadcn. Reuse Sheet, Dialog, Command/CommandDialog, ContextMenu, Popover, Tabs/ToggleGroup, Badge, Sonner, ScrollArea, lucide-react at ~1.7 stroke. The entity chip, YOURS/FOUND origin tag, pending-suggestion amber banner, and verbatim quote block exist on Entity Context — reuse their vocabulary exactly. `AreaChart` and the treemap in `modules/markets/ui/` are the chart family equity curves should join.

**Feel.** Dark-minimal, IDE-like, calm. Amber is the only thing that pops. Motion 150–260ms `cubic-bezier(.2,.8,.2,1)`.

---

# PART VI — Sequencing and open questions

## 22. Two halves that decouple

**Buildable today** — the research container: the `kind`, the folder, its artifacts, the read-only code glance, notes, and the Overview shell. Useful on day one, before a single backtest exists, and it rides the capture surface that already ships.

**Waits on the engine** — control, inspect, runs, metrics.

## 23. Prototype order

1. **Overview** — §15 is settled (typed control plane), so this is drawable now, and it decides the visual register for everything else.
2. **Tab grouping + switcher** — §12–13. Small, and it's the shell everything else sits in.
3. **Inspect** — §18. The unsolved one, and the one that earns its keep earliest.
4. **Runs / sweep surface** — §19.
5. **Control** — §17.

## 24. Open

*Closed in rev 4: §15 (Overview is a typed control plane) and §13 (switcher is a header button + popover).*

- **D6, as-of universe resolution.** Unresolved in `TRADING_PRD.md` §6 and it **blocks T2**. How "what to scan" is expressed and resolved as-of a date decides the shape of the universe field in §17.
- **The rail's existing "Signals" item** means *content feed* and now collides with trading signals. One of them renames.
- **Sequential palette for the sweep surface** (§19).
- **What Overview shows before any run exists** — the most common state early on, and the easiest to leave as a void.
