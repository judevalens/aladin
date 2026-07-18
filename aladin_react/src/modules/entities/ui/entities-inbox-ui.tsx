import { ArrowLeftRight, Check, Inbox, X } from "lucide-react";
import { useState } from "react";

import { cn } from "@/lib/utils";
import type { MergeQueueItem } from "@/modules/entities/entity-list-types";

// The Entities inbox — the default landing view. The entity layer's automation (the
// resolver + LLM judge) generates a stream of questions it can't settle alone; this is
// where you settle them. The page has a JOB (keep the layer clean), and a loop worth
// returning to, rather than being a bare wall of cards.

const SUGGESTION: Record<string, { label: string; className: string }> = {
  synonym: { label: "likely the same", className: "text-for" },
  distinct: { label: "likely distinct", className: "text-ink-3" },
  unsure: { label: "your call", className: "text-ink-4" },
};

function EntityRef({ name, kind }: { name: string; kind: string }) {
  return (
    <span className="inline-flex min-w-0 items-baseline gap-1.5">
      <span className="truncate font-display text-[14px] font-semibold text-ink">{name}</span>
      <span className="shrink-0 font-mono text-[9px] text-ink-4">{kind}</span>
    </span>
  );
}

export function QueueCard({
  item,
  onAccept,
  onReject,
  onOpen,
}: {
  item: MergeQueueItem;
  onAccept: () => Promise<void>;
  onReject: () => Promise<void>;
  onOpen: () => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const meta = SUGGESTION[item.suggestion] ?? SUGGESTION.unsure;

  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    setError(null);
    try {
      await fn();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Could not record that");
      setBusy(false);
    }
  };

  return (
    <div className="rounded-[12px] border border-line-2 bg-card p-3.5">
      <div className="flex items-center gap-2">
        <button type="button" onClick={onOpen} className="min-w-0 flex-1 text-left hover:opacity-80">
          <div className="flex items-center gap-2.5">
            <EntityRef name={item.fromName} kind={item.fromKind} />
            <ArrowLeftRight size={13} strokeWidth={1.9} className="shrink-0 text-ink-4" />
            <EntityRef name={item.intoName} kind={item.intoKind} />
          </div>
        </button>
        <span className="shrink-0 font-mono text-[10px] text-ink-3">
          {Math.round(item.confidence * 100)}%
        </span>
      </div>

      <div className="mt-1.5 flex items-center gap-2 font-mono text-[9.5px]">
        <span className={meta.className}>{meta.label}</span>
        <span className="text-ink-4">· {item.why}</span>
      </div>

      {error ? <p className="mt-1.5 text-[11px] text-against">{error}</p> : null}

      <div className="mt-3 flex items-center gap-2">
        <button
          type="button"
          disabled={busy}
          onClick={() => run(onAccept)}
          className="flex cursor-pointer items-center gap-1 rounded-chip bg-amber px-2.5 py-1 text-[11px] font-semibold text-primary-foreground transition hover:brightness-[1.08] disabled:opacity-40"
        >
          <Check size={12} strokeWidth={2.4} />
          Merge
        </button>
        <button
          type="button"
          disabled={busy}
          onClick={() => run(onReject)}
          className="flex cursor-pointer items-center gap-1 rounded-chip border border-line px-2.5 py-1 text-[11px] font-semibold text-ink-2 transition hover:brightness-[1.08] disabled:opacity-40"
        >
          <X size={12} strokeWidth={2.4} />
          Keep separate
        </button>
        <button
          type="button"
          onClick={onOpen}
          className="ml-auto cursor-pointer font-mono text-[10px] text-ink-4 hover:text-ink-2"
        >
          view →
        </button>
      </div>
    </div>
  );
}

export function EntitiesInboxUI({
  items,
  loading,
  error,
  onAccept,
  onReject,
  onOpen,
}: {
  items: MergeQueueItem[];
  loading: boolean;
  error: string | null;
  onAccept: (item: MergeQueueItem) => Promise<void>;
  onReject: (item: MergeQueueItem) => Promise<void>;
  onOpen: (entityId: string) => void;
}) {
  return (
    <div className="mx-auto max-w-[620px] px-5 py-5">
      <div className="mb-4 flex items-baseline gap-2.5">
        <span className="font-display text-[15px] font-semibold text-ink">Same, or just similar?</span>
        <span className="font-mono text-[10px] text-ink-4">
          {items.length} {items.length === 1 ? "decision" : "decisions"} waiting
        </span>
      </div>

      {error ? (
        <p className="text-[13px] text-ink-2">{error}</p>
      ) : loading ? (
        <div className="flex flex-col gap-2.5">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-[104px] animate-pulse rounded-[12px] bg-card" />
          ))}
        </div>
      ) : items.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-[12px] border border-line-2 bg-card px-5 py-10 text-center">
          <Inbox size={22} strokeWidth={1.6} className="text-ink-4" />
          <p className="text-[13px] font-medium text-ink-2">Inbox zero</p>
          <p className="max-w-xs text-[12px] leading-[1.5] text-ink-4">
            No identities to sort out. New questions appear here as the resolver finds
            possible duplicates.
          </p>
        </div>
      ) : (
        <div className="flex flex-col gap-2.5">
          {items.map((item) => (
            <QueueCard
              key={item.mergeId}
              item={item}
              onAccept={() => onAccept(item)}
              onReject={() => onReject(item)}
              onOpen={() => onOpen(item.fromId)}
            />
          ))}
        </div>
      )}
    </div>
  );
}
