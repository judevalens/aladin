import { useMemo } from "react";
import { useAppComposition } from "@/app/composition/app-composition";
import {
  descriptionLine,
  formatSourceFacts,
  healthLabel,
  isOperational,
  lastRefreshSummary,
  providerLabel,
} from "@/services/sources/sources-service";
import type { IntegrationToken, ProviderConnectionProvider, Source } from "@/shared/api/models";
import { useObservableState } from "@/shared/flow/use-observable-state";
import type { SourcesFormatters, SourcesMetric } from "@/modules/sources/ui/sources-ui-types";

export interface SourcesState {
  catalog: {
    loading: boolean;
    sources: Source[];
    providers: ProviderConnectionProvider[];
    tokens: IntegrationToken[];
    connectedCount: number;
    metrics: SourcesMetric[];
  };
  sourceActions: {
    createSource: (input: {
      query: string;
      title: string;
      limit: string;
    }) => Promise<void>;
    removeSource: (sourceId: string) => Promise<void>;
  };
  formatters: SourcesFormatters;
}

export function useSourcesState(): SourcesState {
  const { services } = useAppComposition();

  const sourcesLoadable = useObservableState(services.sources.sources());
  const providersLoadable = useObservableState(services.sources.providers());
  const tokensLoadable = useObservableState(services.sources.tokens());

  const sources = sourcesLoadable.status === "data" ? sourcesLoadable.value : [];
  const providers =
    providersLoadable.status === "data" ? providersLoadable.value : [];
  const tokens = tokensLoadable.status === "data" ? tokensLoadable.value : [];
  const connectedCount = providers.filter((provider) => provider.connected).length;
  const activeCount = sources.filter((source) => isOperational(source.syncState)).length;
  const pendingCount = sources.filter((source) => !source.lastSyncedAt).length;
  const providerCount = new Set(sources.map((source) => source.type)).size;

  const metrics = useMemo(
    (): SourcesMetric[] => [
      {
        label: "Subscribed",
        value: String(sources.length),
        description: "sources currently feeding this workspace",
      },
      {
        label: "Healthy",
        value: String(activeCount),
        description: "sources reporting a live or active state",
      },
      {
        label: "Pending",
        value: String(pendingCount),
        description: "sources still waiting on a first refresh",
      },
      {
        label: "Providers",
        value: String(providerCount),
        description: "upstream systems represented here",
      },
    ],
    [activeCount, pendingCount, providerCount, sources.length],
  );

  return {
    catalog: {
      loading:
        sourcesLoadable.status !== "data" || providersLoadable.status !== "data",
      sources,
      providers,
      tokens,
      connectedCount,
      metrics,
    },
    sourceActions: {
      createSource: async ({
        query,
        title,
        limit,
      }: {
        query: string;
        title: string;
        limit: string;
      }) => {
        await services.sources.createSource({
          kind: "bluesky_search",
          name: title.trim() || undefined,
          query: query.trim(),
          limit: Number.parseInt(limit, 10) || 25,
        });
      },
      removeSource: async (sourceId: string) => {
        await services.sources.deleteSource(sourceId);
      },
    },
    formatters: {
      providerLabel,
      descriptionLine,
      healthLabel,
      lastRefreshSummary,
      formatSourceFacts,
    },
  };
}
