import { useEffect, useState } from "react";
import { useAppComposition } from "@/app/composition/app-composition";
import type { Artifact } from "@/shared/api/models";

export interface ArtifactResourceState {
  /** Object URL for the fetched blob, or null while loading / on error / when N/A. */
  url: string | null;
  /** The blob's MIME type (from the server content-type), for viewer branching. */
  contentType: string | null;
  loading: boolean;
  error: string | null;
}

/**
 * Loads an artifact's binary resource (audio/file) through the authenticated API
 * client and exposes it as an object URL. Native <audio>/<img> elements can't send
 * the Bearer token this app uses, so pointing them straight at /api/.../resource
 * yields an auth error — we fetch here and hand over an object URL instead.
 */
export function useArtifactResource(artifact: Artifact): ArtifactResourceState {
  const { repos } = useAppComposition();
  const [url, setUrl] = useState<string | null>(null);
  const [contentType, setContentType] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const needsResource =
    (artifact.kind === "voice" || artifact.kind === "file") && Boolean(artifact.resourceUrl);

  useEffect(() => {
    if (!needsResource) {
      setUrl(null);
      setContentType(null);
      setError(null);
      setLoading(false);
      return;
    }

    let objectUrl: string | null = null;
    let cancelled = false;
    setLoading(true);
    setError(null);

    repos.artifacts
      .getResourceBlob(artifact.id)
      .then((blob) => {
        if (cancelled) return;
        objectUrl = URL.createObjectURL(blob);
        setUrl(objectUrl);
        setContentType(blob.type || null);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : "Couldn't load this resource.");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [artifact.id, needsResource, repos.artifacts]);

  return { url, contentType, loading, error };
}
