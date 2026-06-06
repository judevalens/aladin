# Aladin — Product Plan

> A personal knowledge graph that doubles as a research assistant.
> One engine for the domains you obsess over — trading and idea/market research first.

---

## 1. Vision

Notion and Obsidian won the **blank page**. Aladin wins the **pre-filled page**.

Most knowledge tools are inert containers: brilliant, but they do nothing until you feed them, and your research evaporates between sessions. Every new AI chat starts from zero context. Aladin's thesis is the opposite:

- **Stateful, cumulative research.** A living graph that the assistant maintains, so your thinking compounds instead of resetting.
- **The graph makes the assistant smart; the assistant keeps the graph alive.** Neither is interesting alone — the loop between them is the moat.
- **Memory is the compounding moat.** Features get copied in a quarter; accumulated personal context does not. The more you use Aladin, the more it knows what you care about, the sharper its signal, the more it hurts to leave.

**One-liner:** *Aladin keeps up with your inputs so you can spend your time on your output.*

---

## 2. Who it's for

People who live **downstream of a firehose** and are paid to have a point of view: founders, investors, analysts, operators, serious traders. Their unmet job-to-be-done is *"stay on top of my sources and turn them into my own thinking, fast."*

Win the devoted **individual power-user** single-player first (Obsidian's turf). Multiplayer/teams is a later expansion, not the wedge.

### The wedge: trading first
Trading and founder/market research are the **same primitive**:

> ingest a firehose → form a thesis → gather evidence for/against → track entities over time → make a decision → review the outcome.

Trading is the better **dogfood wedge**: higher frequency (daily vs. episodic), a brutal firehose (FinTwit), and a built-in scorecard (P&L) that tells you whether the assistant is actually making you better. Nail trading end-to-end; the research lens is mostly a re-skin of the same engine.

---

## 3. The core primitive

Everything is **one domain-agnostic engine**. Domains (trading, research) are *lenses/templates* over it, never separate apps.

### Data model (see `aladin/core-engine.jsx`)
- **Entity** — a typed node: `asset` (ticker), `company`, `person`, `concept`. Has `id`, `name`, `note`, `aliases` (for extraction matching).
- **Thesis** — a collection with a `stance` (long/short/bull/bear), a `domain`, a `conviction` (0–1), and a set of member entities. This is the unit of "a view you hold."
- **Claim** — evidence. Links a `source` → `entities` → a `thesis`, carrying a `stance` (`for`/`against`/`neutral`), `confidence`, `time`, and `sourceType` (tweet/article/note/filing). Claims are the atoms; for/against balance drives conviction.

This structure is why research stops evaporating: you can *see* the shape of your own thinking — what you believe, why, and where the holes are.

---

## 4. The two loops (what's built)

### Loop A — Capture (deliberate)
Messy input → structured graph data.
- **⌘K command palette** is the universal entry point (create, navigate, capture, ask). Capture is not a separate screen.
- **Extractor** (`extract()` in core-engine): pasted text → entities + claims + thesis routing. Uses Claude when available (`window.claude.complete`) with a robust deterministic fallback (`fallbackExtract`: alias matching, `$TICKER` detection, bull/bear stance lexicon, thesis routing by entity overlap).
- **Inbox**: captures pile up as pending items; nothing hits the graph until you confirm (`commitExtraction`). Inbox-style triage, not a blocking modal.

### Loop B — Consume (ambient) — the Home dashboard
"Triage what the world brought you," content-first.
- **Home = a welcome dashboard** (`aladin/dash.jsx`): greeting, a **daily Brief** (with history modal — the brief regenerates through the day), a **"Top for you"** feed of rich per-type cards (social / email / news), a right rail with **Up Next** (catalysts) and **Tracking** (topics you follow).
- **The graph is on-demand, not in your face.** A faint "why this" hint + a connection glyph sit on each card. Clicking opens a **slide-in panel** that's the payoff: *How this connects* — mentions, linked topics, echoes of your own notes, related feed items.
- **The panel is explorable** ("pull the thread"): every node is clickable and the panel re-centers on it, with a back trail. **"Open in graph"** promotes the focus into a radial node-link **graph modal**.
- **Consuming grows the graph.** From the panel you **Accept** an Aladin-suggested link or **connect by hand** — both write a real claim into the graph, with a subtle "graph grew" acknowledgment.

### The assistant (three modes, over the graph)
- **Ask (pull)** — answer questions grounded only in your graph.
- **Devil's advocate** — the strongest counterarguments you're underweighting.
- **Surface (push)** — proactively connect recent claims to theses you hold.

### Ask-my-graph (latest addition — `aladin/graph-qa.jsx`)
⌘K does double duty: type a **question** (ends in `?` / starts with what/how/why…) instead of a command and it flips into **ask mode**.
- **Cold start:** opening ⌘K empty shows ~4 smart suggested questions derived from the live graph.
- **Three intents:** **recall** ("what do I know about X"), **discovery** ("what connects X and Y"), **temporal** ("what changed on X lately").
- **Answer = synthesized card on top, grounded sources below.** Every answer lists the exact graph nodes (theses, claims with stance pills, feed items) it drew from — each clickable into the graph modal.
- LIVE synthesis via Claude with a deterministic, genuinely-useful fallback.
- **Follow-ups** threaded; **Save** writes the answer back into the graph.

---

## 5. The shell (what the app is)

A **dark-minimal IDE shell** (the canonical app), shipping in two themes — **Dark** (high-contrast) and **Soft** (lower-contrast) — built from one codebase where only the palette differs.

Left activity rail → views:
- **Home** — the consume dashboard (default).
- **Folders** — a knowledge explorer with nested folders and the four artifact types (page / link / voice / file) as leaves. Deep-folder navigation uses a **fixed four-column Miller-column popup** (cascading, anchored, dismiss-on-outside-click) reachable by drilling in or via **right-click context menu**. (We compared drill-down, sticky-scroll, and Miller columns; Miller won.)
- **Signals** — a curated, LLM-distilled feed from your sources (filter sidebar + ranked cards + save/dismiss).
- **Sources** — manage the feeds/monitors that produce signals.
- **Graph** — the knowledge-graph view.
- **⌘K** — command palette + capture + **Ask-my-graph**, from anywhere.

---

## 6. Architecture & file map

Prototype is React via in-browser Babel (no build step). Each `<script type="text/babel">` shares one global scope, so modules export to `window` and read shared globals. **Watch for global name collisions** (we hit one: `SOURCES` in `feeds.jsx` vs `room-data.jsx` — namespaced the dashboard's reads to fix it). A real build should move these to ES modules.

**Deliverables (root):**
- `Aladin - Dark IDE.html` — the app, dark theme.
- `Aladin - Soft IDE.html` — same app, soft theme (sets `window.__ALADIN_THEME` + `window.__ALADIN_AK` before load).

**Engine & data (`aladin/`):**
- `core-engine.jsx` — tokens (`ak`), unified icon set, the **GRAPH** (entities/theses/claims, seeded with one trading + one research thesis), query helpers (`G`), the **extractor**, **commit**, the **assistant** functions, and the **Inbox**.
- `room-data.jsx` — ambient layer: connected sources, the user's own notes/voice memos (for "echo" links), the incoming stream (each item pre-linked to a graph node with a typed connection + reason).
- `shared.jsx`, `feeds.jsx` — IDE sample data, icon paths, the Signals/Sources sample feed.

**Home dashboard (`aladin/`):**
- `dash-data.jsx` — feed items, tracked topics, catalysts, brief history.
- `dash-cards.jsx` — rich per-type cards + rail widgets.
- `dash-graph.jsx` — graph-on-demand: neighborhood derivation + the radial GraphModal.
- `dash-panel.jsx` — the explorable, actionable slide-in detail panel.
- `dash.jsx` — the dashboard home view (`DashboardHome`) + Brief history modal.

**Views & shell (`aladin/`):**
- `signals.jsx`, `sources.jsx`, `graph.jsx` — the rail views.
- `graph-qa.jsx` — Ask-my-graph engine + the in-palette AskView.
- `ide.jsx` — the shell: rail, folders/Miller columns, tab strip, editor, **⌘K command palette** (now with ask mode), and `AladinIDE`.

**Explorations (not loaded by the app — safe to delete):** `dark.jsx`, `dense.jsx`, `refined.jsx`, `design-canvas.jsx`, `Aladin Redesign.html`, `Aladin - Sticky Tree.html`.

---

## 7. Design system

- **Aesthetic:** dark-minimal, IDE-like, calm. Content leads; the graph machinery recedes until summoned.
- **Type:** Space Grotesk (display), system sans (body), JetBrains Mono (labels/meta).
- **Color:** warm amber accent (`#c9925a`) on near-black surfaces; semantic hues — green = supports/for, red-clay = counters/against, blue = catalyst, violet = your own notes ("echo"). Soft theme lifts the surfaces and softens contrast; semantic hues unchanged.
- **Principles:** minimalism (one thousand no's for every yes), no data-slop, graph-as-payoff (reward curiosity, don't tax it), flex/`gap` layouts, ≥44px hit targets.

---

## 8. Make-or-break risks (PM watch list)

1. **Don't drift into "a nice reader."** The moat is the *graph + assistant* loop, not "chat with your notes." Consuming must visibly compound the graph (it now does).
2. **Signal/answer precision is existential.** The whole promise is the push/answer being *right*. One week of irrelevant briefings and trust — the only moat — is gone. Quality of relevance > quantity of features.
3. **The assistant must build the graph as a byproduct.** Manual graph-tending kills every tool like this. Auto-construction from capture/consume is the difference between a category and another abandoned graph.
4. **Cold start.** First 5 minutes decide everything: connect sources, import existing notes, land the first real briefing within 24h. Memory must never start empty (ingest the user's corpus).
5. **Scope.** "Organize all things X for many X" sprawls into mush. Discipline = one shared primitive, one domain deep at a time, opinionated templates.

---

## 9. Roadmap

### Built & verified
- Domain-agnostic graph engine (entities/theses/claims) seeded for trading + research.
- Capture loop: ⌘K → extractor → Inbox triage → commit to graph.
- Consume loop: Home dashboard, rich feed, graph-on-demand panel (explore + grow), Brief history.
- Assistant: ask / devil's-advocate / surface.
- **Ask-my-graph**: ⌘K ask mode, recall/discovery/temporal, synthesized answer + grounded sources, suggestions, follow-ups, save.
- Unified IDE shell (Dark + Soft), Folders w/ Miller-column navigation + right-click, Signals, Sources, Graph view.

### Parked (deliberately, in priority order)
1. **Persistence** — make the graph survive refresh. This is what turns the demo into the actual memory moat. (Today everything is session-only.)
2. **Provenance & trust** — every claim shows source, time, and "Aladin inferred" vs "you asserted." Reviewed/typed write-path for any graph mutation.
3. **The outcome loop** — theses resolve: *thesis → outcome → what did I learn.* Trading's P&L is ground truth; closing this loop is what makes Aladin make you better. Add positions/journal + catalyst calendar for the trading lens.
4. **Delta view** — "what changed since you were last here" (new/moved/stale); overlaps with the temporal Ask intent.
5. **Cold-start / onboarding** — connect sources, import Notion/Obsidian/markdown so memory starts full.
6. **Proactive contradiction-surfacing** — ambiently flag tension ("you're Long NVDA but 4 of your last 5 signals are bearish").
7. **Custom tabs (future, power-user)** — copilot generates a constrained, declarative **AST** (not raw code) rendered into custom views over the graph engine + LLM capabilities. Re-promptable, safe, diffable. A low-code↔AI-assisted spectrum: broad strokes by hand, copilot finishes/polishes. *(Prototyped earlier, then parked to focus on the core shell.)*

---

## 10. Engineering notes for the rebuild

- **Move off in-browser Babel** to a real build (Vite/Next). Convert `window`-globals to ES modules — this removes the whole class of global-collision bugs.
- **Add an error boundary.** A render throw currently unmounts the whole tree (that's how the `SOURCES` collision showed up as a black screen).
- **Replace the mock LLM** (`window.claude.complete`) with a real provider; keep the deterministic fallbacks as offline/degraded behavior.
- **Persistence layer** first (local-first store + sync) — the graph is the product.
- **Typed write-path** for graph mutations (claims/links) with provenance, so generated tabs and the assistant can never silently corrupt the graph.
- The deterministic extractor + Q&A fallbacks are decent test oracles — keep them as the floor the LLM must beat.
