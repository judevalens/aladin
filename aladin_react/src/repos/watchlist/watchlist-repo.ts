import type { ApiClient } from "@/shared/api/client";
import type { WatchlistItem } from "@/shared/api/models";

export interface WatchlistRepo {
  list(): Promise<WatchlistItem[]>;
  add(instrumentId: string): Promise<void>;
  remove(instrumentId: string): Promise<void>;
}

export function createWatchlistRepo(client: ApiClient): WatchlistRepo {
  return {
    list: () => client.fetch<WatchlistItem[]>("/api/watchlist").then((items) => items ?? []),
    add: (instrumentId) =>
      client
        .fetch<{ ok: boolean }>("/api/watchlist", {
          method: "POST",
          body: JSON.stringify({ instrumentId }),
        })
        .then(() => undefined),
    remove: (instrumentId) =>
      client
        .fetch<{ ok: boolean }>(`/api/watchlist/${encodeURIComponent(instrumentId)}`, {
          method: "DELETE",
        })
        .then(() => undefined),
  };
}
