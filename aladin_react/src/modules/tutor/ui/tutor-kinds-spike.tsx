import { useRef, useState } from "react";
import { Eyebrow } from "@/components/ui/eyebrow";
import { Icon } from "@/components/ui/icon";
import { Boxes, GripVertical, Paperclip, Plus, StickyNote, Table2 } from "lucide-react";

import { cn } from "@/lib/utils";

/**
 * Dev-only spike (/spike/tutor-kinds) — attempt 5, and the one that follows the user's own
 * model rather than inventing a surface.
 *
 * The model, stated by the user: "all kinds of artifact surface show up as tabs and the
 * folder acts as a grouper... a canvas tab you use to visually arrange notes and annotate,
 * or a generated pomodoro table to plan and study. All that is context for your agents, and
 * the interaction is just one artifact you open beside your learning material" — with
 * side-by-side being tab switching, not a split view.
 *
 * What that means, and why the previous four attempts were wrong:
 *   · There is NO Tutor surface. The folder is a grouper, which folders already are.
 *   · There is no Overview control plane. Takes 3 and 4 built one; it isn't the product.
 *   · The product is NEW ARTIFACT KINDS, each of which opens as a tab through the switch
 *     that already exists in work-pane-ui.tsx:96-134 (note · app · link · voice · file).
 *   · Everything in the folder is the agent's context, so no kind needs its own chat.
 *
 * So this spike prototypes the two kinds the user named, in the tab strip they'd really
 * live in. It deliberately does NOT re-render the PDF — the source tab is a stub, because
 * DocumentViewerUI already ships and re-implementing it was a mistake in takes 1 and 2.
 *
 *   canvas  — arrange excerpts and your own notes in space; annotate; spatial thinking that
 *             a linear page cannot do. Excerpts carry their page cite, so the canvas stays
 *             traceable to the source.
 *   study   — a session table the agent generates from the plan: what to do, in what order,
 *             in blocks. Plans effort rather than explaining content.
 */

// ── the folder is just a grouper; these are its tabs ────────────────────────────────
type TabId = "source" | "source2" | "canvas" | "study";

/**
 * The real use case, from the user: while researching an algo strategy you spin up a
 * SEPARATE folder to catch up on the math and stats underneath it. So a learning folder is
 * about a SUBJECT, not a document — several sources, sitting beside a research folder in the
 * same tree. That kills the one-source-per-folder assumption the earlier takes carried, and
 * with it the idea that this surface is "a reader plus helpers".
 */
const TABS: { id: TabId; label: string; icon: typeof Paperclip; hint: string }[] = [
  { id: "source", label: "Hamilton ch. 19", icon: Paperclip, hint: "pdf" },
  { id: "source2", label: "Engle & Granger 1987", icon: Paperclip, hint: "pdf" },
  { id: "canvas", label: "Stationarity canvas", icon: StickyNote, hint: "canvas" },
  { id: "study", label: "This week", icon: Table2, hint: "study" },
];

export function TutorKindsSpike() {
  const [tab, setTab] = useState<TabId>("canvas");
  return (
    <div className="flex h-screen flex-col overflow-hidden bg-bg text-ink">
      {/* the folder, as a breadcrumb — it groups, it is not a destination */}
      <div className="flex shrink-0 items-center gap-1.5 px-4 pt-2 font-mono text-meta text-ink-4">
        <span>Pairs trading</span>
        <span>/</span>
        <span className="text-ink-3">Cointegration &amp; stationarity</span>
        {/* The sibling that motivated it. A learning folder earns its place by being next to
            the research it unblocks — same tree, same shell, no special surface either. */}
        <span className="ml-3 text-ink-4">backs</span>
        <span className="text-ink-4">▸ mean-reversion v2</span>
        <span className="rounded-chip bg-field px-1 text-meta uppercase tracking-wider">research</span>
      </div>

      <div className="flex shrink-0 items-center gap-1 border-b border-line px-2">
        {TABS.map((t) => (
          <button
            key={t.id}
            type="button"
            onClick={() => setTab(t.id)}
            className={cn(
              "-mb-px flex items-center gap-1.5 border-b-2 px-3 py-2 text-small transition-colors",
              tab === t.id ? "border-amber text-ink" : "border-transparent text-ink-3 hover:text-ink-2",
            )}
          >
            <t.icon className="size-3.5" />
            {t.label}
            <span className="font-mono text-meta text-ink-4">{t.hint}</span>
          </button>
        ))}
      </div>

      <div className="min-h-0 flex-1 overflow-hidden">
        {tab === "canvas" ? <CanvasKind /> : tab === "study" ? <StudyKind /> : <SourceStub />}
      </div>
    </div>
  );
}

