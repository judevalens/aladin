import { Boxes, Layers, ListChecks, Quote, X } from "lucide-react";
import { Eyebrow } from "@/components/ui/eyebrow";
import { Icon } from "@/components/ui/icon";
import { useMemo, useRef, useState } from "react";

import { cn } from "@/lib/utils";

// Dev-only spike (/spike/tutor-read) — a SECOND, deliberately different take on
// design/TUTOR_PRD.md rev 3. Compare with /spike/tutor (three-pane workspace).
//
// The premise the first attempt got wrong: rev 3 says THE SOURCE TEACHES, and then the
// three-pane layout gave the source a third of the screen and surrounded it with permanent
// chrome. A reader that has to share space with two panels is not primary.
//
// So this version has NO PANES. It is one document, read at full measure, and every
// Aladin affordance is SUMMONED from the text and then goes away:
//
//   · select a passage      → a small action bar at the selection
//   · ask                   → the answer lands INLINE, anchored where you asked
//   · request an aid        → it embeds in the flow, right at the formula it explains
//   · the plan              → a strip you can open, not a pane you live beside
//
// The bet: reading is the activity; everything else is an interruption that should be
// invited, land where you are, and leave no residue.

const SOURCE = { title: "Option Strategies & Payoff Algebra", pages: 361 };

const PLAN = [
  { n: 1, title: "Payoff algebra of the basic legs", span: "pp. 41–68", done: true },
  { n: 2, title: "Collars and spreads", span: "pp. 88–104", done: false, current: true },
  { n: 3, title: "Greeks as partial derivatives", span: "pp. 152–197", done: false },
  { n: 4, title: "Volatility surface fitting", span: "pp. 244–301", done: false },
];

type Block =
  | { id: string; kind: "prose"; text: string }
  | { id: string; kind: "formula"; text: string; extracted: string; cite: string };

const BLOCKS: Block[] = [
  {
    id: "b1",
    kind: "prose",
    text: "A collar caps both tails of a long position. The holder buys a put struck at K₁ and sells a call struck at K₂ > K₁, financing part or all of the protection with the call premium.",
  },
  {
    id: "b2",
    kind: "formula",
    text: "fₜ = min(max(Sₜ, K₁), K₂) − S₀ − C",
    extracted: "fT = min(max(ST, K1), K2) - S0 - C",
    cite: "eq. 4.7",
  },
  {
    id: "b3",
    kind: "prose",
    text: "Because the payoff is bounded on both sides, the maximum gain and loss follow directly from the strikes and the net premium C:",
  },
  { id: "b4", kind: "formula", text: "Lₘₐₓ = K₂ − K₁ − C", extracted: "Lmax = K2 −K1 −C", cite: "eq. 4.8" },
  {
    id: "b5",
    kind: "prose",
    text: "The structure is therefore fully described by three numbers. Practitioners often quote a collar by its width K₂ − K₁ rather than by its legs, since the width determines the whole risk profile once the underlying is fixed.",
  },
  {
    id: "b6",
    kind: "prose",
    text: "This bounded profile is what makes collars attractive to holders of concentrated positions who cannot sell: the downside is capped at a known level, and the cost of that cap is a known ceiling on the upside.",
  },
];

// What the selection bar offers. Deliberately short — the catalogue lives in the plan,
// but at the point of confusion you want the two or three things you'd actually reach for.
const ACTIONS = [
  { id: "explain", label: "Where's this from?", icon: Quote },
  { id: "quiz", label: "Quiz me", icon: ListChecks },
  { id: "visualize", label: "Visualize", icon: Boxes },
  { id: "card", label: "Flashcard", icon: Layers },
];

type Inline =
  | { id: string; after: string; kind: "answer"; selection: string }
  | { id: string; after: string; kind: "aid"; selection: string }
  | { id: string; after: string; kind: "quiz"; selection: string };

