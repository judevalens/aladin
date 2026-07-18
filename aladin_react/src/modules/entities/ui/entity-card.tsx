import { Activity, Globe, Hash, Link2, MapPin, Layers } from "lucide-react";

import { cn } from "@/lib/utils";
import type { EntityListItem } from "@/modules/entities/entity-list-types";

// One card on the Entities index — the rich, design-matched treatment. Visual spec lifted
// from the "Entity Surface" reference (masonry cards); the DATA is Aladin's own, so the
// chips are the entity's aliases and the footer is links/sources (not the reference's
// invented ctx/src vocabulary).

// Relative "updated Nd ago", extended to weeks/months (the shared formatRelativeTime caps
// at days, which reads worse than "6w" at this scale).
function updatedAgo(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const s = (Date.now() - d.getTime()) / 1000;
  if (s < 3600) return `${Math.max(1, Math.round(s / 60))}m ago`;
  if (s < 86400) return `${Math.round(s / 3600)}h ago`;
  const days = Math.round(s / 86400);
  if (days < 14) return `${days}d ago`;
  if (days < 60) return `${Math.round(days / 7)}w ago`;
  return `${Math.round(days / 30)}mo ago`;
}

export function entityInitials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase();
  return name.slice(0, 2).toUpperCase();
}

// The icon/avatar chip. Concepts get a globe, locations a pin, files/"other" a hash;
// orgs and people get an initials monogram — a bit of per-entity identity. An unresolved
// (placeholder) entity tints the chip amber, so its status reads without extra text.
export function EntityGlyph({
  kind,
  name,
  unresolved = false,
  size = 30,
}: {
  kind: string;
  name: string;
  unresolved?: boolean;
  size?: number;
}) {
  const base = cn(
    "grid shrink-0 place-items-center rounded-[9px] border bg-field",
    unresolved ? "border-amber-line text-amber/80" : "border-line-2 text-ink-3",
  );
  const style = { width: size, height: size };
  if (kind === "org" || kind === "person") {
    return (
      <span style={style} className={cn(base, "font-mono text-[10px] font-semibold")}>
        {entityInitials(name)}
      </span>
    );
  }
  const Icon = kind === "location" ? MapPin : kind === "other" ? Hash : kind === "concept" ? Globe : Layers;
  return (
    <span style={style} className={base}>
      <Icon size={Math.round(size / 2)} strokeWidth={1.7} />
    </span>
  );
}

function FootStat({ icon: Icon, value, label }: { icon: typeof Link2; value: number; label: string }) {
  return (
    <span className="flex items-center gap-1">
      <Icon size={11} strokeWidth={1.7} className="text-ink-4" />
      <span className="text-ink-3">{value}</span>
      <span>{label}</span>
    </span>
  );
}

export function EntityCard({ item, onOpen }: { item: EntityListItem; onOpen: () => void }) {
  const unresolved = item.trustTier === "placeholder";
  const ago = updatedAgo(item.updatedAt);

  return (
    <button
      type="button"
      onClick={onOpen}
      className="group mb-[14px] flex w-full break-inside-avoid flex-col rounded-[14px] border border-line-2 bg-card p-[15px] text-left transition-colors hover:border-ink-4 hover:bg-raise"
    >
      {/* header: glyph · name/sub · attention */}
      <div className="flex items-start gap-2.5">
        <EntityGlyph kind={item.kind} name={item.name} unresolved={unresolved} />
        <div className="min-w-0 flex-1">
          <div className="truncate font-display text-[15px] font-semibold tracking-[-0.2px] text-ink">
            {item.name}
          </div>
          <div className="mt-0.5 truncate font-mono text-[10px] text-ink-4">
            {item.kind}
            {ago ? ` · updated ${ago}` : ""}
          </div>
        </div>
        {item.attention > 0 && (
          <span
            title={`${item.attention} open ${item.attention === 1 ? "question" : "questions"}`}
            className="flex shrink-0 items-center gap-1 rounded-[10px] bg-amber-soft px-1.5 py-0.5 font-mono text-[10px] font-semibold text-amber"
          >
            <Activity size={11} strokeWidth={2} />
            {item.attention}
          </span>
        )}
      </div>

      {item.gist ? (
        <p className="mt-2.5 line-clamp-3 text-[12.5px] leading-[1.5] text-pretty text-ink-2">
          {item.gist}
        </p>
      ) : null}

      {item.aliases.length > 0 && (
        <div className="mt-2.5 flex flex-wrap gap-1.5">
          {item.aliases.slice(0, 5).map((a) => (
            <span
              key={a}
              className="rounded-[6px] border border-line-2 bg-bg px-2 py-[3px] font-mono text-[10.5px] text-ink-3"
            >
              {a}
            </span>
          ))}
        </div>
      )}

      <div className="mt-3.5 flex items-center gap-3.5 border-t border-line-2 pt-3 font-mono text-[11px] text-ink-4">
        <FootStat icon={Link2} value={item.links} label={item.links === 1 ? "link" : "links"} />
        <FootStat icon={Layers} value={item.sources} label={item.sources === 1 ? "src" : "sources"} />
      </div>
    </button>
  );
}
