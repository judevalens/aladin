import { useEffect, useState } from "react";

import { useAppComposition } from "@/app/composition/app-composition";
import type { Timeframe } from "@/modules/markets/market-data";

// Maps a UI timeframe to the vendor bar timeframe + how many bars to pull. 1D is intraday
// 5-minute bars; the longer frames are daily bars sliced by count.
const TF_MAP: Record<Timeframe, { timeframe: string; limit: number }> = {
  "1D": { timeframe: "5Min", limit: 100 },
  "1W": { timeframe: "1Day", limit: 7 },
  "1M": { timeframe: "1Day", limit: 22 },
  "6M": { timeframe: "1Day", limit: 130 },
  "1Y": { timeframe: "1Day", limit: 260 },
};

/**
 * Loads the real close-price series for a symbol/timeframe from the bars endpoint (which is
 * read-through cached / lazily filled from Alpaca). Returns [] until loaded or when there's no
 * data — the caller falls back to the placeholder series so the chart always renders.
 */
export function useBars(symbol: string, tf: Timeframe): { series: number[]; loading: boolean } {
  const { repos } = useAppComposition();
  const [series, setSeries] = useState<number[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    const { timeframe, limit } = TF_MAP[tf];
    repos.instruments
      .bars(symbol, timeframe, limit)
      .then((bars) => {
        if (!cancelled) setSeries(bars.map((b) => b.c));
      })
      .catch(() => {
        if (!cancelled) setSeries([]);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [symbol, tf, repos.instruments]);

  return { series, loading };
}
