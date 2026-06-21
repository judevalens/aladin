import { useCallback, useEffect, useState } from "react";

import { useAppComposition } from "@/app/composition/app-composition";
import type { Insight } from "@/modules/insights/insight-types";

export interface UseInsights {
  insights: Insight[];
  loading: boolean;
  error: string | null;
  status: string;
  setStatus: (status: string) => void;
  accept: (id: string) => void;
  dismiss: (id: string) => void;
}

// Loads insights for a status (pending by default) and exposes accept/dismiss that
// optimistically drop the card, then reload.
export function useInsights(): UseInsights {
  const { repos } = useAppComposition();
  const [status, setStatus] = useState("pending");
  const [insights, setInsights] = useState<Insight[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);

  const reload = useCallback(() => setNonce((n) => n + 1), []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    repos.insights
      .list(status)
      .then((result) => {
        if (!cancelled) setInsights(result);
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "Failed to load insights");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [repos.insights, status, nonce]);

  const act = useCallback(
    (id: string, fn: (id: string) => Promise<void>) => {
      setInsights((cur) => cur.filter((i) => i.id !== id)); // optimistic
      fn(id)
        .catch(() => reload()) // restore truth on failure
        .then(() => undefined);
    },
    [reload],
  );

  const accept = useCallback((id: string) => act(id, repos.insights.accept), [act, repos.insights]);
  const dismiss = useCallback((id: string) => act(id, repos.insights.dismiss), [act, repos.insights]);

  return { insights, loading, error, status, setStatus, accept, dismiss };
}
