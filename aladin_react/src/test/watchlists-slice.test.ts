import { beforeEach, describe, expect, it } from "vitest";
import { create } from "zustand";
import { createWatchlistsSlice, type WatchlistsSlice } from "@/app/state/watchlists-slice";

function makeStore() {
  return create<WatchlistsSlice>()((...args) => createWatchlistsSlice(...args));
}

// The vitest env ships an incomplete localStorage; install a working in-memory one so the
// persistence path is exercised deterministically (production uses the browser's).
beforeEach(() => {
  const store = new Map<string, string>();
  Object.defineProperty(window, "localStorage", {
    configurable: true,
    value: {
      getItem: (k: string) => (store.has(k) ? store.get(k)! : null),
      setItem: (k: string, v: string) => void store.set(k, String(v)),
      removeItem: (k: string) => void store.delete(k),
      clear: () => store.clear(),
    },
  });
});

describe("watchlists slice", () => {
  it("persists the active list id to localStorage and reads it back", () => {
    const store = makeStore();
    expect(store.getState().activeWatchlistId).toBeNull();

    store.getState().setActiveWatchlistId("list-42");
    expect(store.getState().activeWatchlistId).toBe("list-42");
    expect(window.localStorage.getItem("aladin.markets.activeWatchlistId")).toBe("list-42");

    // A fresh store instance hydrates the persisted selection.
    const reloaded = makeStore();
    expect(reloaded.getState().activeWatchlistId).toBe("list-42");
    expect(reloaded.getState().persistedWatchlistId()).toBe("list-42");
  });

  it("clears the persisted id when set to null", () => {
    const store = makeStore();
    store.getState().setActiveWatchlistId("x");
    store.getState().setActiveWatchlistId(null);
    expect(store.getState().activeWatchlistId).toBeNull();
    expect(window.localStorage.getItem("aladin.markets.activeWatchlistId")).toBeNull();
  });
});
