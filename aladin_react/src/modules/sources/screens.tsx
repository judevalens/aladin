import { useState } from "react";
import { useSourceDetailsDialogState } from "@/modules/sources/hooks/use-source-details-dialog-state";
import { useSourcesScreen } from "@/modules/sources/hooks/use-sources-screen";
import { AddSourceDialog } from "@/modules/sources/ui/add-source-dialog-view";
import { IntegrationsDialog } from "@/modules/sources/ui/integrations-dialog-view";
import { SourceDetailsDialog } from "@/modules/sources/ui/source-details-dialog-view";
import { SourcesOverviewSection } from "@/modules/sources/ui/sources-overview-section";
import { SourcesRouteView } from "@/modules/sources/ui/sources-views";

export function SourcesRouteScreen() {
  const screen = useSourcesScreen();
  const [addSourceOpen, setAddSourceOpen] = useState(false);
  const [integrationsOpen, setIntegrationsOpen] = useState(false);
  const sourceDetailsDialog = useSourceDetailsDialogState({
    sources: screen.catalog.sources,
    removeSource: screen.sourceActions.removeSource,
  });

  return (
    <SourcesRouteView>
      <div className="flex h-full flex-col bg-[#fafaf9]">
        <SourcesOverviewSection
          overview={{
            loading: screen.catalog.loading,
            metrics: screen.catalog.metrics,
            sources: screen.catalog.sources,
            connectedCount: screen.catalog.connectedCount,
            onOpenAddStream: () => setAddSourceOpen(true),
            onOpenIntegrations: () => setIntegrationsOpen(true),
            onSelectSource: sourceDetailsDialog.onSelectSource,
          }}
          formatters={screen.formatters}
        />
        <AddSourceDialog
          open={addSourceOpen}
          onOpenChange={setAddSourceOpen}
          createSource={screen.sourceActions.createSource}
        />
        <IntegrationsDialog open={integrationsOpen} onOpenChange={setIntegrationsOpen} />
        <SourceDetailsDialog
          selectedSource={sourceDetailsDialog.selectedSource}
          removeSourcePending={sourceDetailsDialog.removeSourcePending}
          onSelectSource={sourceDetailsDialog.onSelectSource}
          onRemoveSelectedSource={sourceDetailsDialog.onRemoveSelectedSource}
          formatters={screen.formatters}
        />
      </div>
    </SourcesRouteView>
  );
}
