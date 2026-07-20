import type { ApiClient } from "@/shared/api/client";

// The market demand API: register/release live-quote interest in symbols. The backend hub
// dedups to one upstream Alpaca subscription per symbol; ticks fan out over the market stream.
export interface MarketRepo {
  subscribe(symbols: string[]): Promise<void>;
  unsubscribe(symbols: string[]): Promise<void>;
}

export function createMarketRepo(client: ApiClient): MarketRepo {
  const post = (path: string, symbols: string[]) => {
    if (symbols.length === 0) return Promise.resolve();
    return client
      .fetch<{ ok: boolean }>(path, { method: "POST", body: JSON.stringify({ symbols }) })
      .then(() => undefined)
      .catch(() => undefined); // demand is best-effort; a miss just means no live ticks
  };
  return {
    subscribe: (symbols) => post("/api/market/subscribe", symbols),
    unsubscribe: (symbols) => post("/api/market/unsubscribe", symbols),
  };
}