export function TutorReadSpike() {
  const [inlines, setInlines] = useState<Inline[]>([]);
  const [sel, setSel] = useState<{ text: string; blockId: string; x: number; y: number } | null>(null);
  const [planOpen, setPlanOpen] = useState(false);
  const scroller = useRef<HTMLDivElement>(null);

  // Real text selection, because the whole design rests on it: if selecting a passage is
  // awkward, "summon from the text" collapses and you need a pane after all.
  const onMouseUp = () => {
    const s = window.getSelection();
    const text = s?.toString().trim() ?? "";
    if (!s || text.length < 3) {
      setSel(null);
      return;
    }
    const node = s.anchorNode?.parentElement?.closest("[data-block]") as HTMLElement | null;
    const rect = s.getRangeAt(0).getBoundingClientRect();
    const host = scroller.current?.getBoundingClientRect();
    if (!node || !host) return;
    setSel({
      text,
      blockId: node.dataset.block ?? "",
      x: rect.left - host.left + rect.width / 2,
      y: rect.top - host.top,
    });
  };

  const act = (action: string) => {
    if (!sel) return;
    const kind = action === "visualize" ? "aid" : action === "quiz" ? "quiz" : "answer";
    setInlines((prev) => [
      ...prev.filter((i) => !(i.after === sel.blockId && i.kind === kind)),
      { id: `${sel.blockId}-${kind}`, after: sel.blockId, kind, selection: sel.text },
    ]);
    setSel(null);
    window.getSelection()?.removeAllRanges();
  };

  const done = PLAN.filter((p) => p.done).length;

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-bg text-ink">
      {/* the only permanent chrome: where you are, and how far in */}
      <header className="flex shrink-0 items-center gap-4 px-6 py-3">
        <span className="truncate font-display text-body text-ink">{SOURCE.title}</span>
        <button
          type="button"
          onClick={() => setPlanOpen((v) => !v)}
          className="group flex items-center gap-2"
          title="your plan"
        >
          <span className="flex items-center gap-1">
            {PLAN.map((p) => (
              <span
                key={p.n}
                className={cn(
                  "h-1 w-8 rounded-chip transition-colors",
                  p.done ? "bg-amber" : p.current ? "bg-amber-soft" : "bg-line-2",
                )}
              />
            ))}
          </span>
          <span className="font-mono text-meta text-ink-4 group-hover:text-ink-3">
            {done}/{PLAN.length} · Collars and spreads
          </span>
        </button>
        <span className="ml-auto shrink-0 font-mono text-meta text-ink-4">p. 94 / {SOURCE.pages}</span>
      </header>

      {planOpen ? <PlanSheet onClose={() => setPlanOpen(false)} /> : null}

      {/* the document — full measure, nothing beside it */}
      <div ref={scroller} className="relative min-h-0 flex-1 overflow-auto" onMouseUp={onMouseUp}>
        <article className="mx-auto max-w-[64ch] px-6 pb-32 pt-8">
          <Eyebrow as="p" className="mb-8 text-ink-4">§4.2 Collars</Eyebrow>

          {BLOCKS.map((b) => (
            <div key={b.id}>
              {b.kind === "prose" ? (
                <p data-block={b.id} className="mb-6 text-lead leading-[1.75] text-ink-2 selection:bg-amber-soft">
                  {b.text}
                </p>
              ) : (
                <div data-block={b.id} className="mb-6 flex items-baseline justify-between gap-4">
                  <code className="font-mono text-lead text-ink selection:bg-amber-soft">{b.text}</code>
                  <span className="shrink-0 font-mono text-meta text-ink-4">{b.cite}</span>
                </div>
              )}

              {inlines
                .filter((i) => i.after === b.id)
                .map((i) => (
                  <InlineBlock
                    key={i.id}
                    inline={i}
                    onClose={() => setInlines((prev) => prev.filter((x) => x.id !== i.id))}
                  />
                ))}
            </div>
          ))}

          <p className="mt-16 text-center font-mono text-meta text-ink-4">
            select any passage to ask about it
          </p>
        </article>

        {/* summoned at the selection, gone the moment you act */}
        {sel ? (
          <div
            className="absolute z-10 -translate-x-1/2 -translate-y-full pb-2"
            style={{ left: sel.x, top: sel.y }}
          >
            <div className="flex items-center gap-0.5 rounded-chip bg-raise p-0.5 shadow-modal">
              {ACTIONS.map((a) => (
                <button
                  key={a.id}
                  type="button"
                  onClick={() => act(a.id)}
                  className="flex items-center gap-1 rounded-chip px-2 py-1 text-meta text-ink-2 transition-colors hover:bg-card hover:text-ink"
                >
                  <a.icon className="size-3" />
                  {a.label}
                </button>
              ))}
            </div>
          </div>
        ) : null}
      </div>
    </div>
  );
}

