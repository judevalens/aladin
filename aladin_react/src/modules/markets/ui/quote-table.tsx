import { useMemo, useState } from "react";
import { Star } from "lucide-react";

import { cn } from "@/lib/utils";
import { type Quote, fmtPct, fmtPrice, fmtSigned, fmtVol } from "@/modules/markets/market-data";
import { Sparkline } from "@/modules/markets/ui/charts";

const TABS = ["Watchlist", "Gainers", "Losers", "Most Active"] as const;
type Tab = (typeof TABS)[number];

/**
 * The quote table under the market map. Tabs re-sort the same quote set; the star toggles
 * watchlist membership; a row selects the symbol into the detail panel.
 */
export function QuoteTable({
  quotes,
  watched,
  selected,
  onSelect,
  onToggleWatch,
}: {
  quotes: Quote[];
  watched: Set<string>;
  selected: string;
  onSelect: (symbol: string) => void;
  onToggleWatch: (symbol: string) => void;
}) {
  const [tab, setTab] = useState<Tab>("Gainers");

  const rows = useMemo(() => {
    const list = [...quotes];
    switch (tab) {
      case "Watchlist":
        return list.filter((q) => watched.has(q.symbol));
      case "Gainers":
        return list.sort((a, b) => b.changePct - a.changePct);
      case "Losers":
        return list.sort((a, b) => a.changePct - b.changePct);
      case "Most Active":
        return list.sort((a, b) => b.volume - a.volume);
    }
  }, [quotes, tab, watched]);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex items-center gap-4 border-b border-line px-1 py-2.5">
        {TABS.map((t) => (
          <button
            key={t}
            type="button"
            onClick={() => setTab(t)}
            className={cn(
              "rounded-chip px-2 py-1 text-[13px] transition-colors",
              t === tab ? "bg-raise font-semibold text-ink" : "text-ink-3 hover:text-ink",
            )}
          >
            {t}
          </button>
        ))}
        <span className="ml-auto font-mono text-[11px] text-ink-4">{rows.length} symbols</span>
      </div>

      <div className="grid grid-cols-[1.6fr_1fr_1fr_0.9fr_0.9fr_0.8fr_28px] gap-2 border-b border-line-2 px-2 py-1.5 font-mono text-[10px] uppercase tracking-[0.4px] text-ink-4">
        <span>Symbol</span>
        <span className="text-right">Last</span>
        <span className="text-right">Chg</span>
        <span className="text-right">%</span>
        <span className="text-center">Trend</span>
        <span className="text-right">Vol</span>
        <span />
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto">
        {rows.length === 0 ? (
          <p className="px-2 py-8 text-center text-[13px] text-ink-4">Nothing here yet.</p>
        ) : (
          rows.map((q) => {
            const up = q.change >= 0;
            const isSel = q.symbol === selected;
            const isWatched = watched.has(q.symbol);
            return (
              <button
                key={q.symbol}
                type="button"
                onClick={() => onSelect(q.symbol)}
                className={cn(
                  "grid w-full grid-cols-[1.6fr_1fr_1fr_0.9fr_0.9fr_0.8fr_28px] items-center gap-2 border-b border-line-2 px-2 py-2.5 text-left transition-colors hover:bg-card",
                  isSel && "bg-card",
                )}
              >
                <span className="flex flex-col">
                  <span className="font-display text-[13px] font-semibold text-ink">{q.symbol}</span>
                  <span className="truncate text-[11px] text-ink-4">{q.name}</span>
                </span>
                <span className="text-right font-mono text-[13px] text-ink">{fmtPrice(q.last)}</span>
                <span className={cn("text-right font-mono text-[12px]", up ? "text-for" : "text-against")}>
                  {fmtSigned(q.change)}
                </span>
                <span
                  className={cn(
                    "justify-self-end rounded-chip px-1.5 py-0.5 text-right font-mono text-[12px]",
                    up ? "bg-for/15 text-for" : "bg-against/15 text-against",
                  )}
                >
                  {fmtPct(q.changePct)}
                </span>
                <span className="flex justify-center">
                  <Sparkline series={q.spark} up={up} />
                </span>
                <span className="text-right font-mono text-[12px] text-ink-3">{fmtVol(q.volume)}</span>
                <span
                  role="button"
                  tabIndex={-1}
                  aria-label={isWatched ? `Unwatch ${q.symbol}` : `Watch ${q.symbol}`}
                  onClick={(e) => {
                    e.stopPropagation();
                    onToggleWatch(q.symbol);
                  }}
                  className="grid size-6 place-items-center rounded text-ink-4 hover:text-amber"
                >
                  <Star className={cn("size-4", isWatched && "fill-amber text-amber")} strokeWidth={1.75} />
                </span>
              </button>
            );
          })
        )}
      </div>
    </div>
  );
}
