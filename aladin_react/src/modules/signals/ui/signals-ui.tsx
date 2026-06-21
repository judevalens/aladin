import { Radio } from "lucide-react";

import { cn } from "@/lib/utils";
import { useSignals } from "@/modules/signals/hooks/use-signals";
import type { ClaimSignal, SignalSort } from "@/modules/signals/signal-types";

const SORT_TABS: { key: SignalSort; label: string }[] = [
  { key: "recent", label: "Recent" },
  { key: "top", label: "Top" },
];

function polarityHue(polarity: string): string {
  switch (polarity) {
    case "deny":
      return "text-against";
    case "neutral":
      return "text-ink-3";
    default:
      return "text-for";
  }
}

function SignalCard({ signal }: { signal: ClaimSignal }) {
  return (
    <article className="rounded-card border border-line bg-card p-4">
      <div className="mb-1.5 flex flex-wrap items-center gap-2">
        <span className={cn("font-mono text-[10px] uppercase tracking-[0.12em]", polarityHue(signal.polarity))}>
          {signal.polarity}
        </span>
        <span className="rounded-chip border border-line px-2 py-0.5 font-mono text-[10px] text-ink-3">
          {signal.trustTier}
        </span>
        {signal.subjects.map((s) => (
          <span key={s.id} className="text-[12px] text-ink-3">
            · {s.name}
          </span>
        ))}
      </div>

      <h3 className="font-display text-[15px] leading-snug text-ink">{signal.text}</h3>

      <div className="mt-3 flex flex-wrap items-center gap-3 font-mono text-[11px]">
        {signal.assertCount > 0 ? (
          <span className="text-for">{signal.assertCount} for</span>
        ) : null}
        {signal.denyCount > 0 ? (
          <span className="text-against">{signal.denyCount} against</span>
        ) : null}
        {signal.contradicts > 0 ? (
          <span className="text-ink-4">{signal.contradicts} contradicts</span>
        ) : null}
        {signal.supports > 0 ? (
          <span className="text-ink-4">{signal.supports} supports</span>
        ) : null}
        {signal.qualifies > 0 ? (
          <span className="text-ink-4">{signal.qualifies} qualifies</span>
        ) : null}
        <span className="ml-auto text-ink-4">signal {Math.round(signal.signalScore)}</span>
      </div>
    </article>
  );
}

export function SignalsUI() {
  const { signals, loading, error, sort, setSort } = useSignals();

  return (
    <div className="flex min-w-0 flex-1 flex-col overflow-hidden bg-bg">
      <header className="flex items-center gap-3 border-b border-line bg-panel px-4 py-2.5">
        <Radio className="size-4 text-amber" />
        <span className="font-display text-[13px] text-ink">Signals</span>
        <div className="ml-auto flex items-center gap-1">
          {SORT_TABS.map((tab) => (
            <button
              key={tab.key}
              type="button"
              onClick={() => setSort(tab.key)}
              className={cn(
                "rounded-chip px-2.5 py-1 text-[12px] transition-colors",
                sort === tab.key
                  ? "bg-[rgb(var(--sel))] text-ink"
                  : "text-ink-3 hover:bg-[rgb(var(--hover))] hover:text-ink",
              )}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </header>

      <div className="min-h-0 flex-1 overflow-auto px-6 py-6">
        <div className="mx-auto w-full max-w-[760px]">
          {loading ? (
            <p className="mt-10 text-center text-[13px] text-ink-4">Loading…</p>
          ) : error ? (
            <p className="mt-10 text-center text-[13px] text-against">{error}</p>
          ) : signals.length === 0 ? (
            <p className="mt-10 text-center text-[13px] text-ink-4">
              No signals yet. Claims appear here as sources are ingested and resolved.
            </p>
          ) : (
            <div className="flex flex-col gap-3">
              {signals.map((signal) => (
                <SignalCard key={signal.id} signal={signal} />
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
