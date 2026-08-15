import { AlertTriangle, CalendarClock, Globe, Sparkles, Zap } from "lucide-react";
import { Eyebrow } from "@/components/ui/eyebrow";
import { Icon } from "@/components/ui/icon";
import { useState } from "react";

import { cn } from "@/lib/utils";
import { useAppStore } from "@/app/state/store";

// Dev-only spike (/spike/entities-home) — the Entities home reimagined as a trader's
// RESEARCH MAP. Aladin is a research/engine platform for retail traders, so the entity
// graph is: tickers, theses, sectors, catalysts, people — and the connections are the
// research substance (supply chains, competition, shared theses, shared catalysts).
// The map shows what a watchlist hides: hidden concentration, catalyst blast radius,
// orphan names, and where your bull/bear conviction collides. Pure mock — the vision.

type Kind = "ticker" | "thesis" | "person" | "catalyst";
type Node = { id: string; name: string; kind: Kind; x: number; y: number; r: number; hot?: boolean; };
type EdgeKind = "supply" | "thesis" | "tension" | "catalyst";
type Edge = { a: string; b: string; kind?: EdgeKind };

const toneClass: Record<Kind, string> = {
  ticker: "text-amber",
  thesis: "text-echo",
  person: "text-for",
  catalyst: "text-catalyst",
};

const NODES: Node[] = [
  // ── AI infrastructure (your core long)
  { id: "nvda", name: "NVDA", kind: "ticker", x: 300, y: 230, r: 18 },
  { id: "tsmc", name: "TSMC", kind: "ticker", x: 235, y: 340, r: 15 },
  { id: "asml", name: "ASML", kind: "ticker", x: 150, y: 430, r: 9 },
  { id: "avgo", name: "AVGO", kind: "ticker", x: 400, y: 300, r: 11 },
  { id: "smci", name: "SMCI", kind: "ticker", x: 385, y: 175, r: 7 },
  { id: "capex", name: "AI capex supercycle", kind: "thesis", x: 300, y: 120, r: 12 },
  { id: "jensen", name: "Jensen Huang", kind: "person", x: 205, y: 190, r: 7 },
  // ── Hyperscaler demand
  { id: "msft", name: "MSFT", kind: "ticker", x: 720, y: 200, r: 13 },
  { id: "amzn", name: "AMZN", kind: "ticker", x: 830, y: 265, r: 11 },
  { id: "googl", name: "GOOGL", kind: "ticker", x: 640, y: 285, r: 10 },
  { id: "sovereign", name: "sovereign AI demand", kind: "thesis", x: 780, y: 130, r: 10 },
  // ── The bear case / macro risk
  { id: "digestion", name: "capex digestion", kind: "thesis", x: 560, y: 460, r: 11 },
  { id: "rates", name: "Fed rate path", kind: "thesis", x: 700, y: 500, r: 9 },
  { id: "china", name: "China export controls", kind: "catalyst", x: 820, y: 420, r: 8, hot: true },
  // ── Catalysts landing this week (hot = imminent)
  { id: "nvdaer", name: "NVDA earnings · Wed", kind: "catalyst", x: 175, y: 130, r: 7, hot: true },
  { id: "cpi", name: "CPI · Thu", kind: "catalyst", x: 620, y: 590, r: 7, hot: true },
  // ── Watching, no thesis yet (orphans — why do you own these?)
  { id: "arm", name: "ARM", kind: "ticker", x: 480, y: 90, r: 6 },
  { id: "pltr", name: "PLTR", kind: "ticker", x: 500, y: 620, r: 6 },
  { id: "mu", name: "MU", kind: "ticker", x: 940, y: 340, r: 6 },
];

const EDGES: Edge[] = [
  // supply chain
  { a: "nvda", b: "tsmc", kind: "supply" }, { a: "tsmc", b: "asml", kind: "supply" },
  { a: "nvda", b: "avgo", kind: "supply" }, { a: "smci", b: "nvda", kind: "supply" },
  // demand (hyperscalers buy NVDA)
  { a: "nvda", b: "msft", kind: "supply" }, { a: "nvda", b: "amzn", kind: "supply" },
  { a: "nvda", b: "googl", kind: "supply" }, { a: "tsmc", b: "avgo", kind: "supply" },
  { a: "jensen", b: "nvda", kind: "supply" },
  // theses touch names (dashed violet)
  { a: "capex", b: "nvda", kind: "thesis" }, { a: "capex", b: "tsmc", kind: "thesis" },
  { a: "capex", b: "avgo", kind: "thesis" }, { a: "capex", b: "smci", kind: "thesis" },
  { a: "sovereign", b: "msft", kind: "thesis" }, { a: "sovereign", b: "nvda", kind: "thesis" },
  { a: "sovereign", b: "amzn", kind: "thesis" }, { a: "digestion", b: "avgo", kind: "thesis" },
  { a: "digestion", b: "smci", kind: "thesis" }, { a: "rates", b: "pltr", kind: "thesis" },
  // catalysts hit names (dashed amber)
  { a: "nvdaer", b: "nvda", kind: "catalyst" }, { a: "china", b: "nvda", kind: "catalyst" },
  { a: "china", b: "tsmc", kind: "catalyst" }, { a: "cpi", b: "rates", kind: "catalyst" },
  // the contested conviction (bull vs bear — red)
  { a: "capex", b: "digestion", kind: "tension" },
];

