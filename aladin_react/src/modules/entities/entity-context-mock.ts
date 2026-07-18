// Entity Context — Phase A mock data, ported verbatim from the design handoff
// reference (design_handoff_entity_context/reference/entity-context.jsx).
// The handoff's hex tones are replaced by semantic tone keys that the UI
// resolves to Aladin theme tokens (see entity-context-ui.tsx).

import type { ContextItem, Edge, Entity } from "./entity-context-types";

// ── the focused entity + its web ──────────────────────────────────────────────
export const ENTITY: Entity = {
  name: "Memory as a moat",
  kind: "concept",
  gist: "The durable advantage in AI tools is the accumulated user context, not the base model — whoever owns the notes owns the agent.",
  watchers: ["x", "rss", "reddit"],
  since: "tracked 3 weeks · 14 pieces of context",
};

// edges — each marked as authored by you or discovered by a watcher
export const EDGES: Edge[] = [
  {
    rel: "enables",
    to: "Switching costs",
    kind: "concept",
    why: "If your context lives here, leaving means rebuilding it — that IS the lock-in.",
    origin: "you",
  },
  {
    rel: "enabled_by",
    to: "Accumulated user context",
    kind: "concept",
    why: "The moat is only as deep as what the user has fed in over time.",
    origin: "you",
  },
  {
    rel: "supports",
    to: "Cursor retention",
    kind: "entity",
    why: "Power users cite “everything is already in here” as the #1 reason they don’t churn.",
    origin: "watcher",
  },
  {
    rel: "supports",
    to: "ChatGPT distribution",
    kind: "entity",
    why: "Distribution, not model lead, is what competitors can’t replicate.",
    origin: "watcher",
  },
  {
    rel: "part_of",
    to: "Aggregation theory",
    kind: "concept",
    why: "A special case: surplus accrues to whoever owns the demand-side relationship.",
    origin: "you",
  },
  {
    rel: "contradicts",
    to: "Model step-change resets moats",
    kind: "concept",
    why: "A big enough capability jump could make switching worth it again — the open threat.",
    origin: "watcher",
  },
  {
    rel: "competes",
    to: "Distribution as the moat",
    kind: "concept",
    why: "Rival explanation for the same retention data — which one is load-bearing?",
    origin: "you",
  },
];

// context — real material, verbatim. Never paraphrased into a "claim".
export const CONTEXT: ContextItem[] = [
  {
    type: "quote",
    body: "memory is the moat. the model is a commodity now — what compounds is the context you’ve fed it. whoever owns the notes owns the agent.",
    who: "@swyx",
    platform: "x",
    time: "2h",
  },
  {
    type: "note",
    body: "This is the whole Aladin thesis, turned inward. I’m dogfooding the belief by building the tool the belief predicts.",
    who: "me",
    platform: null,
    time: "2h",
  },
  {
    type: "quote",
    body: "Switching cost in AI tools is the corpus you’ve imported, not the UI.",
    who: "Stratechery",
    platform: "rss",
    time: "1d",
  },
  {
    type: "question",
    body: "Does a step-change model (GPT-6-class) reset switching costs, or has the context moat already compounded past that?",
    who: "me",
    platform: null,
    time: "5h",
  },
  {
    type: "quote",
    body: "true only while model quality is at parity. a big enough jump and people re-import everywhere.",
    who: "@gdb",
    platform: "x",
    time: "9h",
  },
  {
    type: "quote",
    body: "my second brain is a junk drawer of AI summaries i’ll never read.",
    who: "r/ObsidianMD",
    platform: "reddit",
    time: "6h",
  },
];

// The pending suggestion behind the RelSignal banner (PRD §4.1). In the handoff
// the banner copy is hardcoded and inert; here "Keep edge" appends this edge to
// the local list as FOUND (session-only). The why is authored for the mock —
// the reference carries no why for the suggestion itself.
export const SUGGESTED_EDGE: Edge = {
  rel: "supports",
  to: "Cursor retention",
  kind: "entity",
  why: "Wired in by your watcher as supporting evidence.",
  origin: "watcher",
};
