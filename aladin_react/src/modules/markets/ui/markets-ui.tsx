import { useEffect, useMemo, useState } from "react";
import { Bell, Check, ChevronDown, Pause, Search, Sun } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { type Quote, INDICES, fmtPct } from "@/modules/markets/market-data";
import { useMarketData } from "@/modules/markets/hooks/use-market-data";
import { useQuoteSubscription } from "@/modules/markets/hooks/use-quote-subscription";
import { MarketMap } from "@/modules/markets/ui/market-map";
import { QuoteTable } from "@/modules/markets/ui/quote-table";
import { TickerDetail } from "@/modules/markets/ui/ticker-detail";
import { useAppStore } from "@/app/state/store";

// The map's selectable datasets. `pick` derives the subset the treemap renders.
const MAP_DATASETS = [
  { key: "watchlist", label: "My Watchlist" },
  { key: "all", label: "All Symbols" },
  { key: "gainers", label: "Gainers" },
  { key: "losers", label: "Losers" },
  { key: "active", label: "Most Active" },
] as const;
type MapDatasetKey = (typeof MAP_DATASETS)[number]["key"];

function pickDataset(quotes: Quote[], watched: Set<string>, key: MapDatasetKey): Quote[] {
  switch (key) {
    case "watchlist":
      return quotes.filter((q) => watched.has(q.symbol));
    case "gainers":
      return quotes.filter((q) => q.change >= 0);
    case "losers":
      return quotes.filter((q) => q.change < 0);
    case "active":
      return [...quotes].sort((a, b) => b.volume - a.volume).slice(0, 16);
    default:
      return quotes;
  }
}

function IndexPill({ label, value, changePct }: { label: string; value: number; changePct: number }) {
  const up = changePct >= 0;
  return (
    <div className="flex items-baseline gap-2 whitespace-nowrap border-r border-line px-4">
      <span className="font-mono text-[11px] uppercase tracking-[0.4px] text-ink-4">{label}</span>
      <span className="font-mono text-[13px] text-ink">{value.toLocaleString()}</span>
      <span className={cn("font-mono text-[11px]", up ? "text-for" : "text-against")}>
        {up ? "▲" : "▼"} {fmtPct(changePct)}
      </span>
    </div>
  );
}

/**
 * The Markets surface — a market map + quote table on the left, a live ticker detail on the
 * right. Prices are placeholder (deterministic) until the Alpaca feed lands; the watchlist
 * is real and persisted.
 */
