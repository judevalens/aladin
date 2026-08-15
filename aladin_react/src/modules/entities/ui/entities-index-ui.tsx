import {
  ArrowLeftRight,
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
import { Eyebrow } from "@/components/ui/eyebrow";
import { Icon } from "@/components/ui/icon";
import { useMemo, useState } from "react";

import { cn } from "@/lib/utils";
import { useEntityList } from "@/modules/entities/hooks/use-entity-list";
import { useMergeQueue } from "@/modules/entities/hooks/use-merge-queue";
import { EntityCard, EntityGlyph } from "@/modules/entities/ui/entity-card";
import { QueueCard } from "@/modules/entities/ui/entities-inbox-ui";
import { EntityPeek } from "@/modules/entities/ui/entity-peek-ui";

// The Entities landing — a top-down DASHBOARD by default, not a wall of cards. You don't
// graze a grid of entities; you come here to see what needs a decision, what moved, and
// what just arrived. Search is the way IN to the layer: typing opens the card GRID,
// filtered — so the grid is the search surface. The Home/Grid toggle switches between the
// dashboard and browsing everything by hand.
//
// Real vs. mock: Decisions (the merge queue), the grid/search, and Loose threads are wired
// to real data. "Recently active" (needs a change-log) and "New this week" (needs
// created_at) have no producer yet, so they're MOCK placeholders that hold the idea until
// wired. Plan: memory project_entity_context_surface.

type EntityView = "overview" | "grid";

// ── MOCK — no "what moved" log / created_at yet; placeholders that keep the shape ───────
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

// ── section atoms ──────────────────────────────────────────────────────────────────
function SectionHead({ title, hint, count }: { title: string; hint?: string; count?: string }) {
  return (
    <div className="mb-2.5 flex items-center gap-2">
      <span className="text-body font-semibold text-ink">{title}</span>
      {hint && <span className="font-mono text-meta text-ink-4">{hint}</span>}
      <span className="h-px flex-1 bg-line-2" />
      {count && <span className="font-mono text-meta text-ink-4">{count}</span>}
    </div>
  );
}

// Home / Grid switch — the dashboard is the default; the grid is the browse-everything
// (and search-results) view.
function ViewToggle({ view, onView }: { view: EntityView; onView: (v: EntityView) => void }) {
  const btn = (v: EntityView, Glyph: typeof LayoutGrid, label: string) => (
    <button
      type="button"
      title={label}
      aria-label={label}
      aria-pressed={view === v}
      onClick={() => onView(v)}
      className={cn(
        "flex items-center justify-center rounded-chip px-2 py-1 transition-colors",
        view === v ? "bg-sel text-ink" : "text-ink-3 hover:bg-hover hover:text-ink",
      )}
    >
      <Icon as={Glyph} size="inline" />
    </button>
  );
  return (
    <div className="flex items-center gap-0.5">
      {btn("overview", Home, "Overview")}
      {btn("grid", LayoutGrid, "Grid")}
    </div>
  );
}

function ActivityRow({ a }: { a: (typeof ACTIVITY)[number] }) {
  const Glyph = a.icon;
  return (
    <div className="group flex w-full items-center gap-3 rounded-control px-2 py-2 text-left">
      <EntityGlyph kind={a.kind} name={a.name} size={26} />
      <span className="min-w-0 flex-1">
        <span className="flex items-baseline gap-2">
          <span className="truncate font-display text-body font-semibold text-ink">{a.name}</span>
          <span className="flex shrink-0 items-center gap-1 font-mono text-meta text-ink-3">
            <Icon as={Glyph} size="inline" /> {a.change}
          </span>
        </span>
        <span className="mt-0.5 block truncate text-small text-ink-4">{a.detail}</span>
      </span>
      <span className="shrink-0 font-mono text-meta text-ink-4">{a.time}</span>
    </div>
  );
}

function NewEntityRow({ n }: { n: (typeof NEW_ENTITIES)[number] }) {
  return (
    <div className="flex w-full items-start gap-2.5 rounded-control px-2 py-2 text-left">
      <EntityGlyph kind={n.kind} name={n.name} size={24} />
      <span className="min-w-0 flex-1">
        <span className="flex items-baseline gap-1.5">
          <span className="truncate font-display text-small font-semibold text-ink">{n.name}</span>
          <span className="shrink-0 font-mono text-meta text-ink-4">{n.time}</span>
        </span>
        <span className="mt-0.5 line-clamp-1 text-meta text-ink-4">{n.gist}</span>
      </span>
    </div>
  );
}

// The full masonry grid — browse everything, and the surface search opens into. Search
// filters it live.
function EntityGrid({
  list,
  onOpen,
}: {
  list: ReturnType<typeof useEntityList>;
  onOpen: (id: string) => void;
}) {
  if (list.error) return <p className="text-body text-ink-2">{list.error}</p>;
  if (list.loading && list.entities.length === 0) {
    return (
      <div className="[column-gap:14px] columns-[250px]">
        {Array.from({ length: 8 }).map((_, i) => (
          <div key={i} className="mb-3.5 h-[150px] break-inside-avoid rounded-modal bg-card" />
        ))}
      </div>
    );
  }
  if (list.entities.length === 0) {
    return (
      <p className="text-body text-ink-3">
        {list.query.trim()
          ? `No entity matches “${list.query.trim()}”.`
          : "No entities yet — they appear as you tag pages, @mention things, or ingest sources."}
      </p>
    );
  }
  return (
    <div className="[column-gap:14px] columns-[250px]">
      {list.entities.map((item) => (
        <EntityCard key={item.id} item={item} onOpen={() => onOpen(item.id)} />
      ))}
    </div>
  );
}

export function EntitiesIndexUI() {
  const [peekId, setPeekId] = useState<string | null>(null);
  const [showAllDecisions, setShowAllDecisions] = useState(false);
  const [view, setView] = useState<EntityView>("overview");

  const list = useEntityList();
  const queue = useMergeQueue();

  const searching = list.query.trim().length > 0;
  // Typing opens the grid: search always resolves to the (filtered) card grid.
  const showGrid = view === "grid" || searching;

  // Home lands on the dashboard — clear any query so it isn't stuck showing the grid.
  const selectView = (v: EntityView) => {
    setView(v);
    if (v === "overview") list.setQuery("");
  };

  // Loose threads = real entities with no connections yet. Only meaningful on the
  // dashboard, when the list is the whole (unfiltered) layer.
  const loose = useMemo(
    () => (showGrid ? [] : list.entities.filter((e) => e.links === 0)),
    [showGrid, list.entities],
  );

  const decisions = showAllDecisions ? queue.items : queue.items.slice(0, 3);

  return (
    <div className="flex h-full w-full min-w-0 flex-col overflow-hidden">
      {/* header — search + the Home/Grid toggle */}
      <div className="flex items-center gap-3 border-b border-line px-5 pt-4 pb-3">
        <Icon as={Globe} className="text-ink-3" />
        <Eyebrow as="span">Entities</Eyebrow>
        <div className="relative ml-1 w-72">
          <Icon as={Search} size="inline" className="absolute left-2.5 top-1/2 -translate-y-1/2 text-ink-4" />
          <input
            value={list.query}
            onChange={(e) => list.setQuery(e.target.value)}
            placeholder="Search a ticker, company, thesis…"
            className="w-full rounded-chip border border-line bg-field py-1.5 pl-8 pr-7 text-small text-ink outline-none placeholder:text-ink-4 focus:border-amber-line"
          />
          {searching && (
            <button
              type="button"
              onClick={() => list.setQuery("")}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-ink-4 hover:text-ink-2"
            >
              <Icon as={X} size="inline" mark />
            </button>
          )}
        </div>
        <ViewToggle view={showGrid ? "grid" : "overview"} onView={selectView} />
        <span className="ml-auto font-mono text-meta text-ink-4">
          {showGrid ? (
            <>
              <span className="text-ink-3">{list.entities.length}</span> {searching ? "shown" : "entities"}
            </>
          ) : (
            <>
              <span className="text-ink-3">{list.summary.total}</span> entities ·{" "}
              <span className="text-amber">{queue.items.length}</span> to sort out ·{" "}
              <span className="text-ink-3">{NEW_ENTITIES.length}</span> new this week
            </>
          )}
        </span>
      </div>

      {/* body */}
      {showGrid ? (
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          <EntityGrid list={list} onOpen={setPeekId} />
        </div>
      ) : (
        <div className="flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto px-6 py-5 lg:flex-row">
          {/* main */}
          <div className="min-w-0 flex-1 space-y-7">
            <section>
              <SectionHead
                title="Decisions"
                hint="same, or just similar?"
                count={queue.items.length > 0 ? `${decisions.length} of ${queue.items.length}` : undefined}
              />
              {queue.error ? (
                <p className="text-small text-ink-2">{queue.error}</p>
              ) : queue.loading ? (
                <div className="space-y-2.5">
                  {[0, 1, 2].map((i) => (
                    <div key={i} className="h-[104px] animate-pulse rounded-card bg-card" />
                  ))}
                </div>
              ) : queue.items.length === 0 ? (
                <p className="rounded-card border border-line-2 bg-card px-4 py-6 text-center text-small text-ink-4">
                  Inbox zero — nothing to sort out.
                </p>
              ) : (
                <div className="space-y-2.5">
                  {decisions.map((item) => (
                    <QueueCard
                      key={item.mergeId}
                      item={item}
                      onAccept={() => queue.accept(item)}
                      onReject={() => queue.reject(item)}
                      onOpen={() => setPeekId(item.fromId)}
                    />
                  ))}
                  {!showAllDecisions && queue.items.length > decisions.length && (
                    <button
                      type="button"
                      onClick={() => setShowAllDecisions(true)}
                      className="w-full rounded-control border border-dashed border-line py-2 font-mono text-meta text-ink-4 transition-colors hover:border-amber-line hover:text-ink-2"
                    >
                      review all {queue.items.length} →
                    </button>
                  )}
                </div>
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
              <SectionHead title="Loose threads" hint="unlinked" count={loose.length > 0 ? String(loose.length) : undefined} />
              {loose.length > 0 ? (
                <>
                  <div className="flex flex-wrap gap-1.5">
                    {loose.slice(0, 16).map((e) => (
                      <button
                        key={e.id}
                        type="button"
                        onClick={() => setPeekId(e.id)}
                        className="group flex items-center gap-1 rounded-chip border border-line-2 bg-bg px-2 py-1 font-mono text-meta text-ink-3 transition-colors hover:border-amber-line hover:text-ink"
                      >
                        <Icon as={Link2} size="inline" className="text-ink-4 group-hover:text-amber" />
                        {e.name}
                      </button>
                    ))}
                  </div>
                  <button
                    type="button"
                    className="mt-2.5 flex items-center gap-1 font-mono text-meta text-ink-4 hover:text-ink-2"
                  >
                    <Icon as={Plus} size="inline" /> weave a thread
                  </button>
                </>
              ) : (
                <p className="text-small text-ink-4">Everything's connected — no loose threads.</p>
              )}
            </section>
          </div>
        </div>
      )}

      {/* No refetch on close (prototyping); reactive dataEvents pipeline for entities deferred. */}
      <EntityPeek entityId={peekId} onClose={() => setPeekId(null)} />
    </div>
  );
}

export const EntitiesIcon = Globe;
