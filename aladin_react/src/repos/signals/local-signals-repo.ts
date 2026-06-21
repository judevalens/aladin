import { invoke } from "@tauri-apps/api/core";

import type { DataEventsRepo } from "@/repos/data-events-repo";
import type { ClaimSignal, SignalSort } from "@/modules/signals/signal-types";

// The Signals feed read from the LOCAL `signals` cache (fed by sync frames over the ws). Each
// sort is a small external store (useSyncExternalStore-friendly): it lists from SQLite, then
// re-lists whenever a signal frame applies (signalUpserted/Deleted) so the feed re-ranks live.
export interface SignalStore {
  subscribe: (onChange: () => void) => () => void;
  snapshot: () => ClaimSignal[] | undefined; // undefined = still loading
}

export interface LocalSignalsRepo {
  observe: (sort: SignalSort) => SignalStore;
}

export function createLocalSignalsRepo(dataEvents: DataEventsRepo): LocalSignalsRepo {
  const stores = new Map<SignalSort, SignalStore>();

  function build(sort: SignalSort): SignalStore {
    let current: ClaimSignal[] | undefined;
    const listeners = new Set<() => void>();
    let eventSub: { unsubscribe: () => void } | null = null;

    const notify = () => listeners.forEach((l) => l());
    const refresh = () => {
      invoke<ClaimSignal[]>("db_list_signals", { sort })
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
            if (e.type === "signalUpserted" || e.type === "signalDeleted") refresh();
          });
        }
        return () => {
          listeners.delete(onChange);
          if (listeners.size === 0) {
            eventSub?.unsubscribe();
            eventSub = null;
            stores.delete(sort);
          }
        };
      },
      snapshot: () => current,
    };
  }

  return {
    observe(sort) {
      let store = stores.get(sort);
      if (!store) {
        store = build(sort);
        stores.set(sort, store);
      }
      return store;
    },
  };
}
