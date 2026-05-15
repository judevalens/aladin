import { PlaceholderPane } from "@/components/ui/aladin";
import { SourcesScreen } from "@/features/sources/SourcesScreen";

export function SourcesRoute() {
  return (
    <>
      <section className="flex w-[352px] flex-col bg-white">
        <PlaceholderPane
          title="Sources"
          body="Connections, streams, and agent access for this workspace."
          className="h-full"
        />
      </section>
      <div className="w-px bg-gray-300" />
      <section className="min-w-0 flex-1 bg-white">
        <SourcesScreen />
      </section>
    </>
  );
}
