import {
  BookOpen,
  Boxes,
  Check,
  ChevronRight,
  FileText,
  GraduationCap,
  Layers,
  ListChecks,
  Network,
  Plus,
  Scissors,
  Sparkles,
  SplitSquareHorizontal,
  Trash2,
  X,
} from "lucide-react";
import { Eyebrow } from "@/components/ui/eyebrow";
import { Icon } from "@/components/ui/icon";
import { useMemo, useState } from "react";

import { cn } from "@/lib/utils";

// Dev-only spike (/spike/tutor) — the Tutor surface on mock data, no auth.
//
// Prototypes design/TUTOR_PRD.md rev 3, whose central claim is a LAYOUT claim:
// Tutor is a learning copilot, not a teacher. The SOURCE teaches, so the reader is the
// primary pane; Aladin supplies the three things a PDF cannot — a path through it, a check
// that you got it, and a way to poke the ideas.
//
// What this spike is arguing, concretely:
//   1. The plan is a syllabus you ARGUE WITH (§4a) — editable, scoped to real spans,
//      revisable forever. Not a wizard, not a chat transcript.
//   2. Aids are REQUESTED from a catalogue (§4b), not authored automatically. The user
//      decides; the agent builds it accurately from retrieved text.
//   3. The default answer to "explain this" is a POINTER (§7), not a restatement.
//   4. Scope is a span OR a concept — "partial derivatives" is an idea threaded through
//      the source, not a page range.
//
// Mock content is the real paper ingested into prod (361 pages, 527 isolate_formula
// regions), so the formulas and page refs read like the actual corpus.

// ── scenarios (the use cases this surface has to carry) ──────────────────────────────
type Scenario = "plan" | "read" | "aid";

const SCENARIOS: { id: Scenario; label: string; blurb: string }[] = [
  { id: "plan", label: "1 · Plan it together", blurb: "just ingested — the agent proposes, you edit" },
  { id: "read", label: "2 · Read, and ask", blurb: "the source teaches; you ask for what would help" },
  { id: "aid", label: "3 · An aid, built", blurb: "requested, grounded, open beside the source" },
];

// ── mock source ──────────────────────────────────────────────────────────────────────
const SOURCE = {
  title: "Option Strategies & Payoff Algebra",
  filename: "ssrn_id3453295.pdf",
  pages: 361,
  regions: 6147,
  formulas: 527,
};

type PlanItem = {
  id: string;
  title: string;
  objective: string;
  span: string;
  pages: number;
  status: "planned" | "authored" | "learned";
  aids: { kind: string; label: string }[];
};

const INITIAL_PLAN: PlanItem[] = [
  {
    id: "i1",
    title: "Payoff algebra of the basic legs",
    objective: "Write the payoff of a call, put and forward from first principles.",
    span: "§2.1–2.4 · pp. 41–68",
    pages: 27,
    status: "learned",
    aids: [{ kind: "flashcards", label: "Flashcards · 18 terms" }],
  },
  {
    id: "i2",
    title: "Collars and spreads",
    objective: "Derive max gain / max loss for a collar from its legs, and price one.",
    span: "§4.2 · pp. 88–104",
    pages: 17,
    status: "planned",
    aids: [],
  },
  {
    id: "i3",
    title: "Greeks as partial derivatives",
    objective: "Read each greek as a partial derivative and say what it measures.",
    span: "§6.1–6.3 · pp. 152–197",
    pages: 46,
    status: "planned",
    aids: [],
  },
  {
    id: "i4",
    title: "Volatility surface fitting",
    objective: "Explain why a single vol number cannot price a whole book.",
    span: "§9 · pp. 244–301",
    pages: 58,
    status: "planned",
    aids: [],
  },
];

