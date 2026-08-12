import { useState } from "react";
import {
  ChevronDown,
  FlaskConical,
  GraduationCap,
  Paperclip,
  StickyNote,
  Table2,
  type LucideIcon,
} from "lucide-react";

import { cn } from "@/lib/utils";

/**
 * Dev-only spike (/spike/tutor-purpose) — how you tell a strategy-development folder from a
 * learning folder, without a second kind.
 *
 * The user's ask: keep the research folder, but be able to tell which sort it is.
 *
 * ┌─ SCHEMA: NOT DECIDED. Do not implement from this file. ────────────────────────────┐
 * │ This spike originally argued for a `purpose` column on research_strategies:        │
 * │                                                                                    │
 * │     ALTER TABLE research_strategies                                                │
 * │       ADD COLUMN purpose text NOT NULL DEFAULT 'strategy' ...                      │
 * │                                                                                    │
 * │ REJECTED by the user: "mixing table responsibilities like that will bite later."    │
 * │ Correct, and the tell was already visible — a learning row would carry five         │
 * │ strategy columns it can never use (exec_mode, run_state, manifest, code_hash,       │
 * │ universe), and a table named research_strategies would hold rows that are not       │
 * │ strategies. Every later reader has to know which columns are live for which         │
 * │ purpose, and nothing in the schema tells them.                                      │
 * │                                                                                    │
 * │ Options still open, none chosen:                                                    │
 * │   B. a neutral base row (discriminator + the shared prose region) with              │
 * │      research_strategies and learning_folders each 1:1 off IT. Clean split of       │
 * │      responsibilities; costs moving existing columns.                                │
 * │   C. kind='learning' with its own extension. Cleanest separation, pays the          │
 * │      ~31-file cost of a fourth kind.                                                │
 * │   D. extension-row PRESENCE as the discriminator, both under kind='research'.       │
 * │      No new column, but then "research" names two things and reads need two joins.  │
 * └────────────────────────────────────────────────────────────────────────────────────┘
 *
 * What this spike still establishes, and what survives any of B/C/D: the READ MODEL. One
 * discriminator drives the glyph and label in the tree, which Overview sections render,
 * which views the tab group offers, and which system prompt the copilot uses — and it does
 * NOT fork the route, the tree, or the Overview component. That is a UI finding, independent
 * of where the field physically lives.
 *
 * The shape argument that motivated all of this is unaffected: a learning folder has no
 * folder-level state of its own (ingest status belongs to a source, build state to a shard),
 * so it fails 00037:10's test for a kind — "folders don't have state" — while matching
 * research's SHAPE exactly: a container with a prose region, gathered material, typed slots.
 * Whether that similarity is best expressed as one table, two, or two kinds is the open part.
 */

type Purpose = "strategy" | "learning";

type Folder = {
  id: string;
  title: string;
  purpose: Purpose;
  /** strategy only — the state that justified the kind in the first place */
  runState?: "idle" | "running" | "armed";
  /** learning only — what it unblocks; the reason it exists next to the strategy */
  backs?: string;
};

const TREE: Folder[] = [
  { id: "f1", title: "mean-reversion v2", purpose: "strategy", runState: "idle" },
  { id: "f2", title: "Cointegration & stationarity", purpose: "learning", backs: "mean-reversion v2" },
];

const GLYPH: Record<Purpose, LucideIcon> = { strategy: FlaskConical, learning: GraduationCap };

export function TutorPurposeSpike() {
  const [selected, setSelected] = useState("f2");
  const folder = TREE.find((f) => f.id === selected)!;

  return (
    <div className="flex h-screen overflow-hidden bg-bg text-ink">
      {/* the real browser pane, abbreviated — one tree, both purposes, siblings */}
      <aside className="w-[280px] shrink-0 border-r border-line bg-panel py-3">
        <div className="flex items-center gap-1 px-3 py-1 text-[12px] text-ink-3">
          <ChevronDown className="size-3 text-ink-4" />
          Pairs trading
        </div>
        {TREE.map((f) => {
          const Icon = GLYPH[f.purpose];
          return (
            <button
              key={f.id}
              type="button"
              onClick={() => setSelected(f.id)}
              className={cn(
                "flex w-full items-center gap-2 py-1.5 pl-7 pr-3 text-left transition-colors",
                f.id === selected ? "bg-raise" : "hover:bg-raise",
              )}
            >
              <Icon className="size-3.5 shrink-0 text-ink-4" />
              <span className="min-w-0 flex-1 truncate text-[12px] text-ink-2">{f.title}</span>
              {/* the tell, and it is deliberately quiet: you read it when you look, it does
                  not compete for attention with the run dot */}
              <span className="shrink-0 font-mono text-[9px] uppercase tracking-wider text-ink-4">
                {f.purpose === "strategy" ? "strat" : "learn"}
              </span>
              {f.runState === "running" ? <span className="size-1.5 rounded-chip bg-amber" /> : null}
            </button>
          );
        })}
      </aside>

      <div className="min-h-0 flex-1 overflow-auto">
        <Overview folder={folder} />
      </div>
    </div>
  );
}