const byId = Object.fromEntries(NODES.map((n) => [n.id, n]));

const CLUSTER_LABELS = [
  { label: "AI infrastructure", x: 275, y: 300 },
  { label: "Hyperscaler demand", x: 745, y: 235 },
  { label: "Bear case · macro", x: 660, y: 545 },
];

function neighborsOf(id: string): Set<string> {
  const s = new Set<string>([id]);
  for (const e of EDGES) {
    if (e.a === id) s.add(e.b);
    if (e.b === id) s.add(e.a);
  }
  return s;
}

const edgeStyle: Record<EdgeKind, { stroke: string; width: number; dash?: string }> = {
  supply: { stroke: "var(--line-2)", width: 1 },
  thesis: { stroke: "var(--echo)", width: 1, dash: "1 4" },
  catalyst: { stroke: "var(--amber)", width: 1, dash: "3 3" },
  tension: { stroke: "var(--against)", width: 1.6, dash: "4 3" },
};

function Constellation() {
  const [hover, setHover] = useState<string | null>(null);
  const lit = hover ? neighborsOf(hover) : null;
  const on = (id: string) => !lit || lit.has(id);

  return (
    <svg viewBox="0 0 1000 660" className="h-full w-full" preserveAspectRatio="xMidYMid meet">
      <defs>
        <filter id="soft" x="-60%" y="-60%" width="220%" height="220%">
          <feGaussianBlur stdDeviation="7" />
        </filter>
      </defs>

      {CLUSTER_LABELS.map((c) => (
        <circle key={c.label} cx={c.x} cy={c.y - 40} r="150" className="fill-raise" opacity={0.32} filter="url(#soft)" />
      ))}
      {CLUSTER_LABELS.map((c) => (
        <text key={c.label} x={c.x} y={c.y} textAnchor="middle" className="fill-ink-4 font-display" fontSize="15" opacity={hover ? 0.22 : 0.55}>
          {c.label}
        </text>
      ))}

      {EDGES.map((e, i) => {
        const a = byId[e.a], b = byId[e.b];
        const st = edgeStyle[e.kind ?? "supply"];
        const active = on(e.a) && on(e.b);
        return (
          <line
            key={i}
            x1={a.x} y1={a.y} x2={b.x} y2={b.y}
            style={{ stroke: st.stroke, strokeWidth: st.width, strokeDasharray: st.dash }}
            opacity={active ? (e.kind === "tension" ? 0.9 : e.kind === "supply" ? 1 : 0.6) : 0.07}
          />
        );
      })}

      {NODES.map((n) => {
        const active = on(n.id);
        return (
          <g
            key={n.id}
            className={cn(toneClass[n.kind], "cursor-pointer")}
            opacity={active ? 1 : 0.16}
            onMouseEnter={() => setHover(n.id)}
            onMouseLeave={() => setHover(null)}
          >
            <circle cx={n.x} cy={n.y} r={n.r + 8} fill="currentColor" opacity={n.hot ? 0.3 : 0.12} filter="url(#soft)" className={n.hot ? "animate-pulse" : undefined} />
            {n.hot && <circle cx={n.x} cy={n.y} r={n.r + 4} className="fill-none" style={{ stroke: "var(--amber)" }} strokeWidth={1} opacity={0.7} />}
            {n.kind === "thesis" ? (
              // theses render as diamonds — a different shape for a different kind of thing
              <rect x={n.x - n.r} y={n.y - n.r} width={n.r * 2} height={n.r * 2} transform={`rotate(45 ${n.x} ${n.y})`} fill="currentColor" />
            ) : (
              <circle cx={n.x} cy={n.y} r={n.r} fill="currentColor" />
            )}
            {(n.r >= 9 || hover === n.id) && (
              <text
                x={n.x} y={n.y + n.r + 13} textAnchor="middle"
                className="fill-ink-2 font-mono" fontSize="10.5"
                style={{ paintOrder: "stroke", stroke: "var(--bg)", strokeWidth: 3 }}
              >
                {n.name}
              </text>
            )}
          </g>
        );
      })}
    </svg>
  );
}

