import type { Artifact } from "@/shared/api/models";
import { AladinPanel } from "@/components/ui/aladin";
import { Badge } from "@/components/ui/badge";

export function VoiceArtifactPanel({ artifact }: { artifact: Artifact }) {
  return (
    <div className="space-y-4">
      <AladinPanel className="space-y-4 p-5">
        <div className="flex items-center justify-between gap-4">
          <div>
            <div className="text-xs font-semibold uppercase tracking-[0.2em] text-aladin-code-text">
              Voice artifact
            </div>
            <h2 className="mt-2 text-3xl font-semibold tracking-[-0.04em] text-aladin-ink">
              {artifact.title}
            </h2>
          </div>
          <Badge>{artifact.kind}</Badge>
        </div>
        {artifact.resourceUrl ? <audio className="w-full" controls src={artifact.resourceUrl} /> : null}
        <p className="text-sm leading-6 text-aladin-ink-secondary">
          {artifact.summary ||
            "Transcript and derived context will live here once voice enrichment lands. The audio remains the source of truth."}
        </p>
      </AladinPanel>

      <AladinPanel className="space-y-3 p-5">
        <div className="text-xs font-semibold uppercase tracking-[0.2em] text-aladin-code-text">
          Transcript
        </div>
        <p className="text-sm leading-7 text-aladin-ink-secondary">
          Transcript will appear here after voice transcription is wired. This stub keeps the consumption layout aligned with the current Compose client.
        </p>
      </AladinPanel>
    </div>
  );
}
