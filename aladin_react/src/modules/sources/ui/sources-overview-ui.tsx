import { ArrowUpRight, LucideComponent, LucidePlus } from "lucide-react";
import { Eyebrow } from "@/components/ui/eyebrow";
import { Icon } from "@/components/ui/icon";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { MetricCard, Pill } from "@/modules/sources/ui/sources-parts-ui";
import type {
  SourcesFormatters,
  SourcesOverviewProps,
} from "@/modules/sources/ui/sources-ui-types";

export function SourcesOverviewSection({
  overview,
  formatters,
}: {
  overview: SourcesOverviewProps;
  formatters: SourcesFormatters;
}) {
  const {
    loading,
    metrics,
    sources,
    connectedCount,
    onOpenAddStream,
    onOpenIntegrations,
    onSelectSource,
  } = overview;

  return (
    <>
      <div className="border-b border-line bg-bg px-8 pt-8 pb-6">
        <div className="flex flex-wrap items-start justify-between gap-5">
          <div className="space-y-2">
            <Eyebrow>Sources</Eyebrow>
            <h1 className="max-w-2xl font-display text-title font-semibold leading-[1.15] tracking-[-0.02em] text-ink">
              Keep fresh material flowing in.
            </h1>
            <p className="max-w-2xl text-body leading-[1.6] text-ink-2">
              Manage live search streams, connected providers, and local agent access.
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="default" size="default" onClick={onOpenIntegrations}>
              <Icon as={LucideComponent} mark />
              Integrations ({connectedCount})
            </Button>
            <Button size="default" onClick={onOpenAddStream}>
              <Icon as={LucidePlus} mark /> <span> Add stream </span>
            </Button>
          </div>
        </div>
        <div className="mt-7 grid gap-px overflow-hidden rounded-control border border-line bg-line md:grid-cols-2 xl:grid-cols-4">
          {metrics.map((metric) => (
            <MetricCard
              key={metric.label}
              label={metric.label}
              value={metric.value}
              description={metric.description}
            />
          ))}
        </div>
      </div>

      <ScrollArea className="min-h-0 flex-1">
        {loading ? (
          <p className="px-8 py-6 text-body text-ink-3">Loading sources…</p>
        ) : sources.length === 0 ? (
          <div className="mx-8 my-6 flex flex-col items-start justify-between gap-5 rounded-control border border-dashed border-line bg-card p-6 md:flex-row md:items-center">
            <div className="space-y-1.5">
              <Eyebrow>No live streams yet</Eyebrow>
              <h2 className="font-display text-lead font-semibold tracking-[-0.01em] text-ink">
                Bring in the first stream.
              </h2>
              <p className="max-w-xl text-body leading-[1.6] text-ink-2">
                Add a source to start pulling new material into the workspace.
              </p>
            </div>
            <Button size="sm" onClick={onOpenAddStream}>
              Add stream
            </Button>
          </div>
        ) : (
          <div className="grid gap-3 p-6 md:grid-cols-2 xl:grid-cols-3">
            {sources.map((source) => (
              <button
                key={source.id}
                className="group min-h-[176px] rounded-card border border-line bg-card p-4 text-left transition-all hover:border-ink-4 hover:bg-raise"
                onClick={() => onSelectSource(source.id)}
                type="button"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0 space-y-2">
                    <Eyebrow className="text-ink-3">
                      {formatters.providerLabel(source)}
                    </Eyebrow>
                    <div className="space-y-1">
                      <h3 className="truncate text-lead font-semibold tracking-[-0.01em] text-ink">
                        {source.name}
                      </h3>
                      <p className="line-clamp-2 text-small leading-[1.5] text-ink-2">
                        {formatters.descriptionLine(source)}
                      </p>
                    </div>
                  </div>
                  <Badge>{formatters.healthLabel(source)}</Badge>
                </div>
                <div className="mt-4 flex flex-wrap gap-1.5">
                  <Pill>{formatters.lastRefreshSummary(source)}</Pill>
                </div>
                <div className="mt-4 inline-flex items-center gap-1 text-small font-medium text-ink-3 transition-colors group-hover:text-ink">
                  View details
                  <Icon as={ArrowUpRight} size="inline" mark />
                </div>
              </button>
            ))}
          </div>
        )}
      </ScrollArea>
    </>
  );
}