function EntitiesResearchHome() {
  return (
    <div className="flex h-full w-full overflow-hidden">
      <aside className="flex w-[300px] shrink-0 flex-col gap-5 overflow-y-auto border-r border-line px-5 py-6">
        <div>
          <Eyebrow className="mb-2">This week, your research</Eyebrow>
          <p className="text-body leading-[1.6] text-ink-2">
            concentrated on <span className="font-medium text-ink">AI infrastructure</span>.{" "}
            <span className="font-medium text-ink">NVDA</span> is your most-connected name —
            under 3 theses and your whole semis chain.
          </p>
        </div>

        <div className="rounded-card border border-against/40 bg-card p-3.5">
          <div className="flex items-center gap-1.5 font-mono text-meta text-against">
            <Icon as={AlertTriangle} size="inline" mark /> concentration risk
          </div>
          <p className="mt-1.5 text-small leading-[1.5] text-ink-2">
            <span className="text-ink">TSMC</span> is a single point of failure across{" "}
            <span className="text-ink">5 of your longs</span> — NVDA, AVGO, MU, and 2 more all
            depend on it.
          </p>
        </div>

        <div className="rounded-card border border-amber-line bg-amber-soft p-3.5">
          <div className="flex items-center gap-1.5 font-mono text-meta text-amber">
            <Icon as={CalendarClock} size="inline" mark /> catalysts this week
          </div>
          <p className="mt-1.5 text-small leading-[1.5] text-ink-2">
            <span className="text-ink">NVDA earnings Wed</span> and{" "}
            <span className="text-ink">CPI Thu</span> — both hit your AI-infra cluster. China
            export controls loom over NVDA + TSMC.
          </p>
        </div>

        <div className="rounded-card border border-against/30 bg-card p-3.5">
          <div className="flex items-center gap-1.5 font-mono text-meta text-against">
            <Icon as={Zap} size="inline" mark /> conviction contested
          </div>
          <p className="mt-1.5 text-small leading-[1.5] text-ink-2">
            Your <span className="text-ink">‘AI capex supercycle’</span> thesis collides with
            the <span className="text-ink">‘capex digestion’</span> note you added Monday.
          </p>
        </div>

        <div>
          <Eyebrow className="mb-2 flex items-center gap-1.5">
            <Icon as={Sparkles} size="inline" /> Orphans · no thesis
          </Eyebrow>
          <p className="mb-2 text-meta leading-[1.5] text-ink-4">
            You're watching these but haven't threaded them into a thesis.
          </p>
          <div className="flex flex-wrap gap-1.5">
            {["ARM", "PLTR", "MU"].map((n) => (
              <span key={n} className="rounded-chip border border-line-2 bg-bg px-2 py-0.5 font-mono text-meta text-ink-3">
                {n}
              </span>
            ))}
          </div>
        </div>
      </aside>

      <div className="relative min-w-0 flex-1">
        <Constellation />
        <div className="pointer-events-none absolute bottom-4 left-5 flex flex-wrap items-center gap-x-4 gap-y-1 font-mono text-meta text-ink-4">
          <span className="flex items-center gap-1.5"><span className="size-2 rounded-full bg-amber" /> ticker</span>
          <span className="flex items-center gap-1.5"><span className="size-2 rotate-45 bg-echo" /> thesis</span>
          <span className="flex items-center gap-1.5"><span className="size-2 rounded-full bg-catalyst" /> catalyst</span>
          <span className="flex items-center gap-1.5"><span className="inline-block h-[2px] w-4" style={{ background: "var(--echo)" }} /> supports thesis</span>
          <span className="flex items-center gap-1.5"><span className="inline-block h-[2px] w-4 bg-against" /> bull vs bear</span>
        </div>
      </div>
    </div>
  );
}

export function EntitiesHomeSpike() {
  const theme = useAppStore((s) => s.theme);
  return (
    <div className="flex h-screen flex-col bg-bg text-ink">
      <div className="flex items-center gap-3 border-b border-line bg-chrome px-4 py-2 text-small text-ink-3">
        <span className="font-mono">/spike/entities-home</span>
        <span className="text-ink-4">·</span>
        <span>theme: {theme}</span>
        <button
          className="rounded-chip border border-line px-2 py-0.5 hover:bg-raise hover:text-ink"
          onClick={() => useAppStore.getState().setTheme(theme === "dark" ? "soft" : "dark")}
        >
          toggle theme
        </button>
      </div>
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <div className="flex items-center gap-3 border-b border-line px-5 py-3">
          <Icon as={Globe} className="text-ink-3" />
          <Eyebrow as="span">Research map</Eyebrow>
          <span className="rounded-chip bg-sel px-2.5 py-1 text-small text-ink">Map</span>
          <span className="rounded-chip px-2.5 py-1 text-small text-ink-3">Inbox</span>
          <span className="rounded-chip px-2.5 py-1 text-small text-ink-3">Browse</span>
          <span className="ml-auto font-mono text-meta text-ink-4">
            <span className="text-ink-3">42</span> names · <span className="text-ink-3">11</span> theses · <span className="text-amber">3</span> catalysts this week
          </span>
        </div>
        <div className="min-h-0 flex-1">
          <EntitiesResearchHome />
        </div>
      </div>
    </div>
  );
}
