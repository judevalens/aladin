import { useState } from "react";
import { Boxes, FileText, Paperclip, type LucideIcon } from "lucide-react";

import { cn } from "@/lib/utils";

/**
 * Dev-only spike (/spike/tutor-notebook) — attempt 3 at the Tutor surface, and the first
 * one that is a PLACE rather than a SESSION.
 *
 * Why the first two failed (they were rejected as "confusing", and de-boxing one of them
 * did not help — so it was never visual density):
 *
 *   · /spike/tutor      three panes, a scenario switcher, an "Ask" pane with no composer,
 *                       and a `This item / A concept` toggle that leaked read_document vs
 *                       search_document into the UI as a REQUIRED choice before asking.
 *   · /spike/tutor-read no panes, but its only affordance was invisible (text selection),
 *                       generated content interleaved into the source's own flow, and
 *                       nothing survived — an X deleted an answer with no record.
 *
 * The shared root cause: both asked the user to hold Aladin's ontology — plan vs plan item
 * vs aid vs pointer, span vs concept retrieval, the seam between source text and extracted
 * text — in their head in order to study a PDF. Both also grew a second chat surface next
 * to the Copilot dock, and both re-rendered a fake reader when the real DocumentViewerUI
 * already ships (and deliberately shows PAGES, because flattening tables and figures
 * destroys them).
 *
 * So this is modelled on research-overview-ui.tsx (RESEARCH_SURFACE_PRD §11: no rail item,
 * reuse the workspace shell; a single column, a few labelled sections, no chrome).
 *
 * TAKE 4 — tutor-specific, because take 3 was a passive index and that is not enough.
 * The shape being close to research is fine and intended; what makes this Tutor is that
 * the folder is the AGENT'S CONTEXT and the surface is a control plane over it:
 *
 *   what I want out of this  ·  what we're learning  ·  ask the tutor  ·  what's in here
 *
 * "What we're learning" is the top-down view — topics, not page ranges, each carrying a
 * state that comes from something that actually happened (a quiz taken, an aid built).
 * Research's §15 boundary applies unchanged: this surface LAUNCHES a known thing in one
 * action (quiz me on this topic); anything needing a form is Control and lives elsewhere.
 *
 * The honesty rule stays load-bearing. State is rendered from real events only — there is
 * no mastery score, no % read, no streak, and a topic nothing has happened to says exactly
 * that. Suggestions are marked as the agent's, not as fact, and cost one click to accept.
 *
 * Rules it holds to, each earned from a specific failure above:
 *   - no panes, no tabs, no toolbar, no mode switcher, no composer, no reader
 *   - Copilot stays in the dock; asking happens there, and results land here as rows
 *   - no hover-revealed actions; one click target per row
 *   - no copy that explains or defends the design
 *   - no invented metrics: no % read, no mastery score, no streak, no zeroed tiles
 *   - amber appears at most twice, and only where attention is genuinely required
 *   - empty states name what belongs there, in one sentence
 */

// ── mock: one notebook over the paper actually ingested into prod ────────────────────
const NOTEBOOK = {
  title: "Option strategies",
  goal:
    "Be able to price a collar from its legs and say what each greek measures. I keep bouncing off §6 because the partial-derivative notation loses me.",
};

type MaterialRow = {
  id: string;
  title: string;
  kind: "file" | "note" | "app";
  icon: LucideIcon;
  kindLabel: string;
  ingest?: "reading" | "needs ocr" | "unreadable";
  primary?: boolean;
};

const MATERIAL: MaterialRow[] = [
  {
    id: "m1",
    title: "Option Strategies & Payoff Algebra",
    kind: "file",
    icon: Paperclip,
    kindLabel: "pdf · 361p",
    primary: true,
  },
  { id: "m2", title: "Why the collar width is the whole risk profile", kind: "note", icon: FileText, kindLabel: "note" },
  { id: "m3", title: "Collar payoff", kind: "app", icon: Boxes, kindLabel: "shard" },
];

/**
 * A topic is the unit of the top-down view. `state` is DERIVED from events, never set by
 * a scoring model: untouched · reading · checked (with the real score) · shaky (a specific
 * miss). That keeps the surface from inventing a fact about the user's understanding.
 */
type Topic = {
  id: string;
  title: string;
  objective: string;
  span: string;
  state:
    | { kind: "untouched" }
    | { kind: "reading" }
    | { kind: "checked"; got: number; of: number }
    | { kind: "shaky"; missed: string };
  aid?: { title: string; kindLabel: string };
};

