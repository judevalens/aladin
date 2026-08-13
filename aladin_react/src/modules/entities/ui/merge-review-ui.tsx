import { Link2 } from "lucide-react";
import { Icon } from "@/components/ui/icon";
import { useState } from "react";

import { cn } from "@/lib/utils";
import type { PendingMerge } from "@/modules/entities/entity-context-types";

// "Same, or just similar?" — the judge's open questions about this entity's identity.
//
// This panel is where the entity layer's automation becomes accountable: the resolver
// and the LLM judge propose merges, but anything they can't settle confidently waits
// here for a human. Every proposal shows its evidence and the judge's own reasoning, so
// the decision is informed rather than a coin flip — and both outcomes are recorded
// (a rejection is negative evidence, so the pair is never proposed again).

const SUGGESTION_META: Record<string, { label: string; className: string }> = {
  synonym: { label: "suggested: synonym", className: "text-for" },
  distinct: { label: "suggested: distinct", className: "text-ink-3" },
  unsure: { label: "not sure — your call", className: "text-ink-4" },
};

function MergeRow({
  merge,
  onAccept,
  onReject,
}: {
  merge: PendingMerge;
  onAccept: () => Promise<void>;
  onReject: () => Promise<void>;
}) {
  const [busy, setBusy] = useState<"accept" | "reject" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const meta = SUGGESTION_META[merge.suggestion] ?? SUGGESTION_META.unsure;

  const run = async (which: "accept" | "reject", fn: () => Promise<void>) => {
    setBusy(which);
    setError(null);
    try {
      await fn();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Could not record that");
      setBusy(null);
    }
  };

  return (
    <div className="border-t border-line-2 py-3">
      <div className="flex items-baseline gap-2">
        <span className="font-display text-lead font-semibold text-ink">
          {merge.otherName}
        </span>
        <span className="font-mono text-meta text-ink-4">{merge.otherKind}</span>
        <span className="flex-1" />
        <span className="font-mono text-meta text-ink-3">
          {Math.round(merge.confidence * 100)}%
        </span>
      </div>

      <div className="mt-1 font-mono text-meta text-ink-4">{merge.why}</div>
      <div className={cn("mt-0.5 font-mono text-meta", meta.className)}>{meta.label}</div>
      {merge.reason ? (
        <p className="mt-1 text-small leading-[1.45] text-pretty text-ink-3">
          “{merge.reason}”
        </p>
      ) : null}

      {error ? <p className="mt-1.5 text-meta text-against">{error}</p> : null}

      <div className="mt-2 flex items-center gap-2">
        <button
          type="button"
          disabled={busy !== null}
          onClick={() => run("accept", onAccept)}
          className="cursor-pointer rounded-chip bg-amber px-2.5 py-1 text-meta font-semibold text-primary-foreground transition hover:brightness-[1.08] disabled:opacity-40"
        >
          {busy === "accept" ? "Merging…" : "Same thing"}
        </button>
        <button
          type="button"
          disabled={busy !== null}
          onClick={() => run("reject", onReject)}
          className="cursor-pointer rounded-chip border border-line px-2.5 py-1 text-meta font-semibold text-ink-2 transition hover:brightness-[1.08] disabled:opacity-40"
        >
          {busy === "reject" ? "Saving…" : "Keep separate"}
        </button>
      </div>
    </div>
  );
}

export function MergeReviewUI({
  merges,
  onAccept,
  onReject,
}: {
  merges: PendingMerge[];
  onAccept: (mergeId: string) => Promise<void>;
  onReject: (mergeId: string) => Promise<void>;
}) {
  // No open questions is the normal, healthy state — render nothing rather than an
  // empty panel implying something is missing.
  if (merges.length === 0) return null;

  return (
    <div className="mb-8 rounded-control border border-line-2 bg-card p-3">
      <div className="flex items-center gap-2">
        <Icon as={Link2} size="inline" className="shrink-0 text-ink-3" />
        <span className="font-display text-body font-semibold text-ink">
          Same, or just similar?
        </span>
        <span className="flex-1" />
        <span className="font-mono text-meta text-ink-4">{merges.length}</span>
      </div>
      <p className="mt-1 text-small leading-[1.45] text-ink-3">
        Identity is graded — synonyms fold into one entity, a different thing stays its
        own. Merges are reversible.
      </p>
      <div className="mt-1">
        {merges.map((m) => (
          <MergeRow
            key={m.mergeId}
            merge={m}
            onAccept={() => onAccept(m.mergeId)}
            onReject={() => onReject(m.mergeId)}
          />
        ))}
      </div>
    </div>
  );
}
