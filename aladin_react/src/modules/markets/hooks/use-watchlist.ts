import { useCallback, useMemo, useSyncExternalStore } from "react";

import { useAppComposition } from "@/app/composition/app-composition";
import type { LocalWatchlistItem } from "@/repos/watchlist/local-watchlist-types";

export interface UseWatchlist {
  items: LocalWatchlistItem[];
  loading: boolean;
  add: (instrumentId: string) => Promise<void>;
  remove: (instrumentId: string) => Promise<void>;
}

/**
 * The items of the ACTIVE watchlist, read reactively from the LOCAL frame-fed cache (a watchlist is
 * one synced entity carrying its members, so its items are already in the store — no separate
 * fetch). add/remove proxy to Go over REST; the list's frame re-lands with the new membership, so
 * the UI updates without an optimistic patch or refetch. Waits for a non-null listId.
 */
export function useWatchlist(listId: string | null): UseWatchlist {
  const { repos } = useAppComposition();
  const store = useMemo(() => repos.localWatchlists.observe(), [repos.localWatchlists]);
  const local = useSyncExternalStore(store.subscribe, store.snapshot);

  const items = useMemo<LocalWatchlistItem[]>(() => {
    if (!listId || !local) return [];
    return local.find((l) => l.id === listId)?.items ?? [];
  }, [local, listId]);

  const add = useCallback(
    async (instrumentId: string) => {
      if (!listId) return;
      await repos.watchlist.addItem(listId, instrumentId);
    },
    [repos, listId],
  );

  const remove = useCallback(
    async (instrumentId: string) => {
      if (!listId) return;
      await repos.watchlist.removeItem(listId, instrumentId);
    },
    [repos, listId],
  );

  return { items, loading: local === undefined, add, remove };
}
