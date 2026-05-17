import { useEffect, useMemo } from "react";
import { useAppComposition } from "@/app/composition/app-composition";
import type { IntegrationToken, ProviderConnectionProvider, Source } from "@/shared/api/models";
import { useObservableState } from "@/shared/flow/use-observable-state";
import type { SourcesFormatters, SourcesMetric } from "@/modules/sources/ui/sources-view-types";

export interface SourcesScreenState {
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

export function useSourcesScreen(): SourcesScreenState {
  const { services } = useAppComposition();

  const sourcesLoadable = useObservableState(
    () => services.sources.catalog.observeSources(),
    () => services.sources.catalog.getSourcesSnapshot(),
    [services.sources.catalog],
  );
  const providersLoadable = useObservableState(
    () => services.sources.catalog.observeProviders(),
    () => services.sources.catalog.getProvidersSnapshot(),
    [services.sources.catalog],
  );
  const tokensLoadable = useObservableState(
    () => services.sources.catalog.observeTokens(),
    () => services.sources.catalog.getTokensSnapshot(),
    [services.sources.catalog],
  );

  useEffect(() => {
    void services.sources.catalog.ensureSourcesLoaded();
    void services.sources.catalog.ensureProvidersLoaded();
  }, [services.sources.catalog]);

  const sources = sourcesLoadable.data;
  const providers = providersLoadable.data;
  const tokens = tokensLoadable.data;
  const connectedCount = providers.filter((provider) => provider.connected).length;
  const activeCount = sources.filter((source) => services.sources.isOperational(source.syncState)).length;
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
        sourcesLoadable.status === "loading" ||
        sourcesLoadable.status === "idle" ||
        providersLoadable.status === "loading" ||
        providersLoadable.status === "idle",
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
        await services.sources.catalog.createSource({
          kind: "bluesky.search",
          name: title.trim() || undefined,
          query: query.trim(),
          limit: Number.parseInt(limit, 10) || 25,
        });
      },
      removeSource: async (sourceId: string) => {
        await services.sources.catalog.deleteSource(sourceId);
      },
    },
    formatters: {
      providerLabel: services.sources.providerLabel,
      descriptionLine: services.sources.descriptionLine,
      healthLabel: services.sources.healthLabel,
      lastRefreshSummary: services.sources.lastRefreshSummary,
      formatSourceFacts: services.sources.formatSourceFacts,
    },
  };
}
