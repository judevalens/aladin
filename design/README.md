# Aladin — Handoff

Aladin is a personal knowledge graph that doubles as a research/trading assistant. It ships as a dark-minimal **IDE shell** with a Home briefing dashboard, a knowledge-graph explorer, and an **Ask-my-graph** command palette.

This package is documentation only — meant to be implemented in your existing **React + Tailwind + shadcn/ui** codebase.

## What's here
- **`PRD.md`** — the product plan: vision, target user, the trading-first wedge, the data model (entities / theses / claims), the two core loops (Capture + Consume), Ask-my-graph, the full feature set, risks, and the roadmap (what's built vs. parked). **Start here for the *what* and *why*.**
- **`DESIGN_SPEC.md`** — how to build the IDE design in **Tailwind + React + shadcn/lucide**: theme tokens (Dark + Soft) as CSS variables + Tailwind config, the shadcn component map, screen-by-screen layout/spacing/typography, the ⌘K + Ask-mode spec, icon mapping, and build order. **Use this for the *how*.**
- **`BROWSER_SPEC.md`** — a deep, exact spec for the **Folders view (the file browser)**: the nested tree, the **drill rule** (3 levels then a fixed 864×460 Miller-column popup), the cascading column behavior, leaf preview, right-click context menu, tabbed editor, and full state wiring. Use this when the browser needs to be pixel/behaviour-precise.
- **`/screens`** — reference renders of the static views (Home dark + soft, Folders, Signals, Sources, Graph). Overlays (command palette, drill-in panel, graph modal) are described in the spec.

## Fidelity
High-fidelity — final colors, type, spacing, and interactions are specified. Recreate faithfully with your shadcn components restyled to the Aladin tokens (don't ship default shadcn look).

## Suggested reading order
1. `PRD.md` §1–5 (vision, user, primitive, loops, shell).
2. `DESIGN_SPEC.md` §1–3 (tokens, component map, shell), then §4–6 for the key surfaces.
3. `PRD.md` §9–10 (roadmap + engineering notes) before architecting the data layer — **persistence of the graph is the #1 priority; it's the moat.**
