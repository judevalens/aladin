# Aladin — Handoff  ⚠️ HISTORICAL

> **This is the ORIGINAL design handoff package (pre-implementation). Parts of it are now
> wrong, and it is no longer the place to start.**
>
> - **For onboarding, read [`UI_ARCHITECTURE.md`](UI_ARCHITECTURE.md)** — the current
>   frontend map, verified against the code.
> - **For product intent, read [`TRADING_PRD.md`](TRADING_PRD.md)** — the north star.
>
> What below is superseded: Aladin is **not** a personal knowledge graph — that framing was
> abandoned for a personal algo/swing-trading research workspace. The `claims` layer it
> describes has been **deleted** from the codebase. "Persistence of the graph is the #1
> priority; it's the moat" is no longer true. The Home briefing dashboard it specifies does
> not exist (`/home` renders the Folders workspace). And this is no longer
> "documentation only — meant to be implemented": it is implemented and running in prod.
>
> `DESIGN_SPEC.md` and `BROWSER_SPEC.md`, which this package used to point at, have since
> been **deleted** — a Tailwind v3 config and a shadcn component map that no longer match the
> app. What was still true (tokens, the drill rule, the don'ts) moved into
> `UI_ARCHITECTURE.md`; the rest is in git history. Kept here only as an archive of original
> intent.


Aladin is a personal knowledge graph that doubles as a research/trading assistant. It ships as a dark-minimal **IDE shell** with a Home briefing dashboard, a knowledge-graph explorer, and an **Ask-my-graph** command palette.

This package is documentation only — meant to be implemented in your existing **React + Tailwind + shadcn/ui** codebase.

## What's here
- **`PRD.md`** — the product plan: vision, target user, the trading-first wedge, the data model (entities / theses / claims), the two core loops (Capture + Consume), Ask-my-graph, the full feature set, risks, and the roadmap (what's built vs. parked). **Start here for the *what* and *why*.**
- **`/screens`** — reference renders of the static views (Home dark + soft, Folders, Signals, Sources, Graph). Overlays (command palette, drill-in panel, graph modal) are described in the spec.

## Fidelity
High-fidelity — final colors, type, spacing, and interactions are specified. Recreate faithfully with your shadcn components restyled to the Aladin tokens (don't ship default shadcn look).

## Suggested reading order
1. `PRD.md` §1–5 (vision, user, primitive, loops, shell).
2. `UI_ARCHITECTURE.md` (current) — tokens, shell, conventions.
3. `PRD.md` §9–10 (roadmap + engineering notes) before architecting the data layer — **persistence of the graph is the #1 priority; it's the moat.**
