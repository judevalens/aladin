import type { Artifact } from "@/shared/api/models";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

export function LinkArtifactUI({ artifact }: { artifact: Artifact }) {
  return (
    <div className="mx-auto max-w-workspace-max px-8 py-8">
      <section className="space-y-4 border-b border-line pb-6">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <div className="eyebrow">Source</div>
            <h2 className="mt-1.5 font-display text-[1.5rem] font-semibold leading-tight tracking-[-0.02em] text-ink">
              {artifact.title}
            </h2>
          </div>
          <Badge>{artifact.kind}</Badge>
        </div>
        {artifact.sourceUrl ? (
          <a
            className="inline-flex items-center gap-1 text-[13px] text-amber underline-offset-4 hover:underline"
            href={artifact.sourceUrl}
            rel="noreferrer"
            target="_blank"
          >
            {artifact.sourceUrl}
          </a>
        ) : null}
        <p className="text-[13.5px] leading-[1.65] text-ink-2">
          {artifact.summary || "A saved external reference for this workspace."}
        </p>
      </section>

      <section className="space-y-2 pt-6">
        <div className="eyebrow">Excerpt</div>
        <p className="text-[13.5px] leading-[1.7] text-ink-2">
          {artifact.content || "Captured text will appear here when it is available."}
        </p>
      </section>
    </div>
  );
}

export function VoiceArtifactUI({ artifact }: { artifact: Artifact }) {
  return (
    <div className="mx-auto max-w-workspace-max px-8 py-8">
      <section className="space-y-4 border-b border-line pb-6">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <div className="eyebrow">Voice note</div>
            <h2 className="mt-1.5 font-display text-[1.5rem] font-semibold leading-tight tracking-[-0.02em] text-ink">
              {artifact.title}
            </h2>
          </div>
          <Badge>{artifact.kind}</Badge>
        </div>
        {artifact.resourceUrl ? <audio className="w-full" controls src={artifact.resourceUrl} /> : null}
        <p className="text-[13.5px] leading-[1.65] text-ink-2">
          {artifact.summary || "The original audio remains the source record for this note."}
        </p>
      </section>

      <section className="space-y-2 pt-6">
        <div className="eyebrow">Transcript</div>
        <p className="text-[13.5px] leading-[1.7] text-ink-3">
          Transcript will appear here when it is available.
        </p>
      </section>
    </div>
  );
}

export function FileArtifactUI({ artifact }: { artifact: Artifact }) {
  return (
    <div className="mx-auto max-w-workspace-max px-8 py-8">
      <section className="space-y-3">
        <div className="eyebrow">File</div>
        <h2 className="font-display text-[1.5rem] font-semibold leading-tight tracking-[-0.02em] text-ink">{artifact.title}</h2>
        <p className="text-[13.5px] leading-[1.65] text-ink-2">
          {artifact.summary || "A stored file resource in this workspace."}
        </p>
        {artifact.resourceUrl ? (
          <Button size="sm" onClick={() => window.open(artifact.resourceUrl ?? "", "_blank", "noopener,noreferrer")}>
            Open file
          </Button>
        ) : null}
      </section>
    </div>
  );
}
