import { PlaceholderPane } from "@/components/ui/aladin";
import { SourcesScreen } from "@/features/sources/SourcesScreen";

export function SourcesRoute() {
  return (
    <>
      <section className="flex w-[360px] flex-col bg-aladin-canvas">
        <PlaceholderPane
          title="Sources"
          body="Sources stay available while the shell is being refactored."
          className="h-full"
        />
      </section>
      <div className="w-px bg-aladin-divider" />
      <section className="min-w-0 flex-1 bg-aladin-canvas">
        <SourcesScreen />
      </section>
    </>
  );
}
