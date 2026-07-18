import { HelpCircle, Link2, Plus, Quote, Waypoints } from "lucide-react";
import { useMemo } from "react";

import { cn } from "@/lib/utils";
import type { ContextItem, Edge, Entity, Tone } from "../entity-context-types";
import { CTX_META, groupEdgesByRel, relMeta } from "../entity-context-vocab";
import { PlatChip } from "./plat-chip";

// Entity Context surface — Phase A: pixel-faithful recreation of
// design/ENTITY_CONTEXT_PRD.md + the handoff README on mock data.
// Relationship-first: identity → pending suggestion → typed edges (the lead)
// → raw accreted context (verbatim, underneath).

// Semantic tone keys → Aladin theme tokens. Glyph chips are tone at ~11% bg /
// ~25% border (handoff `1c`/`40` hex alphas); ctx icon chips are tone at ~9%.
const TONE: Record<Tone, { text: string; glyphChip: string; ctxChip: string }> = {
  for: {
    text: "text-for",
    glyphChip: "border-for/25 bg-for/10 text-for",
    ctxChip: "bg-for/10 text-for",
  },
  against: {
    text: "text-against",
    glyphChip: "border-against/25 bg-against/10 text-against",
    ctxChip: "bg-against/10 text-against",
  },
  catalyst: {
    text: "text-catalyst",
    glyphChip: "border-catalyst/25 bg-catalyst/10 text-catalyst",
    ctxChip: "bg-catalyst/10 text-catalyst",
  },
  echo: {
    text: "text-echo",
    glyphChip: "border-echo/25 bg-echo/10 text-echo",
    ctxChip: "bg-echo/10 text-echo",
  },
  amber: {
    text: "text-amber",
    glyphChip: "border-amber/25 bg-amber/10 text-amber",
    ctxChip: "bg-amber/10 text-amber",
  },
  qualify: {
    text: "text-qualify",
    glyphChip: "border-qualify/25 bg-qualify/10 text-qualify",
    ctxChip: "bg-qualify/10 text-qualify",
  },
  neutral: {
    text: "text-neutral",
    glyphChip: "border-neutral/25 bg-neutral/10 text-neutral",
    ctxChip: "bg-neutral/10 text-neutral",
  },
  ink2: {
    text: "text-ink-2",
    glyphChip: "border-ink-2/25 bg-ink-2/10 text-ink-2",
    ctxChip: "bg-ink-2/10 text-ink-2",
  },
};

const CTX_ICON = { source: Quote, add: Plus, question: HelpCircle } as const;

// ── relational signal: what changed in the STRUCTURE ─────────────────────────
function RelSignal({
  edge,
  onKeep,
  onDismiss,
}: {
  edge: Edge;
  onKeep: () => void;
  onDismiss: () => void;
}) {
  return (
    <div className="mb-5 flex items-center gap-3 rounded-[11px] border border-amber-line bg-amber-soft px-[15px] py-[11px]">
      <Link2 size={16} strokeWidth={1.9} className="shrink-0 text-amber" />
      <div className="flex-1 text-[13px]">
        <span className="font-medium text-ink">New connection formed · </span>
        <span className="text-ink-2">your watcher wired </span>
        <span className="font-display font-semibold text-ink">{edge.to}</span>
        <span className="text-ink-2"> into this concept as supporting evidence.</span>
      </div>
      <button
        type="button"
        onClick={onKeep}
        className="shrink-0 cursor-pointer rounded-chip bg-amber px-[11px] py-1.5 text-[11.5px] font-semibold text-primary-foreground transition hover:brightness-[1.08]"
      >
        Keep edge
      </button>
      <button
        type="button"
        onClick={onDismiss}
        className="shrink-0 cursor-pointer rounded-chip border border-line bg-transparent px-[10px] py-1.5 text-[11.5px] font-semibold text-ink-2 transition hover:brightness-[1.08]"
      >
        Dismiss
      </button>
    </div>
  );
}

