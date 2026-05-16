import type { Artifact } from "@/shared/api/models";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

export function LinkArtifactView({ artifact }: { artifact: Artifact }) {
  return (
    <div className="space-y-0 px-7 py-6">
      <section className="space-y-4 border-b border-[#e4e4e4] pb-6">
        <div className="flex items-center justify-between gap-4">
          <div>
            <div className="text-xs text-gray-500">Source</div>
            <h2 className="mt-2 text-2xl font-semibold text-black">{artifact.title}</h2>
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
      </section>

      <section className="space-y-3 pt-6">
        <div className="text-xs text-gray-500">Raw excerpt</div>
        <p className="text-sm leading-7 text-gray-700">
          {artifact.content || "Captured text will appear here when it is available."}
        </p>
      </section>
    </div>
  );
}

export function VoiceArtifactView({ artifact }: { artifact: Artifact }) {
  return (
    <div className="space-y-0 px-7 py-6">
      <section className="space-y-4 border-b border-[#e4e4e4] pb-6">
        <div className="flex items-center justify-between gap-4">
          <div>
            <div className="text-xs text-gray-500">Voice note</div>
            <h2 className="mt-2 text-2xl font-semibold text-black">{artifact.title}</h2>
          </div>
          <Badge>{artifact.kind}</Badge>
        </div>
        {artifact.resourceUrl ? <audio className="w-full" controls src={artifact.resourceUrl} /> : null}
        <p className="text-sm leading-6 text-gray-700">
          {artifact.summary || "The original audio remains the source record for this note."}
        </p>
      </section>

      <section className="space-y-3 pt-6">
        <div className="text-xs text-gray-500">Transcript</div>
        <p className="text-sm leading-7 text-gray-700">
          Transcript will appear here when it is available.
        </p>
      </section>
    </div>
  );
}

export function FileArtifactView({ artifact }: { artifact: Artifact }) {
  return (
    <div className="px-7 py-6">
      <section className="space-y-4">
        <div className="text-xs text-gray-500">File</div>
        <h2 className="text-2xl font-semibold text-black">{artifact.title}</h2>
        <p className="text-sm leading-6 text-gray-700">
          {artifact.summary || "A stored file resource in this workspace."}
        </p>
        {artifact.resourceUrl ? (
          <Button onClick={() => window.open(artifact.resourceUrl ?? "", "_blank", "noopener,noreferrer")}>
            Open file
          </Button>
        ) : null}
      </section>
    </div>
  );
}
