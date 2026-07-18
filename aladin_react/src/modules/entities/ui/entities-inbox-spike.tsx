import {
  ArrowLeftRight,
  Check,
  GitBranch,
  Globe,
  Home,
  LayoutGrid,
  Link2,
  Plus,
  Quote,
  Search,
  Sparkles,
  TrendingUp,
  X,
} from "lucide-react";
import { useMemo, useState } from "react";

import { cn } from "@/lib/utils";
import type { EntityListItem } from "@/modules/entities/entity-list-types";
import { EntityCard, EntityGlyph } from "@/modules/entities/ui/entity-card";
import { useAppStore } from "@/app/state/store";

// Dev-only spike (/spike/entities-inbox) — the Entities home on mock data. Mirrors the real
// /entities surface (entities-index-ui.tsx) so it can be driven without auth.
//
// Model (locked 2026-07-17): the DASHBOARD is the home — what needs a decision, what moved,
// what just arrived. A flat LIST only appears as the answer to a SEARCH. The full card GRID
// is still reachable via the Overview/Grid toggle — an opt-in browse view, not the default.
//
// Recast into the trading domain per the north star: entities are tickers · companies ·
// theses · catalysts · people, not generic knowledge nodes.

type EntityView = "overview" | "grid";

// ── mock data (trading) ──────────────────────────────────────────────────────────────
const DECISIONS = [
  { id: "d1", from: "NVDA", fromKind: "other", to: "Nvidia", toKind: "org", pct: 92, verdict: "likely the same", tone: "text-for" },
  { id: "d2", from: "TSM", fromKind: "other", to: "Taiwan Semi", toKind: "org", pct: 88, verdict: "likely the same", tone: "text-for" },
  { id: "d3", from: "AI capex", fromKind: "concept", to: "AI infra spend", toKind: "concept", pct: 61, verdict: "your call", tone: "text-ink-4" },
];

const ACTIVITY = [
  { id: "a1", name: "NVIDIA", kind: "org", change: "+3 sources", detail: "earnings call, Stratechery", time: "2h", icon: Quote },
  { id: "a2", name: "CoWoS", kind: "concept", change: "new link", detail: "→ TSMC (supply chokepoint)", time: "5h", icon: GitBranch },
  { id: "a3", name: "AI data-center buildout", kind: "concept", change: "thesis firmed", detail: "+2 supporting sources", time: "8h", icon: TrendingUp },
  { id: "a4", name: "Taiwan Semi", kind: "org", change: "merged in", detail: "‘TSM’ folded here", time: "1d", icon: ArrowLeftRight },
  { id: "a5", name: "Micron", kind: "org", change: "gist updated", detail: "you edited the definition", time: "2d", icon: Sparkles },
];

const NEW_ENTITIES = [
  { id: "n1", name: "Blackwell", kind: "concept", gist: "NVIDIA's current GPU architecture.", time: "1d" },
  { id: "n2", name: "CoWoS", kind: "concept", gist: "Advanced packaging — the supply chokepoint.", time: "2d" },
  { id: "n3", name: "Vera Rubin", kind: "concept", gist: "NVIDIA's next-gen platform (2026).", time: "3d" },
  { id: "n4", name: "CoreWeave", kind: "org", gist: "Neocloud GPU renter; NVDA-backed.", time: "4d" },
];

const LOOSE = [
  { name: "ARM", kind: "org" },
  { name: "SMCI", kind: "org" },
  { name: "rate-cut odds", kind: "concept" },
  { name: "ASML", kind: "org" },
  { name: "power constraints", kind: "concept" },
  { name: "coreweave-s1.pdf", kind: "other" },
  { name: "Broadcom", kind: "org" },
  { name: "MU", kind: "other" },
];

