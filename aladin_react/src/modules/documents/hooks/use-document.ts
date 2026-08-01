import { useCallback, useEffect, useState } from "react";
import { debounceTime, filter } from "rxjs";

import { useAppComposition } from "@/app/composition/app-composition";
import type { IngestedDocument } from "@/repos/documents/document-repo";

export interface UseDocument {
  document: IngestedDocument | null;
  loading: boolean;
  error: string | null;
}

/**
 * Loads an artifact's ingested document (design/INGESTION_PRD.md).
 *
 * Ingestion happens in a background worker, so this surface has to learn about a status
 * change it didn't cause. It rides the syncer for that rather than polling: the worker
 * emits the artifact's node frame when it writes the result, and we refetch off that
 * event — the same shape use-graph-pane and use-research-overview use.
 *
 * `withText` is opt-in so a status chip doesn't pull a book's worth of text.
 */
export function useDocument(artifactId: string, withText: boolean): UseDocument {
  const { repos, runtime } = useAppComposition();
  const [document, setDocument] = useState<IngestedDocument | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);

  const reload = useCallback(() => setNonce((n) => n + 1), []);

  useEffect(() => {
    if (!artifactId) return;
    const sub = runtime.dataEvents
      .events()
      .pipe(
        filter((event) => event.type === "nodeUpserted" && event.payload.id === artifactId),
        debounceTime(250),
      )
      .subscribe(() => reload());
    return () => sub.unsubscribe();
  }, [runtime, artifactId, reload]);

  useEffect(() => {
    if (!artifactId) {
      setDocument(null);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    repos.documents
      .get(artifactId, withText)
      .then((result) => {
        if (cancelled) return;
        setDocument(result);
        setLoading(false);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : String(e));
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [repos, artifactId, withText, nonce]);

  return { document, loading, error };
}
