import type { StateCreator } from "zustand";

const localStorageKey = "aladin.markets.activeWatchlistId";

function readPersisted(): string | null {
  try {
    return window.localStorage.getItem(localStorageKey);
  } catch {
    return null;
  }
}

function persist(id: string | null) {
  try {
    if (id) window.localStorage.setItem(localStorageKey, id);
    else window.localStorage.removeItem(localStorageKey);
  } catch {
    // storage unavailable — selection just won't persist across reloads
  }
}

/**
 * The active watchlist (universe) selected on the Markets surface. Persisted so a reload lands
 * back on the same list. The lists themselves are fetched per-render by useWatchlists; only the
 * selection is app state.
 */
export interface WatchlistsSlice {
  activeWatchlistId: string | null;
  setActiveWatchlistId: (id: string | null) => void;
  /** The id persisted from the previous run (for initial selection). */
  persistedWatchlistId: () => string | null;
}

export const createWatchlistsSlice: StateCreator<WatchlistsSlice, [], [], WatchlistsSlice> = (set) => ({
  activeWatchlistId: readPersisted(),

  setActiveWatchlistId: (id) => {
    persist(id);
    set({ activeWatchlistId: id });
  },

  persistedWatchlistId: () => readPersisted(),
});
