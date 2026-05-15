import { PlaceholderPane } from "@/components/ui/aladin";

interface PlaceholderDestinationRouteProps {
  paneTitle: string;
  paneBody: string;
  workTitle: string;
  workBody: string;
}

export function PlaceholderDestinationRoute({
  paneTitle,
  paneBody,
  workTitle,
  workBody,
}: PlaceholderDestinationRouteProps) {
  return (
    <>
      <section className="flex w-[352px] flex-col bg-white">
        <PlaceholderPane title={paneTitle} body={paneBody} className="h-full" />
      </section>
      <div className="w-px bg-gray-300" />
      <section className="flex min-w-0 flex-1 bg-white">
        <PlaceholderPane title={workTitle} body={workBody} className="h-full w-full" />
      </section>
    </>
  );
}