/**
 * Deliberately a stub. The real thing is DocumentViewerUI, already wired for kind==="file"
 * (work-pane-ui.tsx:115). Takes 1 and 2 both hand-rolled a reader over mock text and both
 * were wrong to: the shipped viewer shows PAGES because flattening a paper's tables and
 * figures destroys them.
 */
function SourceStub() {
  return (
    <div className="flex h-full items-center justify-center">
      <p className="max-w-[40ch] text-center text-small leading-relaxed text-ink-4">
        The source opens in the shipped document viewer. Not re-rendered here — that was the
        mistake in the first two attempts.
      </p>
    </div>
  );
}

// ── kind 1: canvas ──────────────────────────────────────────────────────────────────
type Card = {
  id: string;
  x: number;
  y: number;
  w: number;
  kind: "excerpt" | "note" | "formula";
  body: string;
  cite?: string;
};

const CARDS: Card[] = [
  {
    id: "c1",
    x: 40,
    y: 30,
    w: 260,
    kind: "excerpt",
    body: "A series is I(1) if it must be differenced once to become stationary. Two I(1) series are cointegrated if some linear combination of them is I(0).",
    cite: "Hamilton p. 571",
  },
  { id: "c2", x: 350, y: 40, w: 230, kind: "formula", body: "Δyₜ = α + βyₜ₋₁ + εₜ ,  H₀: β = 0", cite: "E&G eq. 3" },
  {
    id: "c3",
    x: 60,
    y: 210,
    w: 240,
    kind: "note",
    body: "So the ADF test is just a regression with a null of a unit root. Failing to reject ≠ proving stationarity.",
  },
  {
    id: "c4",
    x: 360,
    y: 190,
    w: 230,
    kind: "note",
    body: "Still unsure: how do I pick the lag order without p-hacking the spread into looking mean-reverting?",
  },
];

/**
 * Arrange excerpts and your own notes in space. The thing a linear page cannot do — and the
 * reason it belongs in this folder is that excerpts keep their page cite, so a canvas stays
 * traceable to the source instead of becoming free-floating paraphrase.
 */
function CanvasKind() {
  const [cards, setCards] = useState(CARDS);
  const drag = useRef<{ id: string; dx: number; dy: number } | null>(null);
  const host = useRef<HTMLDivElement>(null);

  const onDown = (e: React.MouseEvent, c: Card) => {
    const box = host.current?.getBoundingClientRect();
    if (!box) return;
    drag.current = { id: c.id, dx: e.clientX - box.left - c.x, dy: e.clientY - box.top - c.y };
  };
  const onMove = (e: React.MouseEvent) => {
    const d = drag.current;
    const box = host.current?.getBoundingClientRect();
    if (!d || !box) return;
    setCards((prev) =>
      prev.map((c) =>
        c.id === d.id
          ? { ...c, x: Math.max(0, e.clientX - box.left - d.dx), y: Math.max(0, e.clientY - box.top - d.dy) }
          : c,
      ),
    );
  };

  return (
    <div
      ref={host}
      onMouseMove={onMove}
      onMouseUp={() => (drag.current = null)}
      onMouseLeave={() => (drag.current = null)}
      className="relative h-full overflow-auto"
      style={{
        backgroundImage:
          "radial-gradient(circle, color-mix(in oklab, currentColor 12%, transparent) 1px, transparent 1px)",
        backgroundSize: "22px 22px",
        color: "var(--color-ink-4)",
      }}
    >
      {cards.map((c) => (
        <div
          key={c.id}
          style={{ left: c.x, top: c.y, width: c.w }}
          className={cn(
            "absolute rounded-card border bg-card",
            c.kind === "excerpt" ? "border-line-2" : c.kind === "formula" ? "border-line-2" : "border-line",
          )}
        >
          <div
            onMouseDown={(e) => onDown(e, c)}
            className="flex cursor-grab items-center gap-1 px-2 py-1 active:cursor-grabbing"
          >
            <Icon as={GripVertical} size="inline" mark className="text-ink-4" />
            <Eyebrow as="span" className="text-ink-4">{c.kind}</Eyebrow>
            {c.cite ? (
              <span className="ml-auto font-mono text-meta text-ink-4">{c.cite}</span>
            ) : null}
          </div>
          <p
            className={cn(
              "px-2.5 pb-2.5 leading-relaxed text-ink-2",
              c.kind === "formula" ? "font-mono text-body text-ink" : "text-small",
            )}
          >
            {c.body}
          </p>
        </div>
      ))}

      <button
        type="button"
        className="absolute bottom-4 right-4 flex items-center gap-1.5 rounded-chip bg-raise px-2.5 py-1.5 text-meta text-ink-2 shadow-modal transition-colors hover:text-ink"
      >
        <Icon as={Plus} size="inline" mark /> note
      </button>
    </div>
  );
}

