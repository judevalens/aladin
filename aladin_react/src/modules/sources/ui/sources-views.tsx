import type { ReactNode } from "react";
import { PlaceholderPane } from "@/components/ui/aladin";
import { AddSourceDialog } from "@/modules/sources/ui/add-source-dialog-view";
import { IntegrationsDialog } from "@/modules/sources/ui/integrations-dialog-view";
import { SourceDetailsDialog } from "@/modules/sources/ui/source-details-dialog-view";
import { SourcesOverviewSection } from "@/modules/sources/ui/sources-overview-section";
import type {
  AddSourceDialogProps,
  IntegrationsDialogProps,
  SourceDetailsDialogProps,
  SourcesFormatters,
  SourcesOverviewProps,
} from "@/modules/sources/ui/sources-view-types";

export function SourcesRouteView({ children }: { children: ReactNode }) {
  return (
    <>
      <section className="flex w-[352px] flex-col overflow-hidden border-r border-[#e7e5e4] bg-white">
        <PlaceholderPane
          title="Sources"
          body="Connections, streams, and agent access for this workspace."
          className="h-full"
        />
      </section>
      <section className="min-w-0 flex-1 overflow-hidden bg-white">
        {children}
      </section>
    </>
  );
}

export function SourcesScreenView({
  overview,
  addSourceDialog,
  integrationsDialog,
  sourceDetailsDialog,
  formatters,
}: {
  overview: SourcesOverviewProps;
  addSourceDialog: AddSourceDialogProps;
  integrationsDialog: IntegrationsDialogProps;
  sourceDetailsDialog: SourceDetailsDialogProps;
  formatters: SourcesFormatters;
}) {
  return (
    <div className="flex h-full flex-col bg-[#fafaf9]">
      <SourcesOverviewSection overview={overview} formatters={formatters} />
      <AddSourceDialog {...addSourceDialog} />
      <IntegrationsDialog {...integrationsDialog} />
      <SourceDetailsDialog {...sourceDetailsDialog} formatters={formatters} />
    </div>
  );
}