// ── one relationship row ─────────────────────────────────────────────────────
function EdgeRow({ edge, onOpen }: { edge: Edge; onOpen?: (entityId: string) => void }) {
  const m = relMeta(edge.rel);
  const tone = TONE[m.tone];
  const mine = edge.origin === "you";
  const navigable = Boolean(onOpen && edge.toId);
  return (
    <div
      role={navigable ? "link" : undefined}
      tabIndex={navigable ? 0 : undefined}
      onClick={navigable ? () => onOpen?.(edge.toId!) : undefined}
      onKeyDown={
        navigable
          ? (e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onOpen?.(edge.toId!);
              }
            }
          : undefined
      }
      className={cn(
        "flex gap-3 rounded-[10px] border border-line-2 bg-card px-[13px] py-[11px] transition-colors hover:border-amber-line",
        navigable && "cursor-pointer",
      )}
    >
      <span
        className={cn(
          "grid size-[26px] shrink-0 place-items-center rounded-chip border font-mono text-[13px]",
          tone.glyphChip,
        )}
      >
        {m.glyph}
      </span>
      <div className="min-w-0 flex-1">
        <div className="mb-[3px] flex flex-wrap items-baseline gap-2">
          <span
            className={cn(
              "font-mono text-[9.5px] font-bold uppercase tracking-[0.4px]",
              tone.text,
            )}
          >
            {m.label}
          </span>
          <span className="font-display text-[14px] font-semibold tracking-[-0.2px] text-ink">
            {edge.to}
          </span>
          <span className="font-mono text-[9px] text-ink-4">{edge.kind}</span>
          <span className="flex-1" />
          <span
            title={mine ? "you drew this edge" : "a watcher found this"}
            className={cn(
              "rounded-[4px] border px-[5px] py-px font-mono text-[8.5px] font-bold tracking-[0.4px]",
              mine ? "border-amber-line text-amber" : "border-line text-echo",
            )}
          >
            {mine ? "YOURS" : "FOUND"}
          </span>
        </div>
        <div className="text-[12px] leading-[1.5] text-pretty text-ink-2">{edge.why}</div>
      </div>
    </div>
  );
}

// ── one piece of accreted context ────────────────────────────────────────────
function CtxRow({ item }: { item: ContextItem }) {
  const m = CTX_META[item.type];
  const tone = TONE[m.tone];
  const Icon = CTX_ICON[m.icon];
  return (
    <div className="flex gap-[11px] border-t border-line-2 py-[11px]">
      <span
        className={cn(
          "mt-px grid size-[22px] shrink-0 place-items-center rounded-sm",
          tone.ctxChip,
        )}
      >
        <Icon size={12} strokeWidth={1.9} />
      </span>
      <div className="min-w-0 flex-1">
        <div
          className={cn(
            "text-[13px] leading-[1.55] text-pretty",
            item.type === "note" ? "text-ink" : "text-ink-2",
            item.type === "quote" && "border-l-2 border-line pl-[10px]",
          )}
        >
          {item.body}
        </div>
        <div className="mt-[5px] flex items-center gap-[7px]">
          <span
            className={cn(
              "font-mono text-[8.5px] font-bold uppercase tracking-[0.4px]",
              tone.text,
            )}
          >
            {m.label}
          </span>
          {item.platform && <PlatChip p={item.platform} size={14} />}
          <span className="font-mono text-[9.5px] text-ink-4">
            {item.who} · {item.time}
          </span>
        </div>
      </div>
    </div>
  );
}

// ── section header: icon · title · hint · rule · count ──────────────────────
function SectionHeader({
  icon,
  title,
  hint,
  count,
  className,
}: {
  icon: React.ReactNode;
  title: string;
  hint: string;
  count: string;
  className?: string;
}) {
  return (
    <div className={cn("flex items-center gap-[9px]", className)}>
      {icon}
      <span className="font-display text-[15px] font-semibold text-ink">{title}</span>
      <span className="font-mono text-[10px] text-ink-4">{hint}</span>
      <span className="h-px flex-1 bg-line-2" />
      <span className="font-mono text-[10px] text-ink-4">{count}</span>
    </div>
  );
}

export interface EntityContextViewProps {
  entity: Entity;
  edges: Edge[];
  context: ContextItem[];
  /** Pending suggested edge; null → the amber banner doesn't render (PRD §5). */
  suggestion?: Edge | null;
  onKeepEdge?: () => void;
  onDismissSuggestion?: () => void;
  /** Clicking an edge navigates to that entity's own Context page ("pull the thread"). */
  onOpenEntity?: (entityId: string) => void;
  onDrawConnection?: () => void;
  /** When set, replaces the dashed CTA — the draw-a-connection form, rendered in place. */
  drawSlot?: React.ReactNode;
  /**
   * "Same, or just similar?" — the judge's pending identity questions. A sibling of
   * `suggestion` rather than an overload of it: a merge proposal is a question about
   * what this entity IS, not an edge to accept.
   */
  mergeSlot?: React.ReactNode;
}

/**
 * The Entity Context surface, presentational. Rendered by EntityContextUI against the
 * real read-path, and by the /spike/entity-context route against the design mock.
 */
