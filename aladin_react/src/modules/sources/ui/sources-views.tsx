import type { ReactNode } from "react";
import { PlaceholderPane } from "@/components/ui/aladin";

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
