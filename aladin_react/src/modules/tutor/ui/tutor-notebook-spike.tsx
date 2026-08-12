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
 * reuse the workspace shell; a single column, a few labelled sections, no chrome):
 *
 *   what I want out of this  ·  what's in here  ·  the plan
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

type PlanItem = {
  id: string;
  title: string;
  objective: string;
  span: string;
  status: "planned" | "learned";
  aid?: { title: string; kindLabel: string };
};

const PLAN: PlanItem[] = [
  {
    id: "p1",
    title: "Payoff algebra of the basic legs",
    objective: "Write the payoff of a call, put and forward from first principles.",
    span: "pp. 41–68",
    status: "learned",
  },
  {
    id: "p2",
    title: "Collars and spreads",
    objective: "Derive max gain and max loss for a collar from its legs.",
    span: "pp. 88–104",
    status: "planned",
    aid: { title: "Collar payoff", kindLabel: "shard" },
  },
  {
    id: "p3",
    title: "Greeks as partial derivatives",
    objective: "Read each greek as a partial derivative and say what it measures.",
    span: "pp. 152–197",
    status: "planned",
  },
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
  return (
    <div className="mx-auto w-full max-w-[62rem] px-8 py-7">
      <header className="mb-7">
        <h1 className="font-display text-[24px] leading-tight text-ink">{NOTEBOOK.title}</h1>
        <div className="mt-2 flex flex-wrap items-center gap-x-4 font-mono text-[10.5px] uppercase tracking-[0.5px] text-ink-4">
          <span>1 source</span>
          <span>3 items</span>
        </div>
      </header>

      <Goal />
      <Material />
      <Plan />
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
 * The plan. Ordered items with a real span; an authored aid hangs off its item as a plain
 * child row, which is how the plan finds its aids without a catalogue anywhere on screen.
 *
 * Status is user-set and rendered by absence: a learned item is struck through and quiet.
 * There is no progress bar, because "3 of 7 items" is a number about the plan, not about
 * understanding, and inventing it is the thing research's Overview refuses to do.
 */
function Plan() {
  const empty = PLAN.length === 0;
  return (
    <section className="mb-8">
      <SectionLabel>The plan</SectionLabel>
      {empty ? (
        <Empty>
          Nothing planned yet. Ask the copilot to read the source's outline and draft a way
          through it — then cut what you already know.
        </Empty>
      ) : (
        <div className="-mx-2">
          {PLAN.map((item) => (
            <div key={item.id}>
              <button
                type="button"
                className="flex w-full items-baseline gap-3 rounded-card px-2 py-2 text-left transition-colors hover:bg-raise"
              >
                <span className="min-w-0 flex-1">
                  <span
                    className={cn(
                      "block text-[13px]",
                      item.status === "learned" ? "text-ink-4 line-through" : "text-ink",
                    )}
                  >
                    {item.title}
                  </span>
                  <span className="mt-0.5 block text-[12px] leading-snug text-ink-3">
                    {item.objective}
                  </span>
                </span>
                <span className="shrink-0 font-mono text-[10px] text-ink-4">{item.span}</span>
              </button>
              {item.aid ? (
                <button
                  type="button"
                  className="ml-6 flex items-baseline gap-3 rounded-card px-2 py-1 text-left transition-colors hover:bg-raise"
                >
                  <Boxes className="size-3 shrink-0 text-ink-4" />
                  <span className="text-[12px] text-ink-2">{item.aid.title}</span>
                  <span className="font-mono text-[10px] text-ink-4">{item.aid.kindLabel}</span>
                </button>
              ) : null}
            </div>
          ))}
        </div>
      )}
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