// The reader's content — page text with typed regions, as the ingest engine stores them.
const PAGE = {
  n: 94,
  section: "§4.2 Collars",
  blocks: [
    { kind: "plain", text: "A collar caps both tails of a long position. The holder buys a put struck at K₁ and sells a call struck at K₂ > K₁, financing part or all of the protection with the call premium." },
    { kind: "formula", text: "f\u209C = min(max(S\u209C, K\u2081), K\u2082) \u2212 S\u2080 \u2212 C", extracted: "fT = min(max(ST, K1), K2) - S0 - C", cite: "eq. 4.7" },
    { kind: "plain", text: "Because the payoff is bounded on both sides, the maximum gain and loss follow directly from the strikes and the net premium C:" },
    { kind: "formula", text: "L\u2098\u2090\u2093 = K\u2082 \u2212 K\u2081 \u2212 C", extracted: "Lmax = K2 \u2212K1 \u2212C", cite: "eq. 4.8" },
    { kind: "plain", text: "The structure is therefore fully described by three numbers. Practitioners often quote a collar by its width K₂ − K₁ rather than by its legs, since the width determines the whole risk profile once the underlying is fixed." },
  ],
};

// ── the aid catalogue (§4b / D-D) ────────────────────────────────────────────────────
const CATALOGUE = [
  { kind: "quiz", label: "Quiz", icon: ListChecks, what: "check a span or a concept" },
  { kind: "flashcards", label: "Flashcards", icon: Layers, what: "drill terms and results" },
  { kind: "visualizer", label: "Visualizer", icon: Boxes, what: "make an idea manipulable" },
  { kind: "stepthrough", label: "Step-through", icon: SplitSquareHorizontal, what: "advance a derivation" },
  { kind: "overview", label: "Overview", icon: BookOpen, what: "orient before reading" },
  { kind: "mindmap", label: "Mind map", icon: Network, what: "how the pieces relate" },
];

// ═══════════════════════════════════════════════════════════════════════════════════════

export function TutorSpike() {
  const [scenario, setScenario] = useState<Scenario>("plan");
  const [plan, setPlan] = useState<PlanItem[]>(INITIAL_PLAN);
  const [activeItem, setActiveItem] = useState("i2");
  const [tab, setTab] = useState<"source" | "aid">("source");
  const [planAccepted, setPlanAccepted] = useState(false);

  // Scenario switching seeds the state each case needs, so each is inspectable on its own.
  const pick = (s: Scenario) => {
    setScenario(s);
    setTab(s === "aid" ? "aid" : "source");
    setPlanAccepted(s !== "plan");
    setActiveItem(s === "aid" ? "i3" : "i2");
  };

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-bg text-ink">
      <SpikeBar scenario={scenario} onPick={pick} />
      <div className="flex min-h-0 flex-1">
        <PlanPane
          plan={plan}
          setPlan={setPlan}
          activeItem={activeItem}
          onPick={setActiveItem}
          proposing={scenario === "plan" && !planAccepted}
          onAccept={() => setPlanAccepted(true)}
        />
        <div className="flex min-w-0 flex-1 flex-col">
          <ReaderTabs tab={tab} setTab={setTab} hasAid={scenario === "aid"} />
          <div className="min-h-0 flex-1 overflow-auto">
            {tab === "source" ? <ReaderPane /> : <VisualizerAid />}
          </div>
        </div>
        <AskPane
          scenario={scenario}
          onBuild={() => {
            setScenario("aid");
            setTab("aid");
          }}
        />
      </div>
    </div>
  );
}