// ── inline responses: they land where you asked, and can be dismissed ────────────────
function InlineBlock({ inline, onClose }: { inline: Inline; onClose: () => void }) {
  return (
    <div className="mb-6 border-l-2 border-amber-line pl-4">
      <div className="mb-1.5 flex items-start gap-2">
        <p className="min-w-0 flex-1 truncate font-mono text-meta text-ink-4">
          “{inline.selection}”
        </p>
        <button type="button" onClick={onClose} className="shrink-0 text-ink-4 hover:text-ink">
          <Icon as={X} size="inline" mark />
        </button>
      </div>
      {inline.kind === "answer" ? <AnswerBody /> : inline.kind === "quiz" ? <QuizBody /> : <PayoffBody />}
    </div>
  );
}

function AnswerBody() {
  // §7 — the default is a pointer. Inline, this reads as a footnote rather than a lecture,
  // which is the right weight for "the source already says this."
  return (
    <div>
      <p className="text-body leading-relaxed text-ink-2">
        Bounded by both strikes, so the worst case is the width minus what you paid.
      </p>
      <button
        type="button"
        className="mt-1.5 flex items-baseline gap-2 text-left transition-colors hover:text-ink"
      >
        <code className="font-mono text-meta text-ink-3">Lₘₐₓ = K₂ − K₁ − C</code>
        <span className="font-mono text-meta text-ink-4">eq. 4.8 · p. 94 →</span>
      </button>
    </div>
  );
}

function QuizBody() {
  const [picked, setPicked] = useState<number | null>(null);
  const options = [
    { t: "K₂ − S₀ − C", ok: false },
    { t: "K₁ − S₀ − C", ok: true },
    { t: "K₂ − K₁", ok: false },
  ];
  return (
    <div>
      <p className="text-body leading-relaxed text-ink-2">
        A collar on stock bought at S₀. What is the <span className="text-ink">maximum loss</span>?
      </p>
      <div className="mt-2 space-y-1">
        {options.map((o, idx) => {
          const state = picked === null ? "idle" : o.ok ? "ok" : picked === idx ? "bad" : "idle";
          return (
            <button
              key={o.t}
              type="button"
              onClick={() => setPicked(idx)}
              className={cn(
                "block w-full rounded-chip px-2 py-1 text-left font-mono text-small transition-colors",
                state === "idle" && "text-ink-3 hover:bg-raise",
                state === "ok" && "bg-amber-soft text-ink",
                state === "bad" && "text-against line-through",
              )}
            >
              {o.t}
            </button>
          );
        })}
      </div>
      {picked !== null ? (
        <p className="mt-1.5 font-mono text-meta text-ink-4">
          {options[picked].ok ? "right" : "not quite"} — the put sets the floor · eq. 4.8, p. 94
        </p>
      ) : null}
    </div>
  );
}