// The searchable pool — the whole layer, seen as search results or the grid.
type PoolEntity = {
  name: string;
  kind: string;
  gist: string;
  aliases: string[];
  links: number;
  sources: number;
  time: string;
};
const ENTITIES: PoolEntity[] = [
  { name: "NVIDIA", kind: "org", gist: "GPU maker; the center of the AI-infra trade.", aliases: ["NVDA", "Nvidia"], links: 12, sources: 8, time: "2h" },
  { name: "TSMC", kind: "org", gist: "Sole leading-edge foundry; makes NVIDIA's chips.", aliases: ["TSM", "Taiwan Semi"], links: 9, sources: 6, time: "1d" },
  { name: "CoWoS", kind: "concept", gist: "Advanced packaging — the supply chokepoint.", aliases: [], links: 5, sources: 3, time: "2d" },
  { name: "Blackwell", kind: "concept", gist: "NVIDIA's current GPU architecture.", aliases: ["GB200"], links: 4, sources: 2, time: "1d" },
  { name: "AI data-center buildout", kind: "concept", gist: "Thesis: hyperscaler capex keeps compounding.", aliases: ["AI capex", "AI infra spend"], links: 7, sources: 5, time: "8h" },
  { name: "Micron", kind: "org", gist: "HBM memory; leveraged to the AI-DRAM cycle.", aliases: ["MU"], links: 3, sources: 2, time: "2d" },
  { name: "ASML", kind: "org", gist: "EUV monopoly; upstream of every advanced node.", aliases: [], links: 2, sources: 1, time: "5d" },
  { name: "ARM", kind: "org", gist: "IP licensing; no thesis attached yet.", aliases: ["ARM Holdings"], links: 0, sources: 1, time: "1w" },
  { name: "Super Micro", kind: "org", gist: "Server assembler; volatile AI beta.", aliases: ["SMCI"], links: 0, sources: 2, time: "3d" },
  { name: "CoreWeave", kind: "org", gist: "Neocloud GPU renter; NVDA-backed.", aliases: ["CRWV"], links: 1, sources: 2, time: "4d" },
  { name: "Jensen Huang", kind: "person", gist: "NVIDIA CEO.", aliases: [], links: 3, sources: 4, time: "3d" },
  { name: "Broadcom", kind: "org", gist: "Custom AI silicon (ASICs); the anti-NVDA angle.", aliases: ["AVGO"], links: 1, sources: 1, time: "6d" },
  { name: "Fed rate path", kind: "concept", gist: "Macro overlay on high-duration growth.", aliases: ["rate-cut odds"], links: 2, sources: 3, time: "1d" },
  { name: "Power constraints", kind: "concept", gist: "Grid + datacenter power as the next bottleneck.", aliases: [], links: 1, sources: 1, time: "4d" },
  { name: "Vera Rubin", kind: "concept", gist: "NVIDIA's next-gen platform (2026).", aliases: [], links: 1, sources: 1, time: "3d" },
  { name: "Hyperscaler capex", kind: "concept", gist: "MSFT/GOOG/AMZN/META spend — the demand signal.", aliases: [], links: 4, sources: 3, time: "1d" },
];

// Map a mock pool entity onto the real card shape so the grid reuses EntityCard verbatim.
function toCard(e: PoolEntity): EntityListItem {
  return {
    id: e.name,
    name: e.name,
    kind: e.kind,
    gist: e.gist,
    trustTier: e.links === 0 ? "placeholder" : "believed",
    updatedAt: "",
    links: e.links,
    sources: e.sources,
    attention: 0,
    aliases: e.aliases,
  };
}

// ── section atoms ──────────────────────────────────────────────────────────────────
function SectionHead({ title, hint, count }: { title: string; hint?: string; count?: string }) {
  return (
    <div className="mb-2.5 flex items-center gap-2">
      <span className="font-display text-[13px] font-semibold text-ink">{title}</span>
      {hint && <span className="font-mono text-[10px] text-ink-4">{hint}</span>}
      <span className="h-px flex-1 bg-line-2" />
      {count && <span className="font-mono text-[10px] text-ink-4">{count}</span>}
    </div>
  );
}

function ViewToggle({ view, onView }: { view: EntityView; onView: (v: EntityView) => void }) {
  const btn = (v: EntityView, Icon: typeof LayoutGrid, label: string) => (
    <button
      type="button"
      title={label}
      aria-label={label}
      aria-pressed={view === v}
      onClick={() => onView(v)}
      className={cn(
        "flex items-center justify-center rounded-chip px-2 py-1 transition-colors",
        view === v ? "bg-[rgb(var(--sel))] text-ink" : "text-ink-3 hover:bg-[rgb(var(--hover))] hover:text-ink",
      )}
    >
      <Icon size={14} strokeWidth={1.9} />
    </button>
  );
  return (
    <div className="flex items-center gap-0.5">
      {btn("overview", Home, "Overview")}
      {btn("grid", LayoutGrid, "Grid")}
    </div>
  );
}

