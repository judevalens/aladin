import { Download } from "lucide-react";
import type { Artifact } from "@/shared/api/models";
import { AladinPanel } from "@/components/ui/aladin";
import { Button } from "@/components/ui/button";

export function FileArtifactPanel({ artifact }: { artifact: Artifact }) {
  return (
    <div className="space-y-4">
      <AladinPanel className="space-y-4 p-5">
        <div className="text-xs font-semibold uppercase tracking-[0.2em] text-aladin-code-text">
          File artifact
        </div>
        <h2 className="text-3xl font-semibold tracking-[-0.04em] text-aladin-ink">{artifact.title}</h2>
        <p className="text-sm leading-6 text-aladin-ink-secondary">
          {artifact.summary ||
            "File enrichment will expose metadata, preview text, and related workspace context here."}
        </p>
        {artifact.resourceUrl ? (
          <Button onClick={() => window.open(artifact.resourceUrl ?? "", "_blank", "noopener,noreferrer")}>
            <Download className="h-4 w-4" />
            Open file resource
          </Button>
        ) : null}
      </AladinPanel>
    </div>
  );
}
