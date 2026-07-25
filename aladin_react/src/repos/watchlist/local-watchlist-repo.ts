import { invoke } from "@tauri-apps/api/core";

import type { DataEventsRepo } from "@/repos/data-events-repo";
import type { LocalWatchlist } from "@/repos/watchlist/local-watchlist-types";

// The Markets watchlists read from the LOCAL `watchlists` cache (fed by sync frames over the ws).
// A single external store (useSyncExternalStore-friendly): it lists from SQLite, then re-lists
// whenever a watchlist frame applies (watchlistUpserted/Deleted) so the switcher AND the active
// list's items update live — no refetch-on-mutation. Writes still proxy to Go over REST; the
// change returns as a frame that lands here.
export interface WatchlistsStore {
  subscribe: (onChange: () => void) => () => void;
  snapshot: () => LocalWatchlist[] | undefined; // undefined = still loading
}

export interface LocalWatchlistsRepo {
  observe: () => WatchlistsStore;
}

export function createLocalWatchlistsRepo(dataEvents: DataEventsRepo): LocalWatchlistsRepo {
  let store: WatchlistsStore | null = null;

  function build(): WatchlistsStore {
    let current: LocalWatchlist[] | undefined;
    const listeners = new Set<() => void>();
    let eventSub: { unsubscribe: () => void } | null = null;

    const notify = () => listeners.forEach((l) => l());
    const refresh = () => {
      invoke<LocalWatchlist[]>("db_list_watchlists")
        .then((rows) => {
          current = rows;
          notify();
        })
        .catch(() => {
          if (current === undefined) {
            current = [];
            notify();
          }
        });
    };

    return {
      subscribe(onChange) {
        listeners.add(onChange);
        if (!eventSub) {
          refresh();
          eventSub = dataEvents.events().subscribe((e) => {
            if (e.type === "watchlistUpserted" || e.type === "watchlistDeleted") refresh();
          });
        }
        return () => {
          listeners.delete(onChange);
          if (listeners.size === 0) {
            eventSub?.unsubscribe();
            eventSub = null;
            store = null;
          }
        };
      },
      snapshot: () => current,
    };
  }

  return {
    observe() {
      if (!store) store = build();
      return store;
    },
  };
}