function DecisionCard({ d, onDecide }: { d: (typeof DECISIONS)[number]; onDecide: () => void }) {
  return (
    <div className="rounded-[12px] border border-line-2 bg-card p-3">
      <div className="flex items-center gap-2">
        <span className="flex min-w-0 flex-1 items-center gap-2">
          <span className="truncate font-display text-[13.5px] font-semibold text-ink">{d.from}</span>
          <span className="shrink-0 font-mono text-[9px] text-ink-4">{d.fromKind}</span>
          <ArrowLeftRight size={12} strokeWidth={1.9} className="shrink-0 text-ink-4" />
          <span className="truncate font-display text-[13.5px] font-semibold text-ink">{d.to}</span>
          <span className="shrink-0 font-mono text-[9px] text-ink-4">{d.toKind}</span>
        </span>
        <span className="shrink-0 font-mono text-[10px] text-ink-3">{d.pct}%</span>
      </div>
      <div className={cn("mt-1 font-mono text-[9.5px]", d.tone)}>{d.verdict}</div>
      <div className="mt-2.5 flex items-center gap-2">
        <button
          onClick={onDecide}
          className="flex items-center gap-1 rounded-chip bg-amber px-2.5 py-1 text-[11px] font-semibold text-primary-foreground transition hover:brightness-[1.08]"
        >
          <Check size={12} strokeWidth={2.4} /> Merge
        </button>
        <button
          onClick={onDecide}
          className="flex items-center gap-1 rounded-chip border border-line px-2.5 py-1 text-[11px] font-semibold text-ink-2 transition hover:brightness-[1.08]"
        >
          <X size={12} strokeWidth={2.4} /> Keep separate
        </button>
      </div>
    </div>
  );
}

function ActivityRow({ a }: { a: (typeof ACTIVITY)[number] }) {
  const Icon = a.icon;
  return (
    <button className="group flex w-full items-center gap-3 rounded-[10px] px-2 py-2 text-left transition-colors hover:bg-raise">
      <EntityGlyph kind={a.kind} name={a.name} size={26} />
      <span className="min-w-0 flex-1">
        <span className="flex items-baseline gap-2">
          <span className="truncate font-display text-[13px] font-semibold text-ink">{a.name}</span>
          <span className="flex shrink-0 items-center gap-1 font-mono text-[9.5px] text-ink-3">
            <Icon size={10} strokeWidth={1.8} /> {a.change}
          </span>
        </span>
        <span className="mt-0.5 block truncate text-[11.5px] text-ink-4">{a.detail}</span>
      </span>
      <span className="shrink-0 font-mono text-[9.5px] text-ink-4">{a.time}</span>
    </button>
  );
}

function NewEntityRow({ n }: { n: (typeof NEW_ENTITIES)[number] }) {
  return (
    <button className="flex w-full items-start gap-2.5 rounded-[10px] px-2 py-2 text-left transition-colors hover:bg-raise">
      <EntityGlyph kind={n.kind} name={n.name} size={24} />
      <span className="min-w-0 flex-1">
        <span className="flex items-baseline gap-1.5">
          <span className="truncate font-display text-[12.5px] font-semibold text-ink">{n.name}</span>
          <span className="shrink-0 font-mono text-[8.5px] text-ink-4">{n.time}</span>
        </span>
        <span className="mt-0.5 line-clamp-1 text-[11px] text-ink-4">{n.gist}</span>
      </span>
    </button>
  );
}

