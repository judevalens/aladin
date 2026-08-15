import { Check, Lightbulb, Link2, X } from "lucide-react";
import { Icon } from "@/components/ui/icon";

import { cn } from "@/lib/utils";
import { useInsights } from "@/modules/insights/hooks/use-insights";
import type { Insight } from "@/modules/insights/insight-types";

const STATUS_TABS = [
  { key: "pending", label: "Pending" },
  { key: "accepted", label: "Accepted" },
  { key: "dismissed", label: "Dismissed" },
];

function typeHue(type: string): string {
  switch (type) {
    case "discourse":
      return "text-amber";
    case "trend":
      return "text-echo";
    default:
      return "text-ink-3";
  }
}

function InsightCard({
  insight,
  pending,
  onAccept,
  onDismiss,
}: {
  insight: Insight;
  pending: boolean;
  onAccept: () => void;
  onDismiss: () => void;
}) {
  return (
    <article className="rounded-card border border-line bg-card p-4">
      <div className="mb-1.5 flex items-center gap-2">
        <span className={cn("font-mono text-meta uppercase tracking-[0.12em]", typeHue(insight.type))}>
          {insight.type}
        </span>
        {insight.trustTier ? (
          <span className="rounded-chip border border-line px-2 py-0.5 font-mono text-meta text-ink-3">
            {insight.trustTier}
          </span>
        ) : null}
        {insight.entity ? (
          <span className="text-small text-ink-3">· {insight.entity}</span>
        ) : null}
      </div>

      <h3 className="font-display text-lead leading-snug text-ink">{insight.title}</h3>
      {insight.body ? (
        <p className="mt-1.5 whitespace-pre-wrap text-body leading-relaxed text-ink-2">{insight.body}</p>
      ) : null}

      <div className="mt-3 flex items-center gap-3">
        <span className="flex items-center gap-1 font-mono text-meta text-ink-4">
          <Icon as={Link2} size="inline" mark />
          {insight.recordIds.length} {insight.recordIds.length === 1 ? "source" : "sources"}
        </span>
        {insight.confidence > 0 ? (
          <span className="font-mono text-meta text-ink-4">
            {Math.round(insight.confidence * 100)}% conf
          </span>
        ) : null}

        {pending ? (
          <div className="ml-auto flex items-center gap-1.5">
            <button
              type="button"
              onClick={onDismiss}
              className="flex items-center gap-1 rounded-chip border border-line px-2.5 py-1 text-small text-ink-3 transition-colors hover:bg-hover hover:text-ink"
            >
              <Icon as={X} size="inline" mark /> Dismiss
            </button>
            <button
              type="button"
              onClick={onAccept}
              className="flex items-center gap-1 rounded-chip border border-amber-line bg-amber-soft px-2.5 py-1 text-small text-amber transition-colors hover:bg-amber/10"
            >
              <Icon as={Check} size="inline" mark /> Accept
            </button>
          </div>
        ) : null}
      </div>
    </article>
  );
}

export function InsightsUI() {
  const { insights, loading, error, status, setStatus, accept, dismiss } = useInsights();
  const pending = status === "pending";

  return (
    <div className="flex min-w-0 flex-1 flex-col overflow-hidden bg-bg">
      <header className="flex items-center gap-3 border-b border-line bg-panel px-4 py-2.5">
        <Icon as={Lightbulb} mark className="text-amber" />
        <span className="text-body text-ink">Insights</span>
        <div className="ml-auto flex items-center gap-1">
          {STATUS_TABS.map((tab) => (
            <button
              key={tab.key}
              type="button"
              onClick={() => setStatus(tab.key)}
              className={cn(
                "rounded-chip px-2.5 py-1 text-small transition-colors",
                status === tab.key
                  ? "bg-sel text-ink"
                  : "text-ink-3 hover:bg-hover hover:text-ink",
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
            <p className="mt-10 text-center text-body text-ink-4">Loading…</p>
          ) : error ? (
            <p className="mt-10 text-center text-body text-against">{error}</p>
          ) : insights.length === 0 ? (
            <p className="mt-10 text-center text-body text-ink-4">
              {pending
                ? "No insights yet. They appear here as sources are ingested and analyzed."
                : `No ${status} insights.`}
            </p>
          ) : (
            <div className="flex flex-col gap-3">
              {insights.map((insight) => (
                <InsightCard
                  key={insight.id}
                  insight={insight}
                  pending={pending}
                  onAccept={() => accept(insight.id)}
                  onDismiss={() => dismiss(insight.id)}
                />
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