export function MarketsUI() {
  const { quotes, watched, toggleWatch } = useMarketData();
  const openTicker = useAppStore((s) => s.openTicker);
  const [selected, setSelected] = useState<string>("");
  const [mapDataset, setMapDataset] = useState<MapDatasetKey>("all");

  const mapQuotes = useMemo(() => pickDataset(quotes, watched, mapDataset), [quotes, watched, mapDataset]);
  const datasetLabel = MAP_DATASETS.find((d) => d.key === mapDataset)?.label ?? "All Symbols";

  // Register live-quote demand for every symbol on the surface.
  useQuoteSubscription(useMemo(() => quotes.map((q) => q.symbol), [quotes]));

  // Default the detail panel to the first quote once data is in.
  useEffect(() => {
    if (!selected && quotes.length > 0) setSelected(quotes[0].symbol);
  }, [quotes, selected]);

  const selectedQuote = useMemo(
    () => quotes.find((q) => q.symbol === selected) ?? quotes[0],
    [quotes, selected],
  );

  const advancers = mapQuotes.filter((q) => q.change >= 0).length;
  const decliners = mapQuotes.length - advancers;

  return (
    <div className="flex h-full min-w-0 flex-1 flex-col overflow-hidden bg-bg">
      {/* top bar */}
      <header className="flex shrink-0 items-center gap-3 border-b border-line px-5 py-3">
        <h1 className="font-display text-lg font-semibold text-ink">Markets</h1>
        <span className="text-[13px] text-ink-4">US Equities &amp; ETFs</span>
        <div className="ml-auto flex items-center gap-2">
          <button
            type="button"
            onClick={() => useAppStore.getState().setCommandPaletteOpen(true)}
            className="flex w-[240px] items-center gap-2 rounded-card border border-line bg-field px-3 py-1.5 text-left text-[13px] text-ink-4 transition-colors hover:border-line-2"
          >
            <Search className="size-3.5" strokeWidth={1.75} />
            Search symbol…
            <span className="ml-auto font-mono text-[11px] text-ink-4">/</span>
          </button>
          <button type="button" className="grid size-8 place-items-center rounded-card text-ink-3 hover:text-ink" aria-label="Notifications">
            <Bell className="size-[17px]" strokeWidth={1.7} />
          </button>
          <button type="button" className="grid size-8 place-items-center rounded-card text-ink-3 hover:text-ink" aria-label="Theme">
            <Sun className="size-[17px]" strokeWidth={1.7} />
          </button>
        </div>
      </header>

      {/* index strip */}
      <div className="flex shrink-0 items-center border-b border-line bg-chrome/40 pr-4 text-[13px]">
        <div className="flex items-center gap-2 whitespace-nowrap border-r border-line px-4 py-2.5">
          <span className="size-1.5 rounded-full bg-for" />
          <span className="font-mono text-[11px] uppercase tracking-[0.5px] text-ink-3">Mkt Open</span>
        </div>
        <div className="flex min-w-0 flex-1 items-center overflow-x-auto py-2.5">
          {INDICES.map((idx) => (
            <IndexPill key={idx.label} {...idx} />
          ))}
        </div>
        <div className="flex items-center gap-2 whitespace-nowrap pl-4">
          <span className="flex items-center gap-1.5 rounded-chip border border-line px-2 py-1 font-mono text-[11px] text-ink-3">
            <Pause className="size-3" strokeWidth={2} /> LIVE
          </span>
          <span className="font-mono text-[11px] text-ink-4">09:26:13 ET</span>
        </div>
      </div>

      {/* body: left column + right detail */}
      <div className="flex min-h-0 flex-1">
        <div className="flex min-w-0 flex-1 flex-col gap-4 overflow-hidden px-5 py-4">
          {/* market map */}
          <div className="shrink-0">
            <div className="mb-3 flex items-center gap-3">
              <h2 className="font-display text-[15px] font-semibold text-ink">Market Map</h2>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <button
                    type="button"
                    className="flex items-center gap-1.5 rounded-chip border border-line px-2 py-0.5 font-mono text-[11px] text-ink-2 transition-colors hover:border-line-2"
                  >
                    {datasetLabel} · {mapQuotes.length}
                    <ChevronDown className="size-3 text-ink-4" strokeWidth={2} />
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start" className="w-44">
                  {MAP_DATASETS.map((d) => (
                    <DropdownMenuItem key={d.key} onClick={() => setMapDataset(d.key)} className="justify-between">
                      {d.label}
                      {d.key === mapDataset && <Check className="size-3.5 text-amber" strokeWidth={2} />}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
              <span className="text-[11px] text-ink-4">sized by cap · colored by day change</span>
              <span className="ml-auto flex items-center gap-3 font-mono text-[11px]">
                <span className="flex items-center gap-1 text-ink-4">
                  <span className="size-2 rounded-[2px] bg-for" /> {advancers} adv
                </span>
                <span className="flex items-center gap-1 text-ink-4">
                  <span className="size-2 rounded-[2px] bg-against" /> {decliners} dec
                </span>
              </span>
            </div>
            <MarketMap quotes={mapQuotes} selected={selectedQuote?.symbol ?? ""} onSelect={setSelected} />
          </div>

          {/* table */}
          <QuoteTable
            quotes={quotes}
            watched={watched}
            selected={selectedQuote?.symbol ?? ""}
            onSelect={setSelected}
            onToggleWatch={toggleWatch}
          />
        </div>

        {/* right detail */}
        <div className="w-[400px] shrink-0 border-l border-line">
          {selectedQuote && (
            <TickerDetail
              quote={selectedQuote}
              watched={watched.has(selectedQuote.symbol)}
              onToggleWatch={() => toggleWatch(selectedQuote.symbol)}
              onTrade={() => openTicker(selectedQuote.symbol)}
            />
          )}
        </div>
      </div>
    </div>
  );
}
