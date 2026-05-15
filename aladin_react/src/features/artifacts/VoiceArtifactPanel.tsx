import type { Artifact } from "@/shared/api/models";
import { AladinPanel } from "@/components/ui/aladin";
import { Badge } from "@/components/ui/badge";

export function VoiceArtifactPanel({ artifact }: { artifact: Artifact }) {
  return (
    <div className="space-y-4">
      <AladinPanel className="space-y-4 p-5">
        <div className="flex items-center justify-between gap-4">
          <div>
            <div className="text-xs text-gray-500">
              Voice note
            </div>
            <h2 className="mt-2 text-2xl font-semibold text-black">
              {artifact.title}
            </h2>
          </div>
          <Badge>{artifact.kind}</Badge>
        </div>
        {artifact.resourceUrl ? <audio className="w-full" controls src={artifact.resourceUrl} /> : null}
        <p className="text-sm leading-6 text-gray-700">
          {artifact.summary || "The original audio remains the source record for this note."}
        </p>
      </AladinPanel>

      <AladinPanel className="space-y-3 p-5">
        <div className="text-xs text-gray-500">
          Transcript
        </div>
        <p className="text-sm leading-7 text-gray-700">
          Transcript will appear here when it is available.
        </p>
      </AladinPanel>
    </div>
  );
}
