import { useCallback, useEffect, useState } from "react";

import { useAppComposition } from "@/app/composition/app-composition";
import type { WatchlistItem } from "@/shared/api/models";

export interface UseWatchlist {
  items: WatchlistItem[];
  loading: boolean;
  error: string | null;
  add: (instrumentId: string) => Promise<void>;
  remove: (instrumentId: string) => Promise<void>;
}

// Loads the user's watchlist and exposes add/remove. Both mutations optimistically patch
// the list, then reload from the server so the canonical order/state wins.
export function useWatchlist(): UseWatchlist {
  const { repos } = useAppComposition();
  const [items, setItems] = useState<WatchlistItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);

  const reload = useCallback(() => setNonce((n) => n + 1), []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    repos.watchlist
      .list()
      .then((result) => {
        if (!cancelled) setItems(result);
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "Failed to load watchlist");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [repos, nonce]);

  const add = useCallback(
    async (instrumentId: string) => {
      await repos.watchlist.add(instrumentId);
      reload();
    },
    [repos, reload],
  );

  const remove = useCallback(
    async (instrumentId: string) => {
      setItems((prev) => prev.filter((i) => i.instrumentId !== instrumentId));
      await repos.watchlist.remove(instrumentId);
      reload();
    },
    [repos, reload],
  );

  return { items, loading, error, add, remove };
}