const TOPICS: Topic[] = [
  {
    id: "t1",
    title: "Payoff algebra of the basic legs",
    objective: "Write the payoff of a call, put and forward from first principles.",
    span: "pp. 41–68",
    state: { kind: "checked", got: 5, of: 5 },
  },
  {
    id: "t2",
    title: "Collars and spreads",
    objective: "Derive max gain and max loss for a collar from its legs.",
    span: "pp. 88–104",
    state: { kind: "shaky", missed: "which leg sets the floor" },
    aid: { title: "Collar payoff", kindLabel: "shard" },
  },
  {
    id: "t3",
    title: "Greeks as partial derivatives",
    objective: "Read each greek as a partial derivative and say what it measures.",
    span: "pp. 152–197",
    state: { kind: "reading" },
  },
  {
    id: "t4",
    title: "Volatility surface fitting",
    objective: "Explain why a single vol number cannot price a whole book.",
    span: "pp. 244–301",
    state: { kind: "untouched" },
  },
];

/**
 * The agent's proposals, from parts of the source no topic covers yet. Marked as
 * suggestions and accepted in one click — never silently added, because the plan is the
 * user's document and §4a's whole point is that they argue with it.
 */
const SUGGESTED = [
  { id: "s1", title: "Put-call parity", why: "leant on from §4.2 onward but never covered", span: "pp. 71–80" },
  { id: "s2", title: "Early exercise on American puts", why: "the only section your goal mentions that has no topic", span: "pp. 118–131" },
];

export function TutorNotebookSpike() {
  // The spike renders the surface inside a plain scroller. In the real thing this is a
  // branch in work-pane-ui.tsx's kind switch — no route, no rail item.
  return (
    <div className="h-screen overflow-auto bg-bg">
      <NotebookOverview />
    </div>
  );
}

function NotebookOverview() {
  // The top-down view in one line: how many topics, and how many need attention. Counts of
  // states that actually happened — not a percentage, and not a score.
  const shaky = TOPICS.filter((t) => t.state.kind === "shaky").length;
  const untouched = TOPICS.filter((t) => t.state.kind === "untouched").length;
  return (
    <div className="mx-auto w-full max-w-[62rem] px-8 py-7">
      <header className="mb-7">
        <h1 className="font-display text-[24px] leading-tight text-ink">{NOTEBOOK.title}</h1>
        <div className="mt-2 flex flex-wrap items-center gap-x-4 font-mono text-[10.5px] uppercase tracking-[0.5px] text-ink-4">
          <span>1 source</span>
          <span>{TOPICS.length} topics</span>
          {shaky ? <span className="text-amber">{shaky} shaky</span> : null}
          {untouched ? <span>{untouched} untouched</span> : null}
        </div>
      </header>

      <Goal />
      <Learning />
      <AskTheTutor />
      <Material />
    </div>
  );
}

/**
 * The freeform region, and the most prominent thing on the page — modelled on research's
 * Hypothesis. The largest element is the one field the user writes in their own words,
 * because it is the only thing here that no agent can supply, and it is the brief every
 * later turn inherits.
 */
function Goal() {
  const [draft, setDraft] = useState(NOTEBOOK.goal);
  return (
    <section className="mb-8">
      <SectionLabel>What I want out of this</SectionLabel>
      <textarea
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        rows={3}
        placeholder="What do you want to be able to do afterwards — and what keeps losing you?"
        className="w-full resize-none bg-transparent font-display text-[15px] leading-relaxed text-ink outline-none placeholder:text-ink-4"
      />
    </section>
  );
}

/**
 * What is in this notebook, flat, source first. Every row opens as a tab in the shell and
 * does nothing else — the notebook lists things, it never displays them.
 */
function Material() {
  return (
    <section className="mb-8">
      <SectionLabel>What's in here</SectionLabel>
      <div className="-mx-2">
        {MATERIAL.map((m) => (
          <button
            key={m.id}
            type="button"
            className="flex w-full items-baseline gap-3 rounded-card px-2 py-1.5 text-left transition-colors hover:bg-raise"
          >
            <m.icon className="mt-0.5 size-3.5 shrink-0 text-ink-4" />
            <span
              className={cn(
                "min-w-0 flex-1 truncate text-[13px]",
                m.primary ? "text-ink" : "text-ink-2",
              )}
            >
              {m.title}
            </span>
            {m.ingest ? (
              <span className="shrink-0 font-mono text-[10px] text-amber">{m.ingest}</span>
            ) : null}
            <span className="shrink-0 font-mono text-[10px] text-ink-4">{m.kindLabel}</span>
          </button>
        ))}
      </div>
    </section>
  );
}

/**
 * The top-down view of what we're learning. One row per topic: what it is, what it's for,
 * where it lives in the source, the state, and the one action that matters — check me on
 * this. The action is VISIBLE, not hover-revealed, because a surface whose verbs only
 * appear on hover reads as having none (the mistake in take 1).
 *
 * §15's boundary: "quiz me on this topic" is a one-action launch of a known thing, so it
 * belongs here. Choosing the aid kind, the length, or a custom scope is a form — that is
 * Control, and it is not on this page.
 */