// ── the spike's own chrome (not part of the surface) ─────────────────────────────────
function SpikeBar({ scenario, onPick }: { scenario: Scenario; onPick: (s: Scenario) => void }) {
  const active = SCENARIOS.find((s) => s.id === scenario);
  return (
    <div className="flex shrink-0 items-center gap-3 border-b border-line bg-chrome px-4 py-2">
      <span className="flex items-center gap-1.5 font-display text-small uppercase tracking-wider text-ink-3">
        <Icon as={GraduationCap} size="inline" mark /> Tutor spike
      </span>
      <div className="flex items-center gap-1">
        {SCENARIOS.map((s) => (
          <button
            key={s.id}
            type="button"
            onClick={() => onPick(s.id)}
            className={cn(
              "rounded-chip px-2.5 py-1 text-small transition-colors",
              scenario === s.id
                ? "bg-amber-soft text-ink border border-amber-line"
                : "text-ink-3 hover:bg-raise hover:text-ink-2 border border-transparent",
            )}
          >
            {s.label}
          </button>
        ))}
      </div>
      <span className="truncate text-small text-ink-4">{active?.blurb}</span>
      <span className="ml-auto shrink-0 font-mono text-meta text-ink-4">
        {SOURCE.pages}p · {SOURCE.regions} regions · {SOURCE.formulas} formulas
      </span>
    </div>
  );
}

// ── LEFT: the plan — a syllabus you argue with (§4a) ─────────────────────────────────
function PlanPane({
  plan,
  setPlan,
  activeItem,
  onPick,
  proposing,
  onAccept,
}: {
  plan: PlanItem[];
  setPlan: (p: PlanItem[]) => void;
  activeItem: string;
  onPick: (id: string) => void;
  proposing: boolean;
  onAccept: () => void;
}) {
  const drop = (id: string) => setPlan(plan.filter((i) => i.id !== id));
  const split = (id: string) =>
    setPlan(
      plan.flatMap((i) =>
        i.id !== id
          ? [i]
          : [
              { ...i, id: `${i.id}a`, title: `${i.title} — part 1`, pages: Math.ceil(i.pages / 2) },
              { ...i, id: `${i.id}b`, title: `${i.title} — part 2`, pages: Math.floor(i.pages / 2), aids: [] },
            ],
      ),
    );

  return (
    <aside className="flex w-[340px] shrink-0 flex-col border-r border-line bg-panel">
      <div className="border-b border-line px-4 py-3">
        <div className="flex items-center gap-2">
          <Icon as={FileText} size="inline" mark className="shrink-0 text-ink-3" />
          <span className="truncate font-display text-body text-ink">{SOURCE.title}</span>
        </div>
        <p className="mt-1 truncate font-mono text-meta text-ink-4">{SOURCE.filename}</p>
        <p className="mt-2 text-small leading-relaxed text-ink-3">
          <span className="text-ink-4">Goal · </span>
          be able to price and risk-manage a collar, and read the greeks as derivatives
        </p>
      </div>

      {proposing ? (
        <div className="border-b border-amber-line bg-amber-soft px-4 py-2.5">
          <p className="flex items-start gap-1.5 text-small leading-relaxed text-ink-2">
            <Icon as={Sparkles} size="inline" mark className="mt-0.5 shrink-0 text-amber" />
            <span>
              I read the outline and drafted 4 items. <span className="text-ink-4">Edit freely — drop what you know, split what's too big.</span>
            </span>
          </p>
          <button
            type="button"
            onClick={onAccept}
            className="mt-2 rounded-chip border border-amber-line bg-amber px-2 py-1 text-meta font-semibold text-bg"
          >
            Looks right — start here
          </button>
        </div>
      ) : null}

      <div className="min-h-0 flex-1 overflow-auto px-2 py-2">
        {plan.map((item, idx) => (
          <PlanRow
            key={item.id}
            item={item}
            idx={idx + 1}
            active={item.id === activeItem}
            onPick={() => onPick(item.id)}
            onDrop={() => drop(item.id)}
            onSplit={() => split(item.id)}
          />
        ))}
        <button
          type="button"
          className="mt-2 flex w-full items-center gap-1.5 px-3 py-1 text-small text-ink-4 transition-colors hover:text-ink-2"
        >
          <Icon as={Plus} size="inline" mark /> Add an item
        </button>
      </div>

    </aside>
  );
}

