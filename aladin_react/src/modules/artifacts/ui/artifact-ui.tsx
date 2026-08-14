import { ExternalLink } from "lucide-react";
import { Eyebrow } from "@/components/ui/eyebrow";
import { Icon } from "@/components/ui/icon";
import type { Artifact } from "@/shared/api/models";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useArtifactResource } from "@/modules/artifacts/hooks/use-artifact-resource";

/** The host of a link, for a compact source chip. */
function linkHost(sourceUrl?: string | null): string | null {
  if (!sourceUrl) return null;
  try {
    return new URL(sourceUrl).host;
  } catch {
    return null;
  }
}

type FileCategory = "image" | "pdf" | "video" | "audio" | "other";

/** Coarse file category from a MIME type, to choose an inline viewer. */
function fileCategory(contentType: string | null): FileCategory {
  const type = (contentType ?? "").toLowerCase();
  if (type.startsWith("image/")) return "image";
  if (type === "application/pdf") return "pdf";
  if (type.startsWith("video/")) return "video";
  if (type.startsWith("audio/")) return "audio";
  return "other";
}

export function LinkArtifactUI({ artifact }: { artifact: Artifact }) {
  const host = linkHost(artifact.sourceUrl);
  return (
    <div className="mx-auto max-w-workspace-max px-8 py-8">
      <section className="space-y-4 border-b border-line pb-6">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0 space-y-1.5">
            <Eyebrow>Source{host ? ` · ${host}` : ""}</Eyebrow>
            <h2 className="font-display text-title font-semibold leading-tight tracking-[-0.02em] text-ink">
              {artifact.title}
            </h2>
          </div>
          <Badge>{artifact.kind}</Badge>
        </div>
        {artifact.sourceUrl ? (
          <div className="flex flex-wrap items-center gap-3">
            <Button
              size="sm"
              onClick={() => window.open(artifact.sourceUrl ?? "", "_blank", "noopener,noreferrer")}
            >
              <Icon as={ExternalLink} />
              Open link
            </Button>
            <a
              className="truncate text-body text-amber underline-offset-4 hover:underline"
              href={artifact.sourceUrl}
              rel="noreferrer"
              target="_blank"
            >
              {artifact.sourceUrl}
            </a>
          </div>
        ) : null}
        <p className="text-body leading-[1.65] text-ink-2">
          {artifact.summary || "A saved external reference for this workspace."}
        </p>
      </section>

      <section className="space-y-2 pt-6">
        <Eyebrow>Excerpt</Eyebrow>
        <p className="text-body leading-[1.7] text-ink-2">
          {artifact.content || "Captured text will appear here when it is available."}
        </p>
      </section>
    </div>
  );
}

export function VoiceArtifactUI({ artifact }: { artifact: Artifact }) {
  const { url, loading, error } = useArtifactResource(artifact);
  return (
    <div className="mx-auto max-w-workspace-max px-8 py-8">
      <section className="space-y-4 border-b border-line pb-6">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <Eyebrow>Voice note</Eyebrow>
            <h2 className="mt-1.5 font-display text-title font-semibold leading-tight tracking-[-0.02em] text-ink">
              {artifact.title}
            </h2>
          </div>
          <Badge>{artifact.kind}</Badge>
        </div>
        {loading ? (
          <p className="text-body text-ink-3">Loading audio…</p>
        ) : error ? (
          <p className="text-body text-against">Couldn’t load the audio. {error}</p>
        ) : url ? (
          <audio className="w-full" controls src={url} />
        ) : null}
        <p className="text-body leading-[1.65] text-ink-2">
          {artifact.summary || "The original audio remains the source record for this note."}
        </p>
      </section>

      <section className="space-y-2 pt-6">
        <Eyebrow>Transcript</Eyebrow>
        <p className="text-body leading-[1.7] text-ink-3">
          Transcript will appear here when it is available.
        </p>
      </section>
    </div>
  );
}

export function FileArtifactUI({ artifact }: { artifact: Artifact }) {
  const { url, contentType, loading, error } = useArtifactResource(artifact);
  const category = fileCategory(contentType);
  return (
    <div className="mx-auto max-w-workspace-max px-8 py-8">
      <section className="space-y-4">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <Eyebrow>File</Eyebrow>
            <h2 className="mt-1.5 font-display text-title font-semibold leading-tight tracking-[-0.02em] text-ink">
              {artifact.title}
            </h2>
          </div>
          {contentType ? <Badge>{contentType}</Badge> : null}
        </div>
        {artifact.summary ? (
          <p className="text-body leading-[1.65] text-ink-2">{artifact.summary}</p>
        ) : null}

        {loading ? (
          <p className="text-body text-ink-3">Loading file…</p>
        ) : error ? (
          <p className="text-body text-against">Couldn’t load the file. {error}</p>
        ) : !url ? null : category === "image" ? (
          <img
            src={url}
            alt={artifact.title}
            className="max-h-[70vh] max-w-full rounded-control border border-line object-contain"
          />
        ) : category === "pdf" ? (
          <iframe src={url} title={artifact.title} className="h-[75vh] w-full rounded-control border border-line" />
        ) : category === "video" ? (
          <video src={url} controls className="max-h-[70vh] w-full rounded-control border border-line" />
        ) : category === "audio" ? (
          <audio src={url} controls className="w-full" />
        ) : (
          <div className="space-y-3">
            <p className="text-body text-ink-3">No inline preview for this file type.</p>
            <Button size="sm" onClick={() => window.open(url, "_blank", "noopener,noreferrer")}>
              <Icon as={ExternalLink} />
              Open file
            </Button>
          </div>
        )}
      </section>
    </div>
  );
}