// ── the home ─────────────────────────────────────────────────────────────────────────
function EntitiesHome() {
  const [decisions, setDecisions] = useState(DECISIONS);
  const [query, setQuery] = useState("");
  const [view, setView] = useState<EntityView>("overview");
  const drop = (id: string) => setDecisions((p) => p.filter((d) => d.id !== id));

  const q = query.trim().toLowerCase();
  const searching = q.length > 0;
  const pool = useMemo(() => {
    if (!q) return ENTITIES;
    return ENTITIES.filter(
      (e) => e.name.toLowerCase().includes(q) || e.aliases.some((a) => a.toLowerCase().includes(q)),
    );
  }, [q]);

  // Typing opens the grid: search always resolves to the (filtered) card grid.
  const showGrid = view === "grid" || searching;
  const selectView = (v: EntityView) => {
    setView(v);
    if (v === "overview") setQuery("");
  };

  return (
    <div className="flex h-full w-full flex-col overflow-hidden">
      {/* header bar — search + the Overview/Grid toggle */}
      <div className="flex items-center gap-3 border-b border-line px-5 pt-4 pb-3">
        <Globe size={15} strokeWidth={1.8} className="text-ink-3" />
        <span className="eyebrow">Entities</span>
        <div className="relative ml-1 w-72">
          <Search size={13} strokeWidth={1.9} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-ink-4" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search a ticker, company, thesis…"
            className="w-full rounded-chip border border-line bg-field py-1.5 pl-8 pr-7 text-[12.5px] text-ink outline-none placeholder:text-ink-4 focus:border-amber-line"
          />
          {searching && (
            <button
              onClick={() => setQuery("")}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-ink-4 hover:text-ink-2"
            >
              <X size={13} strokeWidth={2.2} />
            </button>
          )}
        </div>
        <ViewToggle view={showGrid ? "grid" : "overview"} onView={selectView} />
        <span className="ml-auto font-mono text-[10px] text-ink-4">
          {showGrid ? (
            <>
              <span className="text-ink-3">{pool.length}</span> {searching ? "shown" : "entities"}
            </>
          ) : (
            <>
              <span className="text-ink-3">{ENTITIES.length}</span> entities ·{" "}
              <span className="text-amber">{decisions.length}</span> to sort out ·{" "}
              <span className="text-ink-3">{NEW_ENTITIES.length}</span> new this week
            </>
          )}
        </span>
      </div>

      {/* body */}
      {showGrid ? (
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          {pool.length > 0 ? (
            <div className="[column-gap:14px] columns-[250px]">
              {pool.map((e) => (
                <EntityCard key={e.name} item={toCard(e)} onOpen={() => {}} />
              ))}
            </div>
          ) : (
            <p className="text-[13px] text-ink-3">No entity matches “{query.trim()}”.</p>
          )}
        </div>
      ) : (
        <div className="flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto px-6 py-5 lg:flex-row">
          {/* main */}
          <div className="min-w-0 flex-1 space-y-7">
            <section>
              <SectionHead title="Decisions" hint="same, or just similar?" count={`${decisions.length} of 9`} />
              {decisions.length > 0 ? (
                <div className="space-y-2.5">
                  {decisions.map((d) => (
                    <DecisionCard key={d.id} d={d} onDecide={() => drop(d.id)} />
                  ))}
                  <button className="w-full rounded-[10px] border border-dashed border-line py-2 font-mono text-[11px] text-ink-4 transition-colors hover:border-amber-line hover:text-ink-2">
                    review all 9 →
                  </button>
                </div>
              ) : (
                <p className="rounded-[12px] border border-line-2 bg-card px-4 py-6 text-center text-[12px] text-ink-4">
                  Inbox zero — nothing to sort out.
                </p>
              )}
            </section>

            <section>
              <SectionHead title="Recently active" hint="what moved" />
              <div className="flex flex-col">
                {ACTIVITY.map((a) => (
                  <ActivityRow key={a.id} a={a} />
                ))}
              </div>
            </section>
          </div>

          {/* rail — stacks under the main column below lg */}
          <div className="w-full space-y-7 lg:w-[300px] lg:shrink-0">
            <section>
              <SectionHead title="New this week" count={String(NEW_ENTITIES.length)} />
              <div className="flex flex-col">
                {NEW_ENTITIES.map((n) => (
                  <NewEntityRow key={n.id} n={n} />
                ))}
              </div>
            </section>

            <section>
              <SectionHead title="Loose threads" hint="unlinked" count="23" />
              <div className="flex flex-wrap gap-1.5">
                {LOOSE.map((l) => (
                  <button
                    key={l.name}
                    className="group flex items-center gap-1 rounded-chip border border-line-2 bg-bg px-2 py-1 font-mono text-[10.5px] text-ink-3 transition-colors hover:border-amber-line hover:text-ink"
                  >
                    <Link2 size={10} strokeWidth={1.8} className="text-ink-4 group-hover:text-amber" />
                    {l.name}
                  </button>
                ))}
              </div>
              <button className="mt-2.5 flex items-center gap-1 font-mono text-[10px] text-ink-4 hover:text-ink-2">
                <Plus size={11} strokeWidth={1.8} /> weave a thread
              </button>
            </section>
          </div>
        </div>
      )}
    </div>
  );
}

export function EntitiesInboxSpike() {
  const theme = useAppStore((s) => s.theme);
  return (
    <div className="flex h-screen flex-col bg-bg text-ink">
      <div className="flex items-center gap-3 border-b border-line bg-chrome px-4 py-2 text-xs text-ink-3">
        <span className="font-mono">/spike/entities-inbox</span>
        <span className="text-ink-4">·</span>
        <span>theme: {theme}</span>
        <button
          className="rounded-chip border border-line px-2 py-0.5 hover:bg-raise hover:text-ink"
          onClick={() => useAppStore.getState().setTheme(theme === "dark" ? "soft" : "dark")}
        >
          toggle theme
        </button>
      </div>
      <div className="min-h-0 flex-1 overflow-hidden">
        <EntitiesHome />
      </div>
    </div>
  );
}