function PlanRow({
  item,
  idx,
  active,
  onPick,
  onDrop,
  onSplit,
}: {
  item: PlanItem;
  idx: number;
  active: boolean;
  onPick: () => void;
  onDrop: () => void;
  onSplit: () => void;
}) {
  const big = item.pages > 40;
  return (
    <div
      className={cn(
        "group mb-px border-l-2 px-3 py-2 transition-colors",
        active ? "border-amber bg-raise" : "border-transparent hover:bg-raise",
      )}
    >
      <button type="button" onClick={onPick} className="w-full text-left">
        <div className="flex items-start gap-2">
          <span
            className={cn(
              "mt-px flex size-4 shrink-0 items-center justify-center rounded-chip font-mono text-meta",
              item.status === "learned" ? "bg-amber text-bg" : "bg-field text-ink-4",
            )}
          >
            {item.status === "learned" ? <Icon as={Check} size="inline" mark /> : idx}
          </span>
          <span className="min-w-0 flex-1">
            <span className="block text-small font-medium leading-snug text-ink">{item.title}</span>
            <span className="mt-0.5 block text-meta leading-snug text-ink-3">{item.objective}</span>
            <span className="mt-1 flex items-center gap-1.5 font-mono text-meta text-ink-4">
              {item.span}
              <span className={cn(big && "text-against")}>· {item.pages}p</span>
            </span>
          </span>
        </div>
      </button>

      {/* Honest about size (§4a): a fat item makes a bad aid, so say so and offer the fix. */}
      {big ? (
        <button
          type="button"
          onClick={onSplit}
          className="mt-1.5 flex items-center gap-1 rounded-chip border border-line-2 px-1.5 py-0.5 text-meta text-ink-3 hover:text-ink"
        >
          <Icon as={Scissors} size="inline" mark /> 46 pages is a lot — split it
        </button>
      ) : null}

      {item.aids.length ? (
        <div className="mt-1.5 flex flex-wrap gap-1">
          {item.aids.map((a) => (
            <span key={a.kind} className="rounded-chip bg-field px-1.5 py-0.5 font-mono text-meta text-ink-3">
              {a.label}
            </span>
          ))}
        </div>
      ) : null}

      <div className="mt-1.5 flex items-center gap-2 opacity-0 transition-opacity group-hover:opacity-100">
        <button type="button" onClick={onDrop} className="flex items-center gap-1 text-meta text-ink-4 hover:text-against">
          <Icon as={Trash2} size="inline" mark /> I know this
        </button>
      </div>
    </div>
  );
}

// ── CENTER: the reader is the PRIMARY pane (§13 rev 3) ───────────────────────────────
function ReaderTabs({
  tab,
  setTab,
  hasAid,
}: {
  tab: "source" | "aid";
  setTab: (t: "source" | "aid") => void;
  hasAid: boolean;
}) {
  return (
    <div className="flex shrink-0 items-center gap-1 border-b border-line bg-chrome px-2">
      <TabButton active={tab === "source"} onClick={() => setTab("source")} icon={FileText} label="Source" hint="p. 94" />
      {hasAid ? (
        <TabButton active={tab === "aid"} onClick={() => setTab("aid")} icon={Boxes} label="Collar payoff" hint="visualizer" closable />
      ) : null}
    </div>
  );
}

function TabButton({
  active,
  onClick,
  icon: Glyph,
  label,
  hint,
  closable,
}: {
  active: boolean;
  onClick: () => void;
  icon: typeof FileText;
  label: string;
  hint?: string;
  closable?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "-mb-px flex items-center gap-1.5 border-b-2 px-3 py-2 text-small transition-colors",
        active ? "border-amber text-ink" : "border-transparent text-ink-3 hover:text-ink-2",
      )}
    >
      <Icon as={Glyph} size="inline" mark />
      {label}
      {hint ? <span className="font-mono text-meta text-ink-4">{hint}</span> : null}
      {closable ? <Icon as={X} size="inline" mark className="text-ink-4 hover:text-ink" /> : null}
    </button>
  );
}

