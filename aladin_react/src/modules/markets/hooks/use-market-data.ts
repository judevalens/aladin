import { useCallback, useMemo } from "react";

import { useAppComposition } from "@/app/composition/app-composition";
import { useWatchlist } from "@/modules/markets/hooks/use-watchlist";
import { type Quote, DEFAULT_UNIVERSE, buildQuote } from "@/modules/markets/market-data";

export interface UseMarketData {
  quotes: Quote[];
  watched: Set<string>;
  loading: boolean;
  getQuote: (symbol: string) => Quote;
  toggleWatch: (symbol: string) => Promise<void>;
}

/**
 * The Markets data layer. Quotes are placeholder (deterministic per symbol) until the
 * Alpaca feed lands; the watched set + toggle are the REAL per-user watchlist, so watching
 * a ticker here persists. The universe is the watchlist ∪ a default set, so the surface
 * always renders something.
 */
export function useMarketData(): UseMarketData {
  const { repos } = useAppComposition();
  const { items, add, remove, loading } = useWatchlist();

  const symbols = useMemo(() => {
    const seen = new Set<string>();
    const out: string[] = [];
    for (const s of [...items.map((i) => i.symbol), ...DEFAULT_UNIVERSE]) {
      if (!seen.has(s)) {
        seen.add(s);
        out.push(s);
      }
    }
    return out;
  }, [items]);

  const quotes = useMemo(() => symbols.map((s) => buildQuote(s)), [symbols]);
  const watched = useMemo(() => new Set(items.map((i) => i.symbol)), [items]);

  const getQuote = useCallback((symbol: string) => buildQuote(symbol), []);

  // Persisted toggle: remove uses the known watchlist instrument id; add resolves it via
  // ticker search (the market universe may include symbols not yet in the watchlist).
  const toggleWatch = useCallback(
    async (symbol: string) => {
      const item = items.find((i) => i.symbol === symbol);
      if (item) {
        await remove(item.instrumentId);
        return;
      }
      const hits = await repos.instruments.search(symbol, 5);
      const exact = hits.find((h) => h.symbol.toUpperCase() === symbol.toUpperCase());
      if (exact) await add(exact.id);
    },
    [items, add, remove, repos.instruments],
  );

  return { quotes, watched, loading, getQuote, toggleWatch };
}
