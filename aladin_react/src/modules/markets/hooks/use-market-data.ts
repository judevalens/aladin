import { useCallback, useMemo } from "react";

import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";
import type { LiveQuote } from "@/app/state/market-slice";
import { useWatchlist } from "@/modules/markets/hooks/use-watchlist";
import { type Quote, DEFAULT_UNIVERSE, buildQuote } from "@/modules/markets/market-data";

// overlay applies a live tick onto a placeholder quote: the real last price, with change /
// changePct recomputed against the (placeholder) prev close. Sparkline/stats stay placeholder
// until bars feed them.
function overlay(base: Quote, live: LiveQuote | undefined): Quote {
  if (!live) return base;
  // Prefer the seeded real prevClose (snapshot); fall back to the placeholder's. Change is
  // recomputed from it so bare WS ticks (last only) stay consistent.
  const prevClose = live.prevClose ?? base.prevClose;
  const change = live.last - prevClose;
  return {
    ...base,
    last: live.last,
    prevClose,
    change,
    changePct: prevClose ? (change / prevClose) * 100 : base.changePct,
  };
}

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
  const activeWatchlistId = useAppStore((s) => s.activeWatchlistId);
  const { items, add, remove, loading } = useWatchlist(activeWatchlistId);

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

  const liveQuotes = useAppStore((s) => s.liveQuotes);
  const quotes = useMemo(
    () => symbols.map((s) => overlay(buildQuote(s), liveQuotes[s.toUpperCase()])),
    [symbols, liveQuotes],
  );
  const watched = useMemo(() => new Set(items.map((i) => i.symbol)), [items]);

  const getQuote = useCallback(
    (symbol: string) => overlay(buildQuote(symbol), liveQuotes[symbol.toUpperCase()]),
    [liveQuotes],
  );

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