function Learning() {
  return (
    <section className="mb-8">
      <SectionLabel>What we&apos;re learning</SectionLabel>
      {TOPICS.length === 0 ? (
        <Empty>
          No topics yet. Ask the tutor to read the source&apos;s outline and propose a way
          through it — then cut what you already know.
        </Empty>
      ) : (
        <div className="-mx-2">
          {TOPICS.map((t) => (
            <div key={t.id} className="rounded-card px-2 py-2 transition-colors hover:bg-raise">
              <div className="flex items-baseline gap-3">
                <button type="button" className="min-w-0 flex-1 text-left">
                  <span className="block text-[13px] text-ink">{t.title}</span>
                  <span className="mt-0.5 block text-[12px] leading-snug text-ink-3">
                    {t.objective}
                  </span>
                </button>
                <TopicState state={t.state} />
                <span className="shrink-0 font-mono text-[10px] text-ink-4">{t.span}</span>
                <button
                  type="button"
                  className="shrink-0 rounded-chip px-1.5 py-0.5 font-mono text-[10px] text-ink-4 transition-colors hover:bg-card hover:text-ink"
                >
                  quiz me
                </button>
              </div>
              {t.aid ? (
                <button
                  type="button"
                  className="ml-4 mt-1 flex items-baseline gap-2 text-left text-[12px] text-ink-2 hover:text-ink"
                >
                  <Boxes className="size-3 shrink-0 text-ink-4" />
                  {t.aid.title}
                  <span className="font-mono text-[10px] text-ink-4">{t.aid.kindLabel}</span>
                </button>
              ) : null}
            </div>
          ))}
        </div>
      )}

      {SUGGESTED.length ? (
        <div className="mt-4">
          <p className="mb-1.5 font-mono text-[10px] uppercase tracking-[0.5px] text-ink-4">
            the tutor suggests
          </p>
          <div className="-mx-2">
            {SUGGESTED.map((sg) => (
              <div
                key={sg.id}
                className="flex items-baseline gap-3 rounded-card px-2 py-1.5 transition-colors hover:bg-raise"
              >
                <span className="min-w-0 flex-1">
                  <span className="text-[13px] text-ink-2">{sg.title}</span>
                  <span className="ml-2 text-[12px] text-ink-4">{sg.why}</span>
                </span>
                <span className="shrink-0 font-mono text-[10px] text-ink-4">{sg.span}</span>
                <button
                  type="button"
                  className="shrink-0 rounded-chip px-1.5 py-0.5 font-mono text-[10px] text-ink-4 transition-colors hover:bg-card hover:text-ink"
                >
                  add
                </button>
              </div>
            ))}
          </div>
        </div>
      ) : null}
    </section>
  );
}

/**
 * State rendered from what happened, and silent when there is nothing to say. `untouched`
 * is the only case that gets no ink at all — an empty state is not a status worth a badge.
 */
function TopicState({ state }: { state: Topic["state"] }) {
  if (state.kind === "untouched") return null;
  if (state.kind === "reading")
    return <span className="shrink-0 font-mono text-[10px] text-ink-4">reading</span>;
  if (state.kind === "checked")
    return (
      <span className="shrink-0 font-mono text-[10px] text-ink-4">
        {state.got}/{state.of}
      </span>
    );
  return (
    <span className="shrink-0 truncate font-mono text-[10px] text-amber" title={state.missed}>
      missed {state.missed}
    </span>
  );
}

/**
 * The interaction widgets. Whole-folder actions, one click each, no form — the agent takes
 * everything in this notebook as its context, so none of these need arguments. Anything
 * that would need a parameter belongs in the dock as a sentence, not here as a control.
 */
function AskTheTutor() {
  const actions = [
    { id: "quiz-shaky", label: "Quiz me on what's shaky" },
    { id: "suggest", label: "What should I learn next?" },
    { id: "where", label: "Where am I losing the thread?" },
  ];
  return (
    <section className="mb-8">
      <SectionLabel>Ask the tutor</SectionLabel>
      <div className="-mx-2 flex flex-wrap gap-1">
        {actions.map((a) => (
          <button
            key={a.id}
            type="button"
            className="rounded-chip px-2 py-1 text-[12px] text-ink-2 transition-colors hover:bg-raise hover:text-ink"
          >
            {a.label}
          </button>
        ))}
      </div>
    </section>
  );
}

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <h2 className="mb-2 font-mono text-[10.5px] uppercase tracking-[0.5px] text-ink-4">
      {children}
    </h2>
  );
}

function Empty({ children }: { children: React.ReactNode }) {
  return (
    <p className="rounded-card border border-dashed border-line-2 px-3 py-3 text-[12px] leading-relaxed text-ink-4">
      {children}
    </p>
  );
}