// The one thing prose cannot be. Embedded at the formula it comes from.
function PayoffBody() {
  const [k1, setK1] = useState(90);
  const [k2, setK2] = useState(115);
  const s0 = 100;
  const c = 2;

  const { path, zeroY } = useMemo(() => {
    const f = (s: number) => Math.min(Math.max(s, k1), k2) - s0 - c;
    const xs = Array.from({ length: 81 }, (_, i) => 60 + i);
    const ys = xs.map(f);
    const lo = Math.min(...ys, 0);
    const hi = Math.max(...ys, 0);
    const pad = (hi - lo) * 0.2 || 1;
    const toY = (v: number) => 110 - ((v - (lo - pad)) / (hi - lo + pad * 2)) * 100;
    return {
      path: xs.map((s, i) => `${(((s - 60) / 80) * 420 + 10).toFixed(1)},${toY(ys[i]).toFixed(1)}`).join(" "),
      zeroY: toY(0),
    };
  }, [k1, k2]);

  return (
    <div>
      <svg viewBox="0 0 440 120" className="w-full" role="img" aria-label="Collar payoff">
        <line x1="10" y1={zeroY} x2="430" y2={zeroY} className="stroke-line-2" strokeWidth="1" />
        {[k1, k2].map((k) => (
          <line
            key={k}
            x1={((k - 60) / 80) * 420 + 10}
            y1="6"
            x2={((k - 60) / 80) * 420 + 10}
            y2="114"
            className="stroke-line"
            strokeWidth="1"
            strokeDasharray="2 3"
          />
        ))}
        <polyline points={path} fill="none" className="stroke-amber" strokeWidth="2" />
      </svg>
      <div className="mt-1 flex items-center gap-4">
        <Mini label="K₁" value={k1} min={70} max={99} onChange={setK1} />
        <Mini label="K₂" value={k2} min={101} max={135} onChange={setK2} />
        <span className="ml-auto shrink-0 font-mono text-meta text-ink-4">
          <span className="text-for">+{(k2 - s0 - c).toFixed(0)}</span> /{" "}
          <span className="text-against">{(k1 - s0 - c).toFixed(0)}</span>
        </span>
      </div>
      <p className="mt-1.5 font-mono text-meta text-ink-4">from eq. 4.7 · pp. 88–104</p>
    </div>
  );
}

function Mini({
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
    <label className="flex items-center gap-1.5">
      <span className="font-mono text-meta text-ink-4">{label}</span>
      <input
        type="range"
        min={min}
        max={max}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="h-1 w-20 accent-amber"
      />
      <span className="w-6 font-mono text-meta text-ink-3">{value}</span>
    </label>
  );
}

// ── the plan: summoned, not resident ─────────────────────────────────────────────────
function PlanSheet({ onClose }: { onClose: () => void }) {
  return (
    <div className="absolute inset-x-0 top-12 z-20 mx-auto max-w-[64ch] px-6">
      <div className="rounded-card bg-raise p-4 shadow-modal">
        <div className="mb-3 flex items-baseline gap-2">
          <span className="font-display text-small uppercase tracking-wider text-ink-3">Your plan</span>
          <span className="text-meta text-ink-4">edit freely — this outlives every aid</span>
          <button type="button" onClick={onClose} className="ml-auto text-ink-4 hover:text-ink">
            <Icon as={X} size="inline" mark />
          </button>
        </div>
        {PLAN.map((p) => (
          <div
            key={p.n}
            className={cn(
              "group flex items-baseline gap-3 border-l-2 py-1.5 pl-3",
              p.current ? "border-amber" : "border-transparent",
            )}
          >
            <span className={cn("text-body", p.done ? "text-ink-4 line-through" : "text-ink")}>{p.title}</span>
            <span className="font-mono text-meta text-ink-4">{p.span}</span>
            <span className="ml-auto flex shrink-0 gap-2 opacity-0 transition-opacity group-hover:opacity-100">
              <button type="button" className="font-mono text-meta text-ink-4 hover:text-ink">
                split
              </button>
              <button type="button" className="font-mono text-meta text-ink-4 hover:text-against">
                drop
              </button>
            </span>
          </div>
        ))}
        <button type="button" className="mt-2 pl-3 font-mono text-meta text-ink-4 hover:text-ink-2">
          + add an item
        </button>
      </div>
    </div>
  );
}
