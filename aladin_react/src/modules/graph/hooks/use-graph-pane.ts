import { useCallback, useEffect, useState } from "react";
import { debounceTime, filter } from "rxjs";

import { useAppComposition } from "@/app/composition/app-composition";
import type { GraphPane } from "@/modules/graph/graph-pane-types";

export interface UseGraphPane {
  pane: GraphPane | null;
  loading: boolean;
  error: string | null;
  reload: () => void;
}

/**
 * Loads the "On the graph" pane for the artifact currently being viewed. The
 * pane shows where that artifact sits in the knowledge graph — its entities,
 * connected claims, and cited sources.
 */
export function useGraphPane(artifactId: string): UseGraphPane {
  const { repos, runtime } = useAppComposition();
  const [pane, setPane] = useState<GraphPane | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);

  const reload = useCallback(() => setNonce((n) => n + 1), []);

  // Reactive: the pane is a derived aggregate, so instead of carrying its data on the
  // frame we refetch when a node frame for THIS artifact arrives via the syncer. Every
  // write that changes the artifact's graph inputs — @entity/#ref/attach edits AND async
  // page ingestion — emits one server-side, so this covers all of them (no invalidation).
  useEffect(() => {
    if (!artifactId) return;
    const sub = runtime.dataEvents
      .events()
      .pipe(
        filter((event) => event.type === "nodeUpserted" && event.payload.id === artifactId),
        // Coalesce bursts (the editor re-syncs mentions/refs on each keystroke) so we
        // refetch once things settle, not per keystroke.
        debounceTime(500),
      )
      .subscribe(() => reload());
    return () => sub.unsubscribe();
  }, [runtime, artifactId, reload]);

  useEffect(() => {
    if (!artifactId) {
      setPane(null);
      setError(null);
      setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    repos.graphPane
      .getPane(artifactId)
      .then((result) => {
        if (cancelled) return;
        setPane(result);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setPane(null);
        setError(err instanceof Error ? err.message : "Failed to load the graph pane");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [repos.graphPane, artifactId, nonce]);

  return { pane, loading, error, reload };
}