function ReaderPane() {
  return (
    <div className="mx-auto max-w-[68ch] px-8 py-8">
      <Eyebrow as="p" className="text-ink-4">
        {PAGE.section} · page {PAGE.n} of {SOURCE.pages}
      </Eyebrow>
      <div className="mt-5 space-y-4">
        {PAGE.blocks.map((b, i) =>
          b.kind === "formula" ? (
            <div key={i} className="group/f border-l-2 border-line-2 py-1 pl-4">
              {/* The source's OWN typeset math — the user is reading the paper, so nothing
                  here is transcribed. The flattened text the AGENT retrieves appears
                  underneath on hover: subscripts are lost, which is exactly why an aid that
                  MANIPULATES a formula needs vision-to-LaTeX (D-J) and the reader never does. */}
              <div className="flex items-baseline justify-between gap-3">
                <code className="font-mono text-body text-ink">{b.text}</code>
                <span className="shrink-0 font-mono text-meta text-ink-4">{b.cite}</span>
              </div>
              <code className="mt-1 block truncate font-mono text-meta text-ink-4 opacity-0 transition-opacity group-hover/f:opacity-100">
                agent reads · {b.extracted}
              </code>
            </div>
          ) : (
            <p key={i} className="text-body leading-relaxed text-ink-2">
              {b.text}
            </p>
          ),
        )}
      </div>
    </div>
  );
}

// ── RIGHT: the ask — catalogue + pointer-first answers (§4b, §7) ─────────────────────
function AskPane({ scenario, onBuild }: { scenario: Scenario; onBuild: () => void }) {
  const [scope, setScope] = useState<"item" | "concept">("item");
  const [concept, setConcept] = useState("partial derivatives");

  return (
    <aside className="flex w-[380px] shrink-0 flex-col border-l border-line bg-panel">
      <div className="border-b border-line px-4 py-2.5">
        <span className="font-display text-small uppercase tracking-wider text-ink-3">Ask</span>
      </div>

      <div className="min-h-0 flex-1 space-y-3 overflow-auto p-4">
        {/* §7 — the default answer is a POINTER, not a restatement. */}
        <Exchange
          you="why is the collar's max loss K2 − K1 − C?"
          answer={
            <>
              <p className="text-small leading-relaxed text-ink-2">
                It falls out of eq. 4.8 — the payoff is bounded by both strikes, so the worst
                case is the width minus what you paid.
              </p>
              <Pointer page={94} label="§4.2 · eq. 4.8" quote="L_max = K_2 - K_1 - C" />
              <p className="mt-2 text-meta leading-relaxed text-ink-4">
                Pointing, not restating — a citation you can verify, and it cannot invent the
                material.
              </p>
            </>
          }
        />

        {scenario === "aid" ? (
          <Exchange
            you="build a visualizer for the collar payoff"
            answer={
              <>
                <p className="text-small leading-relaxed text-ink-2">
                  Built from eq. 4.7, retrieved from pp. 88–104 — not from what I know about
                  collars.
                </p>
                <div className="mt-2 flex items-center gap-1.5 rounded-chip border border-amber-line bg-amber-soft px-2 py-1 text-meta text-ink">
                  <Icon as={Boxes} size="inline" mark className="text-amber" /> Collar payoff
                  <span className="font-mono text-meta text-ink-3">· open in a tab</span>
                </div>
              </>
            }
          />
        ) : null}
      </div>

      {/* the catalogue — a fixed set, not an open canvas (D-D) */}
      <div className="shrink-0 border-t border-line p-3">
        <div className="mb-2 flex items-center gap-1 rounded-chip bg-field p-0.5">
          <ScopeTab active={scope === "item"} onClick={() => setScope("item")} label="This item" />
          <ScopeTab active={scope === "concept"} onClick={() => setScope("concept")} label="A concept" />
        </div>

        {scope === "item" ? (
          <p className="mb-2 truncate font-mono text-meta text-ink-4">Collars and spreads · pp. 88–104</p>
        ) : (
          <div className="mb-2">
            <input
              value={concept}
              onChange={(e) => setConcept(e.target.value)}
              className="w-full rounded-chip border border-line bg-field px-2 py-1 font-mono text-meta text-ink placeholder:text-ink-4 focus:border-amber-line focus:outline-none"
              placeholder="partial derivatives"
            />
            <p className="mt-1 text-meta leading-relaxed text-ink-4">
              Not a page range — <span className="text-ink-3">search_document</span> finds every
              region touching it, across the whole source.
            </p>
          </div>
        )}

        <div className="grid grid-cols-2 gap-1">
          {CATALOGUE.map((c) => (
            <button
              key={c.kind}
              type="button"
              onClick={onBuild}
              className="flex items-center gap-1.5 rounded-chip px-2 py-1.5 text-left transition-colors hover:bg-raise"
            >
              <c.icon className="size-3 shrink-0 text-ink-4" />
              <span className="truncate text-meta text-ink-2">{c.label}</span>
            </button>
          ))}
        </div>
        <p className="mt-2 text-meta leading-relaxed text-ink-4">
          You choose what gets built. Nothing is authored automatically.
        </p>
      </div>
    </aside>
  );
}

