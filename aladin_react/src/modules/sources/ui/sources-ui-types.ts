import type { Source } from "@/shared/api/models";

export type SourcesMetric = {
  label: string;
  value: string;
  description: string;
};

export type SourceFact = {
  label: string;
  value: string;
};

export type SourcesFormatters = {
  providerLabel: (source: Source) => string;
  descriptionLine: (source: Source) => string;
  healthLabel: (source: Source) => string;
  lastRefreshSummary: (source: Source) => string;
  formatSourceFacts: (source: Source) => SourceFact[];
};

export type SourcesOverviewProps = {
  loading: boolean;
  metrics: SourcesMetric[];
  sources: Source[];
  connectedCount: number;
  onOpenAddStream: () => void;
  onOpenIntegrations: () => void;
  onSelectSource: (sourceId: string | null) => void;
};

export type AddSourceDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  createSource: (input: {
    query: string;
    title: string;
    limit: string;
  }) => Promise<void>;
};

export type IntegrationsDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

export type SourceDetailsDialogProps = {
  selectedSource: Source | null;
  removeSourcePending: boolean;
  onSelectSource: (sourceId: string | null) => void;
  onRemoveSelectedSource: () => void;
};