/**
 * ONE Overview component. The purpose selects which sections render — it does not fork the
 * file. That constraint is the whole point: if this ever becomes two components, the
 * discriminator has failed and a second kind would have been the honest thing to build.
 */
function Overview({ folder }: { folder: Folder }) {
  const learning = folder.purpose === "learning";
  return (
    <div className="mx-auto w-full max-w-[62rem] px-8 py-7">
      <header className="mb-7">
        <h1 className="font-display text-[24px] leading-tight text-ink">{folder.title}</h1>
        <div className="mt-2 flex flex-wrap items-center gap-x-4 font-mono text-[10.5px] uppercase tracking-[0.5px] text-ink-4">
          <span>{folder.purpose}</span>
          {learning ? (
            <span>backs {folder.backs}</span>
          ) : (
            <>
              <span>event-driven</span>
              <span>{folder.runState}</span>
            </>
          )}
        </div>
      </header>

      {/* Shared: the prose region. Same slot, different question — which is the clearest
          evidence these are one shape and not two. */}
      <Section label={learning ? "What I want out of this" : "Hypothesis"}>
        <p className="font-display text-[15px] leading-relaxed text-ink">
          {learning
            ? "Understand cointegration well enough to defend the pair-selection rule — and to know when failing to reject a unit root is not evidence of anything."
            : "Pairs selected on cointegration over a 2-year window mean-revert within 5 days often enough to beat costs."}
        </p>
      </Section>

      {learning ? (
        <Section label="What we're learning">
          {[
            { t: "Unit roots and the ADF test", s: "Hamilton 571–590", state: "5/5" },
            { t: "Cointegration, two-step", s: "E&G 251–258", state: "reading" },
            { t: "Lag selection without p-hacking", s: "Hamilton 591–600", state: "" },
          ].map((r) => (
            <Row key={r.t} title={r.t} right={r.s} mid={r.state} />
          ))}
        </Section>
      ) : (
        <Section label="Strategy code">
          <Row title="mean_reversion/" right="authored · no runs yet" />
        </Section>
      )}

      {/* Shared: gathered material. The kinds differ by what you put in them, not by schema —
          a learning folder fills up with canvases and study tables, a strategy with code and
          notes, and both are just artifacts in the same tree. */}
      <Section label="What's in here">
        {(learning
          ? [
              { t: "Hamilton ch. 19", k: "pdf", i: Paperclip },
              { t: "Engle & Granger 1987", k: "pdf", i: Paperclip },
              { t: "Stationarity canvas", k: "canvas", i: StickyNote },
              { t: "This week", k: "study", i: Table2 },
            ]
          : [
              { t: "Why cointegration over correlation", k: "note", i: StickyNote },
              { t: "Universe screen", k: "note", i: StickyNote },
            ]
        ).map((m) => (
          <Row key={m.t} title={m.t} right={m.k} icon={m.i} />
        ))}
      </Section>
    </div>
  );
}

function Section({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <section className="mb-8">
      <h2 className="mb-2 font-mono text-[10.5px] uppercase tracking-[0.5px] text-ink-4">{label}</h2>
      <div className="-mx-2">{children}</div>
    </section>
  );
}

function Row({
  title,
  right,
  mid,
  icon: Icon,
}: {
  title: string;
  right: string;
  mid?: string;
  icon?: LucideIcon;
}) {
  return (
    <button
      type="button"
      className="flex w-full items-baseline gap-3 rounded-card px-2 py-1.5 text-left transition-colors hover:bg-raise"
    >
      {Icon ? <Icon className="mt-0.5 size-3.5 shrink-0 text-ink-4" /> : null}
      <span className="min-w-0 flex-1 truncate text-[13px] text-ink-2">{title}</span>
      {mid ? <span className="shrink-0 font-mono text-[10px] text-ink-4">{mid}</span> : null}
      <span className="shrink-0 font-mono text-[10px] text-ink-4">{right}</span>
    </button>
  );
}
