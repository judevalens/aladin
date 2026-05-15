import type { Artifact } from "@/shared/api/models";
import { AladinPanel } from "@/components/ui/aladin";
import { Badge } from "@/components/ui/badge";

export function LinkArtifactPanel({ artifact }: { artifact: Artifact }) {
  return (
    <div className="space-y-4">
      <AladinPanel className="space-y-4 p-5">
        <div className="flex items-center justify-between gap-4">
          <div>
            <div className="text-xs text-gray-500">
              Source
            </div>
            <h2 className="mt-2 text-2xl font-semibold text-black">
              {artifact.title}
            </h2>
          </div>
          <Badge>{artifact.kind}</Badge>
        </div>
        {artifact.sourceUrl ? (
          <a
            className="inline-flex items-center gap-2 text-sm text-gray-700 underline underline-offset-4"
            href={artifact.sourceUrl}
            rel="noreferrer"
            target="_blank"
          >
            {artifact.sourceUrl}
          </a>
        ) : null}
        <p className="text-sm leading-6 text-gray-700">
          {artifact.summary || "A saved external reference for this workspace."}
        </p>
      </AladinPanel>

      <AladinPanel className="space-y-3 p-5">
        <div className="text-xs text-gray-500">
          Raw excerpt
        </div>
        <p className="text-sm leading-7 text-gray-700">
          {artifact.content || "Captured text will appear here when it is available."}
        </p>
      </AladinPanel>
    </div>
  );
}
