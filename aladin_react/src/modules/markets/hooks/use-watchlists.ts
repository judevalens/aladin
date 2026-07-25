import { useCallback, useEffect, useMemo, useRef, useSyncExternalStore } from "react";

import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";
import type { LocalWatchlist } from "@/repos/watchlist/local-watchlist-types";

export interface UseWatchlists {
  lists: LocalWatchlist[];
  activeId: string | null;
  loading: boolean;
  setActive: (id: string) => void;
  create: (name: string) => Promise<string | null>;
  rename: (id: string, name: string) => Promise<void>;
  remove: (id: string) => Promise<void>;
  addItem: (listId: string, instrumentId: string) => Promise<void>;
  removeItem: (listId: string, instrumentId: string) => Promise<void>;
}

/**
 * The user's named watchlists WITH their members, read reactively from the LOCAL frame-fed cache
 * (a watchlist is one synced entity carrying its items[], so the switcher, the modal manager, the
 * map, and the star menu all read one store). Mutations proxy to Go over REST; the change returns
 * as a `watchlist` frame that updates the store — no invalidation.
 *
 * Active-list = the "viewed" list the map/table default to; a stale persisted id is repaired ONCE
 * on first load, then we only fill a null selection (so a just-created list isn't clobbered).
 */
export function useWatchlists(): UseWatchlists {
  const { repos } = useAppComposition();
  const store = useMemo(() => repos.localWatchlists.observe(), [repos.localWatchlists]);
  const lists = useSyncExternalStore(store.subscribe, store.snapshot);
  const activeId = useAppStore((s) => s.activeWatchlistId);
  const setActive = useAppStore((s) => s.setActiveWatchlistId);

  const reconciled = useRef(false);
  useEffect(() => {
    if (lists === undefined) return;
    if (!reconciled.current) {
      reconciled.current = true;
      if (lists.length > 0 && !lists.some((l) => l.id === activeId)) setActive(lists[0].id);
      return;
    }
    if (activeId === null && lists.length > 0) setActive(lists[0].id);
  }, [lists, activeId, setActive]);

  const create = useCallback(
    async (name: string) => {
      const list = await repos.watchlist.createWatchlist(name);
      setActive(list.id); // the list's frame populates the store shortly after
      return list.id;
    },
    [repos, setActive],
  );

  const rename = useCallback(
    async (id: string, name: string) => {
      await repos.watchlist.renameWatchlist(id, name);
    },
    [repos],
  );

  const remove = useCallback(
    async (id: string) => {
      await repos.watchlist.deleteWatchlist(id);
      if (id === useAppStore.getState().activeWatchlistId) {
        const next = (store.snapshot() ?? []).find((l) => l.id !== id);
        setActive(next?.id ?? null);
      }
    },
    [repos, store, setActive],
  );

  const addItem = useCallback(
    async (listId: string, instrumentId: string) => {
      await repos.watchlist.addItem(listId, instrumentId);
    },
    [repos],
  );

  const removeItem = useCallback(
    async (listId: string, instrumentId: string) => {
      await repos.watchlist.removeItem(listId, instrumentId);
    },
    [repos],
  );

  return {
    lists: lists ?? [],
    activeId,
    loading: lists === undefined,
    setActive,
    create,
    rename,
    remove,
    addItem,
    removeItem,
  };
}