// ── kind 2: study table ─────────────────────────────────────────────────────────────
type Block = { id: string; when: string; topic: string; task: string; span: string; done: boolean };

const BLOCKS: Block[] = [
  { id: "b1", when: "Mon · 2 × 25", topic: "Unit roots", task: "Work the ADF regression by hand on one pair", span: "Hamilton 571–590", done: true },
  { id: "b2", when: "Tue · 1 × 25", topic: "Unit roots", task: "Answer: why is failing to reject not proof?", span: "Hamilton p. 578", done: false },
  { id: "b3", when: "Wed · 3 × 25", topic: "Cointegration", task: "Read E&G §2, write the two-step procedure", span: "E&G 251–258", done: false },
  { id: "b4", when: "Fri · 2 × 25", topic: "Lag selection", task: "AIC vs BIC on the spread — decide a rule before backtesting", span: "Hamilton 591–600", done: false },
];

/**
 * A session table the agent generates from the plan and what's shaky. This kind plans
 * EFFORT, not content — which is why it can be generated safely: it makes no claim about
 * the material, so there is nothing here to hallucinate.
 */
function StudyKind() {
  const [blocks, setBlocks] = useState(BLOCKS);
  const toggle = (id: string) =>
    setBlocks((prev) => prev.map((b) => (b.id === id ? { ...b, done: !b.done } : b)));

  return (
    <div className="h-full overflow-auto">
      <div className="mx-auto w-full max-w-[62rem] px-8 py-7">
        <h1 className="font-display text-title text-ink">This week</h1>
        <Eyebrow as="p" className="mt-1 text-ink-4">
          8 × 25 min · what mean-reversion v2 needs you to actually understand
        </Eyebrow>

        <div className="-mx-2 mt-5">
          {blocks.map((b) => (
            <div
              key={b.id}
              className="flex items-baseline gap-3 rounded-card px-2 py-2 transition-colors hover:bg-raise"
            >
              <button
                type="button"
                onClick={() => toggle(b.id)}
                className={cn(
                  "mt-0.5 flex size-3.5 shrink-0 items-center justify-center rounded-chip border",
                  b.done ? "border-amber bg-amber" : "border-line-2",
                )}
                aria-label={b.done ? "done" : "not done"}
              />
              <span className="w-24 shrink-0 font-mono text-meta text-ink-4">{b.when}</span>
              <span className="min-w-0 flex-1">
                <span className={cn("block text-body", b.done ? "text-ink-4 line-through" : "text-ink")}>
                  {b.task}
                </span>
                <span className="mt-0.5 block text-meta text-ink-3">{b.topic}</span>
              </span>
              <span className="shrink-0 font-mono text-meta text-ink-4">{b.span}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