function ScopeTab({ active, onClick, label }: { active: boolean; onClick: () => void; label: string }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex-1 rounded-chip px-2 py-1 text-meta transition-colors",
        active ? "bg-raise text-ink" : "text-ink-4 hover:text-ink-2",
      )}
    >
      {label}
    </button>
  );
}

function Exchange({ you, answer }: { you: string; answer: React.ReactNode }) {
  return (
    <div>
      <p className="mb-1.5 flex items-start gap-1.5 text-small text-ink-3">
        <Icon as={ChevronRight} size="inline" mark className="mt-0.5 shrink-0 text-ink-4" />
        {you}
      </p>
      <div className="pl-4">{answer}</div>
    </div>
  );
}

function Pointer({ page, label, quote }: { page: number; label: string; quote: string }) {
  return (
    <button
      type="button"
      className="mt-2 flex w-full items-baseline gap-2 border-l-2 border-amber-line pl-2 text-left transition-colors hover:border-amber"
    >
      <code className="min-w-0 flex-1 truncate font-mono text-meta text-ink">{quote}</code>
      <span className="shrink-0 font-mono text-meta text-ink-4">{label} · p.{page} →</span>
    </button>
  );
}

// ── an actual aid: manipulable, which is the one thing prose can't be (§4b) ──────────
function VisualizerAid() {
  const [k1, setK1] = useState(90);
  const [k2, setK2] = useState(115);
  const s0 = 100;
  const premium = 2;

  // The y-scale is derived from the data, not fixed. A hardcoded scale silently clipped the
  // curve off-canvas once the put strike went far enough out — the chart looked fine and was
  // lying, which is the one thing a "manipulable model" must never do.
  const { path, zeroY, lo, hi } = useMemo(() => {
    const payoff = (s: number) => Math.min(Math.max(s, k1), k2) - s0 - premium; // eq. 4.7
    const xs: number[] = [];
    for (let s = 60; s <= 140; s += 1) xs.push(s);
    const ys = xs.map(payoff);
    const lo = Math.min(...ys, 0);
    const hi = Math.max(...ys, 0);
    const pad = (hi - lo) * 0.15 || 1;
    const toY = (v: number) => 180 - ((v - (lo - pad)) / (hi - lo + pad * 2)) * 160;
    return {
      path: xs.map((s, i) => `${(((s - 60) / 80) * 460 + 30).toFixed(1)},${toY(ys[i]).toFixed(1)}`).join(" "),
      zeroY: toY(0),
      lo,
      hi,
    };
  }, [k1, k2]);

  const maxGain = k2 - s0 - premium;
  const maxLoss = k1 - s0 - premium;

  return (
    <div className="mx-auto max-w-[68ch] px-8 py-8">
      <div className="flex items-baseline gap-2">
        <h2 className="font-display text-lead text-ink">Collar payoff</h2>
        <span className="font-mono text-meta text-ink-4">built from eq. 4.7 · pp. 88–104</span>
      </div>
      <p className="mt-1 text-small text-ink-3">
        Drag the strikes. This is the thing the paper cannot do — the formula is static on
        p. 94.
      </p>

      <div className="mt-5 rounded-card border border-line bg-card p-4">
        <svg viewBox="0 0 500 210" className="w-full" role="img" aria-label="Collar payoff diagram">
          {/* zero line — where the position breaks even */}
          <line x1="30" y1={zeroY} x2="490" y2={zeroY} className="stroke-line-2" strokeWidth="1" />
          <text x="30" y={zeroY - 4} className="fill-ink-4" style={{ fontSize: 8 }}>
            0
          </text>
          <line x1="30" y1="15" x2="30" y2="185" className="stroke-line-2" strokeWidth="1" />
          {[k1, k2].map((k, i) => (
            <g key={k}>
              <line
                x1={((k - 60) / 80) * 460 + 30}
                y1="15"
                x2={((k - 60) / 80) * 460 + 30}
                y2="185"
                className="stroke-line"
                strokeWidth="1"
                strokeDasharray="3 3"
              />
              <text
                x={((k - 60) / 80) * 460 + 30}
                y="197"
                textAnchor="middle"
                className="fill-ink-4"
                style={{ fontSize: 8 }}
              >
                {i === 0 ? "K₁" : "K₂"} {k}
              </text>
            </g>
          ))}
          <polyline points={path} fill="none" className="stroke-amber" strokeWidth="2" />
          <text x="490" y="207" textAnchor="end" className="fill-ink-4" style={{ fontSize: 8 }}>
            S_T →
          </text>
          <text x="30" y="12" className="fill-ink-4" style={{ fontSize: 8 }}>
            payoff {hi.toFixed(0)} … {lo.toFixed(0)}
          </text>
        </svg>

        <div className="mt-3 space-y-2">
          <Slider label="K₁ (put)" value={k1} min={70} max={99} onChange={setK1} />
          <Slider label="K₂ (call)" value={k2} min={101} max={135} onChange={setK2} />
        </div>

        <div className="mt-4 grid grid-cols-2 gap-2">
          <Stat label="max gain" value={`+${maxGain.toFixed(0)}`} tone="text-for" note="K₂ − S₀ − C" />
          <Stat label="max loss" value={maxLoss.toFixed(0)} tone="text-against" note="K₁ − S₀ − C" />
        </div>
      </div>

      <p className="mt-4 text-meta leading-relaxed text-ink-4">
        Grounded in the source — payoff, strikes and premium all from retrieved text.
      </p>
    </div>
  );
}

function Slider({
  label,
  value,
  min,
  max,
  onChange,
}: {
  label: string;
  value: number;
  min: number;
  max: number;
  onChange: (n: number) => void;
}) {
  return (
    <label className="flex items-center gap-3">
      <span className="w-16 shrink-0 font-mono text-meta text-ink-3">{label}</span>
      <input
        type="range"
        min={min}
        max={max}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="h-1 flex-1 accent-amber"
      />
      <span className="w-8 shrink-0 text-right font-mono text-meta text-ink">{value}</span>
    </label>
  );
}

function Stat({ label, value, tone, note }: { label: string; value: string; tone: string; note: string }) {
  return (
    <div>
      <Eyebrow as="p" className="text-ink-4">{label}</Eyebrow>
      <p className={cn("font-display text-lead", tone)}>{value}</p>
      <p className="font-mono text-meta text-ink-4">{note}</p>
    </div>
  );
}
