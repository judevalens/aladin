import type { Artifact } from "@/shared/api/models";
import { AladinPanel } from "@/components/ui/aladin";
import { Button } from "@/components/ui/button";

export function FileArtifactPanel({ artifact }: { artifact: Artifact }) {
  return (
    <div className="space-y-4">
      <AladinPanel className="space-y-4 p-5">
        <div className="text-xs text-gray-500">
          File
        </div>
        <h2 className="text-2xl font-semibold text-black">{artifact.title}</h2>
        <p className="text-sm leading-6 text-gray-700">
          {artifact.summary || "A stored file resource in this workspace."}
        </p>
        {artifact.resourceUrl ? (
          <Button onClick={() => window.open(artifact.resourceUrl ?? "", "_blank", "noopener,noreferrer")}>
            Open file
          </Button>
        ) : null}
      </AladinPanel>
    </div>
  );
}
