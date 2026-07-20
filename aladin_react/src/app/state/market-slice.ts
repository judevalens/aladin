import type { StateCreator } from "zustand";

/** A live quote for a symbol. The snapshot seed carries prevClose; later WS ticks carry only
 *  `last`, so the slice MERGES (keeping prevClose) rather than replacing. */
export interface LiveQuote {
  symbol: string;
  instrumentId?: string;
  last: number;
  change?: number;
  changePct?: number;
  prevClose?: number;
  ts?: string;
}

export interface MarketSlice {
  /** Latest live quote per symbol (uppercase). Fed by the quote.update realtime handler. */
  liveQuotes: Record<string, LiveQuote>;
  applyQuote: (quote: LiveQuote) => void;
}

export const createMarketSlice: StateCreator<MarketSlice, [], [], MarketSlice> = (set) => ({
  liveQuotes: {},
  applyQuote: (quote) =>
    set((state) => {
      const symbol = quote.symbol.toUpperCase();
      const prev = state.liveQuotes[symbol];
      // Merge, ignoring undefined — a bare tick (last only) must not wipe the seeded prevClose.
      const next: LiveQuote = { ...prev, symbol, last: quote.last };
      if (quote.instrumentId !== undefined) next.instrumentId = quote.instrumentId;
      if (quote.change !== undefined) next.change = quote.change;
      if (quote.changePct !== undefined) next.changePct = quote.changePct;
      if (quote.prevClose !== undefined) next.prevClose = quote.prevClose;
      if (quote.ts !== undefined) next.ts = quote.ts;
      return { liveQuotes: { ...state.liveQuotes, [symbol]: next } };
    }),
});
