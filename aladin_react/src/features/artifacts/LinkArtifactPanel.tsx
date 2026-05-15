import { ExternalLink } from "lucide-react";
import type { Artifact } from "@/shared/api/models";
import { AladinPanel } from "@/components/ui/aladin";
import { Badge } from "@/components/ui/badge";

export function LinkArtifactPanel({ artifact }: { artifact: Artifact }) {
  return (
    <div className="space-y-4">
      <AladinPanel className="space-y-4 p-5">
        <div className="flex items-center justify-between gap-4">
          <div>
            <div className="text-xs font-semibold uppercase tracking-[0.2em] text-aladin-code-text">
              Source
            </div>
            <h2 className="mt-2 text-3xl font-semibold tracking-[-0.04em] text-aladin-ink">
              {artifact.title}
            </h2>
          </div>
          <Badge>{artifact.kind}</Badge>
        </div>
        {artifact.sourceUrl ? (
          <a
            className="inline-flex items-center gap-2 text-sm text-aladin-ink-secondary underline underline-offset-4"
            href={artifact.sourceUrl}
            rel="noreferrer"
            target="_blank"
          >
            <ExternalLink className="h-4 w-4" />
            {artifact.sourceUrl}
          </a>
        ) : null}
        <p className="text-sm leading-6 text-aladin-ink-secondary">
          {artifact.summary ||
            "This saved source appears relevant to the workspace because it adds external context, evidence, or a reusable reference point."}
        </p>
      </AladinPanel>

      <AladinPanel className="space-y-3 p-5">
        <div className="text-xs font-semibold uppercase tracking-[0.2em] text-aladin-code-text">
          Raw excerpt
        </div>
        <p className="text-sm leading-7 text-aladin-ink-secondary">
          {artifact.content ||
            "Raw captured source text will appear here once link ingestion is wired. For now this keeps the consumption layout realistic without backend enrichment."}
        </p>
      </AladinPanel>
    </div>
  );
}