export function EntityContextView({
  entity,
  edges,
  context,
  suggestion = null,
  onKeepEdge,
  onDismissSuggestion,
  onOpenEntity,
  onDrawConnection,
  drawSlot = null,
  mergeSlot = null,
}: EntityContextViewProps) {
  const groupedEdges = useMemo(() => groupEdgesByRel(edges), [edges]);
  return (
    <div
      className={cn(
        "h-full overflow-y-auto bg-panel font-sans",
        "[&::-webkit-scrollbar]:w-[9px] [&::-webkit-scrollbar-track]:bg-transparent",
        "[&::-webkit-scrollbar-thumb]:rounded-[6px] [&::-webkit-scrollbar-thumb]:bg-line",
      )}
    >
      <div className="mx-auto max-w-[820px] px-[30px] pt-[30px] pb-[60px]">
        {/* meta line (provenance header) */}
        <div className="mb-3 flex items-center gap-[9px] font-mono text-[10.5px] text-ink-3">
          <span aria-hidden className="size-[5px] shrink-0 rounded-full border border-echo" />
          <span className="uppercase tracking-[0.6px]">{entity.kind}</span>
          <span className="text-ink-4">·</span>
          <span>{entity.since}</span>
          <span className="flex-1" />
          {entity.watchers.length > 0 && (
            <span className="inline-flex gap-[3px]">
              {entity.watchers.map((p) => (
                <PlatChip key={p} p={p} size={15} />
              ))}
            </span>
          )}
        </div>

        {/* title + gist */}
        <h1 className="mb-[10px] font-display text-[30px] font-semibold tracking-[-0.6px] text-ink">
          {entity.name}
        </h1>
        {entity.gist ? (
          <p className="mb-[26px] max-w-[620px] text-[15px] leading-[1.55] text-pretty text-ink-2">
            {entity.gist}
          </p>
        ) : (
          <div className="mb-[26px]" />
        )}

        {suggestion && (
          <RelSignal
            edge={suggestion}
            onKeep={() => onKeepEdge?.()}
            onDismiss={() => onDismissSuggestion?.()}
          />
        )}

        {/* Identity questions come before the neighborhood: what this entity IS has to
            settle before how it relates means anything. */}
        {mergeSlot}

        {/* RELATIONSHIPS — the lead */}
        <SectionHeader
          className="mb-[13px]"
          icon={<Waypoints size={16} strokeWidth={1.7} className="shrink-0 text-amber" />}
          title="How it relates"
          hint="the understanding lives here"
          count={`${edges.length} edges`}
        />
        {edges.length === 0 ? (
          <p className="mb-4 text-[12px] leading-[1.5] text-ink-3">
            No connections yet — draw the first one below.
          </p>
        ) : (
          <div className="mb-4 flex flex-col gap-4">
            {groupedEdges.map((group) => (
              <div key={group.rel} className="flex flex-col gap-2">
                <div
                  className={cn(
                    "font-mono text-[9.5px] font-bold uppercase tracking-[0.4px]",
                    TONE[relMeta(group.rel).tone].text,
                  )}
                >
                  {relMeta(group.rel).label}
                </div>
                {group.edges.map((e, i) => (
                  <EdgeRow key={`${e.rel}-${e.to}-${i}`} edge={e} onOpen={onOpenEntity} />
                ))}
              </div>
            ))}
          </div>
        )}
        {drawSlot ?? (
          <button
            type="button"
            onClick={onDrawConnection}
            className="mb-[34px] flex w-full cursor-pointer items-center justify-center gap-2 rounded-[10px] border border-dashed border-line bg-transparent p-[10px] text-[12px] font-semibold text-ink-3 transition-colors hover:border-amber-line"
          >
            <Link2 size={14} strokeWidth={1.9} className="shrink-0" />
            Draw a connection…
          </button>
        )}

        {/* CONTEXT — the accreted material, underneath */}
        <SectionHeader
          className="mb-1.5"
          icon={<Quote size={16} strokeWidth={1.7} className="shrink-0 text-ink-2" />}
          title="Context"
          hint="real material · nothing summarized"
          count={`${context.length}`}
        />
        {context.length === 0 ? (
          <p className="border-t border-line-2 pt-[11px] text-[12px] leading-[1.5] text-ink-3">
            Nothing captured yet — quotes, notes, and questions will accrete here.
          </p>
        ) : (
          <div>
            {context.map((c, i) => (
              <CtxRow key={i} item={c} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
