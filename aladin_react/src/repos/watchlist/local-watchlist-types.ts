// The watchlist as it arrives over the sync spine (entity kind "watchlist") and is cached in the
// local `watchlists` table. Mirrors the Rust WatchlistRow serde shape 1:1. A list is ONE synced
// entity carrying its ordered members, so the switcher (list metadata) and the active-list view
// (items) both read from this same row — no separate items fetch.

export interface LocalWatchlistItem {
  instrumentId: string;
  symbol: string;
  name: string;
  position: number;
}

export interface LocalWatchlist {
  id: string;
  name: string;
  kind: string; // manual | screener | hybrid
  position: number;
  items: LocalWatchlistItem[];
  itemCount: number;
  updatedAt: number;
}
