import type { ApiClient } from "@/shared/api/client";
import type { WatchlistItem } from "@/shared/api/models";

/** A named watchlist (a universe): manual members, or a screener rule (dynamic, v2). */
export interface Watchlist {
  id: string;
  name: string;
  kind: string; // manual | screener | hybrid
  position: number;
  itemCount: number;
  createdAt: string;
}

export interface WatchlistRepo {
  listWatchlists(): Promise<Watchlist[]>;
  createWatchlist(name: string): Promise<Watchlist>;
  renameWatchlist(id: string, name: string): Promise<void>;
  deleteWatchlist(id: string): Promise<void>;
  listItems(listId: string): Promise<WatchlistItem[]>;
  addItem(listId: string, instrumentId: string): Promise<void>;
  removeItem(listId: string, instrumentId: string): Promise<void>;
}

export function createWatchlistRepo(client: ApiClient): WatchlistRepo {
  return {
    listWatchlists: () =>
      client
        .fetch<{ watchlists: Watchlist[] | null }>("/api/watchlists")
        .then((r) => r.watchlists ?? []),

    createWatchlist: (name) =>
      client.fetch<Watchlist>("/api/watchlists", {
        method: "POST",
        body: JSON.stringify({ name }),
      }),

    renameWatchlist: (id, name) =>
      client
        .fetch<{ ok: boolean }>(`/api/watchlists/${encodeURIComponent(id)}`, {
          method: "PATCH",
          body: JSON.stringify({ name }),
        })
        .then(() => undefined),

    deleteWatchlist: (id) =>
      client
        .fetch<{ ok: boolean }>(`/api/watchlists/${encodeURIComponent(id)}`, { method: "DELETE" })
        .then(() => undefined),

    listItems: (listId) =>
      client
        .fetch<WatchlistItem[]>(`/api/watchlists/${encodeURIComponent(listId)}/items`)
        .then((items) => items ?? []),

    addItem: (listId, instrumentId) =>
      client
        .fetch<{ ok: boolean }>(`/api/watchlists/${encodeURIComponent(listId)}/items`, {
          method: "POST",
          body: JSON.stringify({ instrumentId }),
        })
        .then(() => undefined),

    removeItem: (listId, instrumentId) =>
      client
        .fetch<{ ok: boolean }>(
          `/api/watchlists/${encodeURIComponent(listId)}/items/${encodeURIComponent(instrumentId)}`,
          { method: "DELETE" },
        )
        .then(() => undefined),
  };
}
