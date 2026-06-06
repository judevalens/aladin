import { useState } from "react";
import { PlaceholderPane } from "@/components/ui/aladin";
import { useSourceDetailsDialogState } from "@/modules/sources/hooks/use-source-details-dialog-state";
import { useSourcesState } from "@/modules/sources/hooks/use-sources-state";
import { AddSourceDialog } from "@/modules/sources/ui/add-source-dialog-ui";
import { IntegrationsDialog } from "@/modules/sources/ui/integrations-dialog-ui";
import { SourceDetailsDialog } from "@/modules/sources/ui/source-details-dialog-ui";
import { SourcesOverviewSection } from "@/modules/sources/ui/sources-overview-ui";

export function SourcesRoute() {
  const sources = useSourcesState();
  const [addSourceOpen, setAddSourceOpen] = useState(false);
  const [integrationsOpen, setIntegrationsOpen] = useState(false);
  const sourceDetailsDialog = useSourceDetailsDialogState({
    sources: sources.catalog.sources,
    removeSource: sources.sourceActions.removeSource,
  });

  return (
    <>
      <section className="flex w-[352px] flex-col overflow-hidden border-r border-line bg-explorer">
        <PlaceholderPane
          title="Sources"
          body="Connections, streams, and agent access for this workspace."
          className="h-full"
        />
      </section>
      <section className="min-w-0 flex-1 overflow-hidden bg-bg">
        <div className="flex h-full flex-col bg-bg">
          <SourcesOverviewSection
            overview={{
              loading: sources.catalog.loading,
              metrics: sources.catalog.metrics,
              sources: sources.catalog.sources,
              connectedCount: sources.catalog.connectedCount,
              onOpenAddStream: () => setAddSourceOpen(true),
              onOpenIntegrations: () => setIntegrationsOpen(true),
              onSelectSource: sourceDetailsDialog.onSelectSource,
            }}
            formatters={sources.formatters}
          />
          <AddSourceDialog
            open={addSourceOpen}
            onOpenChange={setAddSourceOpen}
            createSource={sources.sourceActions.createSource}
          />
          <IntegrationsDialog open={integrationsOpen} onOpenChange={setIntegrationsOpen} />
          <SourceDetailsDialog
            selectedSource={sourceDetailsDialog.selectedSource}
            removeSourcePending={sourceDetailsDialog.removeSourcePending}
            onSelectSource={sourceDetailsDialog.onSelectSource}
            onRemoveSelectedSource={sourceDetailsDialog.onRemoveSelectedSource}
            formatters={sources.formatters}
          />
        </div>
      </section>
    </>
  );
}
