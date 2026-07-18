import { useEffect, useRef, useState } from "react";

import { useAppComposition } from "@/app/composition/app-composition";
import { cn } from "@/lib/utils";
import type { EntityHit } from "@/modules/graph/graph-pane-types";
import { REL, REL_OPTIONS } from "../entity-context-vocab";

// "Draw a connection" (PRD §4.2): pick a relation type + a target entity + say why.
// On confirm the caller writes a YOURS edge. Three deliberate details:
//   - the target picker is the alias-aware entity search, so typing a synonym finds the
//     entity and each row shows its other surfaces — you can't accidentally connect to a
//     duplicate;
//   - `why` is required: an edge without reasoning is a backlink, which the PRD explicitly
//     rejects — the reasoning IS the substance;
//   - the focused entity is excluded from results (no self-edges).
export function DrawConnectionUI({
  selfId,
  onCancel,
  onConfirm,
}: {
  selfId?: string;
  onCancel: () => void;
  onConfirm: (edge: { rel: string; toId: string; why: string }) => Promise<void>;
}) {
  const { repos } = useAppComposition();
  const [rel, setRel] = useState(REL_OPTIONS[0]);
  const [query, setQuery] = useState("");
  const [hits, setHits] = useState<EntityHit[]>([]);
  const [target, setTarget] = useState<EntityHit | null>(null);
  const [why, setWhy] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const seq = useRef(0);

  // Debounced + race-guarded: only the latest query's results may land.
  useEffect(() => {
    if (target || query.trim() === "") {
      setHits([]);
      return;
    }
    const mine = ++seq.current;
    const timer = setTimeout(() => {
      repos.graphPane
        .searchEntities(query)
        .then((result) => {
          if (mine !== seq.current) return;
          setHits(result.filter((h) => h.id !== selfId));
        })
        .catch(() => {
          if (mine === seq.current) setHits([]);
        });
    }, 150);
    return () => clearTimeout(timer);
  }, [repos, query, target, selfId]);

  const ready = Boolean(target && why.trim() && !saving);

  const confirm = async () => {
    if (!target || !why.trim()) return;
    setSaving(true);
    setError(null);
    try {
      await onConfirm({ rel, toId: target.id, why: why.trim() });
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Could not draw the connection");
      setSaving(false);
    }
  };

  return (
    <div className="mb-[34px] rounded-[10px] border border-amber-line bg-card p-[13px]">
      {/* relation type */}
      <div className="mb-3 flex flex-wrap gap-1.5">
        {REL_OPTIONS.map((key) => (
          <button
            key={key}
            type="button"
            onClick={() => setRel(key)}
            className={cn(
              "rounded-chip border px-2 py-1 font-mono text-[9.5px] font-bold uppercase tracking-[0.4px] transition-colors",
              rel === key
                ? "border-amber-line bg-amber-soft text-amber"
                : "border-line text-ink-3 hover:border-amber-line",
            )}
          >
            {REL[key].glyph} {REL[key].label}
          </button>
        ))}
      </div>

      {/* target entity */}
      {target ? (
        <div className="mb-3 flex items-center gap-2">
          <span className="font-display text-[14px] font-semibold text-ink">{target.name}</span>
          <span className="font-mono text-[9px] text-ink-4">{target.kind}</span>
          <button
            type="button"
            onClick={() => {
              setTarget(null);
              setQuery("");
            }}
            className="font-mono text-[9px] text-ink-3 underline hover:text-ink"
          >
            change
          </button>
        </div>
      ) : (
        <div className="mb-3">
          <input
            autoFocus
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Connect to…"
            className="w-full rounded-chip border border-line bg-field px-2.5 py-1.5 text-[13px] text-ink outline-none placeholder:text-ink-4 focus:border-amber-line"
          />
          {hits.length > 0 && (
            <div className="mt-1.5 flex flex-col gap-px">
              {hits.map((hit) => (
                <button
                  key={hit.id}
                  type="button"
                  onClick={() => setTarget(hit)}
                  className="flex items-baseline gap-2 rounded-chip px-2 py-1.5 text-left hover:bg-raise"
                >
                  <span className="font-display text-[13px] font-semibold text-ink">
                    {hit.name}
                  </span>
                  <span className="font-mono text-[9px] text-ink-4">{hit.kind}</span>
                  {hit.aliases.length > 0 && (
                    <span className="truncate font-mono text-[9px] text-ink-4">
                      · {hit.aliases.join(", ")}
                    </span>
                  )}
                  {hit.trustTier === "placeholder" && (
                    <span className="font-mono text-[8.5px] text-ink-4">unresolved</span>
                  )}
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {/* why — the substance */}
      <textarea
        value={why}
        onChange={(e) => setWhy(e.target.value)}
        rows={2}
        placeholder="Why? — the reasoning is the substance"
        className="mb-3 w-full resize-none rounded-chip border border-line bg-field px-2.5 py-1.5 text-[12px] leading-[1.5] text-ink outline-none placeholder:text-ink-4 focus:border-amber-line"
      />

      {error && <p className="mb-2 text-[11.5px] text-against">{error}</p>}

      <div className="flex items-center gap-2">
        <button
          type="button"
          disabled={!ready}
          onClick={confirm}
          className="cursor-pointer rounded-chip bg-amber px-[11px] py-1.5 text-[11.5px] font-semibold text-primary-foreground transition hover:brightness-[1.08] disabled:cursor-not-allowed disabled:opacity-40"
        >
          {saving ? "Drawing…" : "Draw"}
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="cursor-pointer rounded-chip border border-line px-[10px] py-1.5 text-[11.5px] font-semibold text-ink-2 transition hover:brightness-[1.08]"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}
