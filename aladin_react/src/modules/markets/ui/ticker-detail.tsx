import { useMemo, useState } from "react";
import { Star } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  type Quote,
  type Timeframe,
  TIMEFRAMES,
  fmtCap,
  fmtPct,
  fmtPrice,
  fmtSigned,
  fmtVol,
  headlinesFor,
  seriesFor,
} from "@/modules/markets/market-data";
import { AreaChart } from "@/modules/markets/ui/charts";
import { AddToListMenu } from "@/modules/markets/ui/add-to-list-menu";
import { useBars } from "@/modules/markets/hooks/use-bars";
import { useWatchlists } from "@/modules/markets/hooks/use-watchlists";

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="font-mono text-[10px] uppercase tracking-[0.4px] text-ink-4">{label}</span>
      <span className="font-mono text-[13px] text-ink">{value}</span>
    </div>
  );
}

// A min–max range bar with a marker at `value`.
function RangeBar({ low, high, value }: { low: number; high: number; value: number }) {
  const pct = Math.min(100, Math.max(0, ((value - low) / (high - low || 1)) * 100));
  return (
    <div className="relative h-[3px] w-full rounded-full bg-line-2">
      <div className="absolute inset-y-0 left-0 rounded-full bg-ink-4/50" style={{ width: `${pct}%` }} />
      <div className="absolute top-1/2 size-2 -translate-x-1/2 -translate-y-1/2 rounded-full bg-amber" style={{ left: `${pct}%` }} />
    </div>
  );
}

/**
 * The ticker detail — used both as the Markets right-hand panel and inside the global
 * ticker modal. Reads a Quote (placeholder today, Alpaca-fed later).
 */
export function TickerDetail({
  quote,
  onTrade,
}: {
  quote: Quote;
  onTrade: () => void;
}) {
  const [tf, setTf] = useState<Timeframe>("1D");
  const { lists } = useWatchlists();
  const watched = lists.some((l) => l.items.some((i) => i.symbol === quote.symbol));
  const up = quote.change >= 0;
  // Real bars when we have them; otherwise the deterministic placeholder so the chart never blanks.
  const { series: barSeries } = useBars(quote.symbol, tf);
  const placeholder = useMemo(() => seriesFor(quote, tf), [quote, tf]);
  const series = barSeries.length > 1 ? barSeries : placeholder;
  const news = useMemo(() => headlinesFor(quote.symbol), [quote.symbol]);

  return (
    <div className="flex h-full flex-col overflow-y-auto bg-panel">
      <div className="flex flex-col gap-5 px-6 py-5">
        {/* identity */}
        <div>
          <div className="flex items-center gap-2">
            <h2 className="font-display text-2xl font-semibold text-ink">{quote.symbol}</h2>
            <span className="rounded-chip border border-line px-1.5 py-0.5 font-mono text-[9px] uppercase tracking-[0.4px] text-ink-3">
              {quote.sector}
            </span>
          </div>
          <p className="mt-0.5 text-sm text-ink-3">{quote.name}</p>
        </div>

        {/* price */}
        <div className="flex items-end gap-3">
          <span className="font-display text-4xl font-semibold tracking-[-0.5px] text-ink">
            {fmtPrice(quote.last)}
          </span>
          <span className={cn("mb-1 font-mono text-sm", up ? "text-for" : "text-against")}>
            {up ? "▲" : "▼"} {fmtSigned(quote.change)} {fmtPct(quote.changePct)}
          </span>
        </div>

        {/* timeframe */}
        <div className="flex gap-1">
          {TIMEFRAMES.map((t) => (
            <button
              key={t}
              type="button"
              onClick={() => setTf(t)}
              className={cn(
                "rounded-chip px-2.5 py-1 font-mono text-[11px] transition-colors",
                t === tf ? "bg-raise text-ink" : "text-ink-4 hover:text-ink-2",
              )}
            >
              {t}
            </button>
          ))}
        </div>

        <AreaChart series={series} up={up} />

        {/* day range */}
        <div>
          <div className="mb-2 flex items-baseline justify-between">
            <span className="font-mono text-[10px] uppercase tracking-[0.4px] text-ink-4">Day range</span>
            <span className="font-mono text-[11px] text-ink-3">
              {fmtPrice(quote.low)} – {fmtPrice(quote.high)}
            </span>
          </div>
          <RangeBar low={quote.low} high={quote.high} value={quote.last} />
        </div>

        {/* stat grid */}
        <div className="grid grid-cols-3 gap-x-4 gap-y-4 border-t border-line-2 pt-4">
          <Stat label="Open" value={fmtPrice(quote.open)} />
          <Stat label="High" value={fmtPrice(quote.high)} />
          <Stat label="Low" value={fmtPrice(quote.low)} />
          <Stat label="Prev Close" value={fmtPrice(quote.prevClose)} />
          <Stat label="Volume" value={fmtVol(quote.volume)} />
          <Stat label="Mkt Cap" value={fmtCap(quote.marketCap)} />
        </div>

        {/* 52-week range */}
        <div>
          <div className="mb-2 flex items-baseline justify-between">
            <span className="font-mono text-[10px] uppercase tracking-[0.4px] text-ink-4">52-week range</span>
            <span className="font-mono text-[11px] text-ink-3">
              {Math.round(((quote.last - quote.week52Low) / (quote.week52High - quote.week52Low || 1)) * 100)}% of range
            </span>
          </div>
          <RangeBar low={quote.week52Low} high={quote.week52High} value={quote.last} />
          <div className="mt-1 flex justify-between font-mono text-[10px] text-ink-4">
            <span>{fmtPrice(quote.week52Low)}</span>
            <span>{fmtPrice(quote.week52High)}</span>
          </div>
        </div>

        {/* news */}
        <div className="border-t border-line-2 pt-4">
          <span className="font-mono text-[10px] uppercase tracking-[0.4px] text-ink-4">Latest on {quote.symbol}</span>
          <div className="mt-3 flex flex-col gap-3">
            {news.map((n, i) => (
              <div key={i} className="flex gap-2.5">
                <span className="mt-1 h-full w-[2px] shrink-0 rounded bg-amber-line" />
                <div>
                  <p className="text-[13px] leading-snug text-ink-2">{n.title}</p>
                  <p className="mt-0.5 font-mono text-[10px] text-ink-4">
                    {n.source} · {n.ago}
                  </p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* actions */}
      <div className="sticky bottom-0 mt-auto flex gap-2 border-t border-line bg-panel px-6 py-3">
        <AddToListMenu symbol={quote.symbol} align="start">
          <button
            type="button"
            className={cn(
              "flex flex-1 items-center justify-center gap-2 rounded-card border py-2.5 text-[13px] font-semibold transition-colors",
              watched
                ? "border-amber-line bg-amber-soft text-amber"
                : "border-line text-ink-2 hover:border-amber-line hover:text-ink",
            )}
          >
            <Star className={cn("size-4", watched && "fill-amber")} strokeWidth={1.75} />
            {watched ? "In a list" : "Add to list"}
          </button>
        </AddToListMenu>
        <button
          type="button"
          onClick={onTrade}
          className="flex-1 rounded-card bg-amber py-2.5 text-[13px] font-semibold text-primary-foreground transition-opacity hover:opacity-90"
        >
          Trade
        </button>
      </div>
    </div>
  );
}
