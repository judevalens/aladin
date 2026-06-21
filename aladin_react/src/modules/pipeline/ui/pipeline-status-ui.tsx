import { Activity, AlertTriangle, CheckCircle2, Clock, Loader2, XCircle } from "lucide-react";

import { cn } from "@/lib/utils";
import { usePipelineStats } from "@/modules/pipeline/hooks/use-pipeline-stats";
import type { PipelineStats } from "@/modules/pipeline/pipeline-types";

function fmtDuration(secs: number | null): string {
  if (secs == null) return "—";
  if (secs < 90) return `${Math.round(secs)}s`;
  if (secs < 5400) return `${Math.round(secs / 60)}m`;
  return `${(secs / 3600).toFixed(1)}h`;
}

function StatCard({
  label,
  value,
  hint,
  tone = "ink",
  icon,
}: {
  label: string;
  value: string | number;
  hint?: string;
  tone?: "ink" | "amber" | "against" | "for";
  icon: React.ReactNode;
}) {
  const valueTone =
    tone === "amber"
      ? "text-amber"
      : tone === "against"
        ? "text-against"
        : tone === "for"
          ? "text-for"
          : "text-ink";
  return (
    <div className="rounded-card border border-line bg-card p-3.5">
      <div className="flex items-center gap-1.5 text-ink-4">
        {icon}
        <span className="font-mono text-[10px] uppercase tracking-[0.12em]">{label}</span>
      </div>
      <div className={cn("mt-1.5 font-display text-2xl", valueTone)}>{value}</div>
      {hint ? <div className="mt-0.5 text-[11px] text-ink-4">{hint}</div> : null}
    </div>
  );
}

// The ordered pipeline stages, so the breakdown reads left-to-right as a record flows.
const STAGE_ORDER = ["pending", "captured", "enriched", "complete", "failed"];

function StageBreakdown({ byStatus }: { byStatus: Record<string, number> }) {
  const keys = [
    ...STAGE_ORDER.filter((k) => k in byStatus),
    ...Object.keys(byStatus).filter((k) => !STAGE_ORDER.includes(k)),
  ];
  if (keys.length === 0) {
    return <p className="text-[13px] text-ink-4">No records ingested yet.</p>;
  }
  return (
    <ul className="flex flex-wrap gap-2">
      {keys.map((k) => (
        <li
          key={k}
          className="flex items-center gap-2 rounded-chip border border-line bg-raise px-2.5 py-1"
        >
          <span
            className={cn(
              "font-mono text-[10px] uppercase",
              k === "complete" ? "text-for" : k === "failed" ? "text-against" : "text-ink-3",
            )}
          >
            {k}
          </span>
          <span className="text-[13px] text-ink">{byStatus[k]}</span>
        </li>
      ))}
    </ul>
  );
}

function Body({ stats }: { stats: PipelineStats }) {
  const { records, throughput } = stats;
  return (
    <div className="mx-auto flex w-full max-w-[820px] flex-col gap-6">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
        <StatCard
          label="In flight"
          value={records.inFlight}
          hint={`oldest ${fmtDuration(records.oldestInFlightSecs)}`}
          icon={<Loader2 className="size-3.5" />}
        />
        <StatCard
          label="Stuck"
          value={records.stuckOver15Min}
          hint="no progress 15m+"
          tone={records.stuckOver15Min > 0 ? "amber" : "ink"}
          icon={<AlertTriangle className="size-3.5" />}
        />
        <StatCard
          label="Failed"
          value={records.failed}
          tone={records.failed > 0 ? "against" : "ink"}
          icon={<XCircle className="size-3.5" />}
        />
        <StatCard
          label="Done / hr"
          value={throughput.completedLast1h}
          hint={`${throughput.completedLast24h} in 24h`}
          tone="for"
          icon={<CheckCircle2 className="size-3.5" />}
        />
        <StatCard
          label="Avg latency"
          value={fmtDuration(throughput.avgLatencySecs)}
          hint="capture → complete"
          icon={<Clock className="size-3.5" />}
        />
        <StatCard
          label="Relationships"
          value={stats.relationships}
          icon={<Activity className="size-3.5" />}
        />
      </div>

      <section>
        <h3 className="mb-2 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-4">
          Records by stage
        </h3>
        <StageBreakdown byStatus={records.byStatus} />
      </section>

      <section>
        <h3 className="mb-2 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-4">
          Insights · {Object.values(stats.insights.byType).reduce((a, b) => a + b, 0)}
        </h3>
        <StageBreakdown byStatus={stats.insights.byType} />
      </section>
    </div>
  );
}

export function PipelineStatusUI() {
  const { stats, loading, error } = usePipelineStats();

  return (
    <div className="flex min-w-0 flex-1 flex-col overflow-hidden bg-bg">
      <header className="flex items-center gap-2 border-b border-line bg-panel px-4 py-2.5">
        <Activity className="size-4 text-amber" />
        <span className="font-display text-[13px] text-ink">Ingestion pipeline</span>
        <span className="ml-auto font-mono text-[11px] text-ink-4">live · 5s</span>
      </header>
      <div className="min-h-0 flex-1 overflow-auto px-6 py-6">
        {loading && !stats ? (
          <p className="mt-10 text-center text-[13px] text-ink-4">Loading…</p>
        ) : error && !stats ? (
          <p className="mt-10 text-center text-[13px] text-against">{error}</p>
        ) : stats ? (
          <Body stats={stats} />
        ) : null}
      </div>
    </div>
  );
}
