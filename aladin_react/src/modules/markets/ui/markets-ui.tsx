import { useEffect, useMemo, useState } from "react";
import { Eyebrow } from "@/components/ui/eyebrow";
import { Icon } from "@/components/ui/icon";
import { Bell, Check, ChevronDown, ListChecks, Pause, Search, Sun } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { type Quote, INDICES, fmtPct } from "@/modules/markets/market-data";
import { useMarketData } from "@/modules/markets/hooks/use-market-data";
import { useWatchlists } from "@/modules/markets/hooks/use-watchlists";
import { useQuoteSubscription } from "@/modules/markets/hooks/use-quote-subscription";
import { MarketMap } from "@/modules/markets/ui/market-map";
import { QuoteTable } from "@/modules/markets/ui/quote-table";
import { TickerDetail } from "@/modules/markets/ui/ticker-detail";
import { WatchlistManagerDialog } from "@/modules/markets/ui/watchlist-manager-dialog";
import { useAppStore } from "@/app/state/store";

// The market lenses the treemap can render (besides a specific watchlist).
const MARKET_LENSES = [
  { key: "all", label: "All Symbols" },
  { key: "gainers", label: "Gainers" },
  { key: "losers", label: "Losers" },
  { key: "active", label: "Most Active" },
] as const;
type LensKey = (typeof MARKET_LENSES)[number]["key"];
// A map view is either the active watchlist or a market lens.
type MapView = "watchlist" | LensKey;

function pickDataset(quotes: Quote[], watched: Set<string>, view: MapView): Quote[] {
  switch (view) {
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
      <Eyebrow as="span" className="text-ink-4">{label}</Eyebrow>
      <span className="font-mono text-body text-ink">{value.toLocaleString()}</span>
      <span className={cn("font-mono text-meta", up ? "text-for" : "text-against")}>
        {up ? "▲" : "▼"} {fmtPct(changePct)}
      </span>
    </div>
  );
}

/**
 * The Markets surface — a market map + quote table on the left, a live ticker detail on the right.
 * Watchlists are curated in the manager modal and via the per-row "add to list" star menu; the map
 * dropdown picks which list/lens the treemap shows.
 */
export function MarketsUI() {
  const { quotes, watched } = useMarketData();
  const { lists, activeId, setActive } = useWatchlists();
  const openTicker = useAppStore((s) => s.openTicker);
  const [selected, setSelected] = useState<string>("");
  const [mapView, setMapView] = useState<MapView>("all");
  const [managerOpen, setManagerOpen] = useState(false);

  const activeName = lists.find((l) => l.id === activeId)?.name ?? "Watchlist";
  // Every symbol that lives in ANY list — drives the star fill (membership is per-list via the menu).
  const inAnyList = useMemo(
    () => new Set(lists.flatMap((l) => l.items.map((i) => i.symbol))),
    [lists],
  );
  const mapQuotes = useMemo(() => pickDataset(quotes, watched, mapView), [quotes, watched, mapView]);
  const datasetLabel =
    mapView === "watchlist" ? activeName : MARKET_LENSES.find((d) => d.key === mapView)?.label ?? "All Symbols";

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
        <h1 className="font-display text-lead font-semibold text-ink">Markets</h1>
        <span className="text-body text-ink-4">US Equities &amp; ETFs</span>
        <span className="mx-1 h-4 w-px bg-line" />
        <button
          type="button"
          onClick={() => setManagerOpen(true)}
          className="flex items-center gap-1.5 rounded-card border border-line bg-field px-2.5 py-1.5 text-body text-ink-2 transition-colors hover:border-line-2"
        >
          <Icon as={ListChecks} size="inline" className="text-ink-4" />
          Watchlists
        </button>
        <div className="ml-auto flex items-center gap-2">
          <button
            type="button"
            onClick={() => useAppStore.getState().setCommandPaletteOpen(true)}
            className="flex w-[240px] items-center gap-2 rounded-card border border-line bg-field px-3 py-1.5 text-left text-body text-ink-4 transition-colors hover:border-line-2"
          >
            <Icon as={Search} size="inline" />
            Search symbol…
            <span className="ml-auto font-mono text-meta text-ink-4">/</span>
          </button>
          <button type="button" className="grid size-8 place-items-center rounded-card text-ink-3 hover:text-ink" aria-label="Notifications">
            <Icon as={Bell} />
          </button>
          <button type="button" className="grid size-8 place-items-center rounded-card text-ink-3 hover:text-ink" aria-label="Theme">
            <Icon as={Sun} />
          </button>
        </div>
      </header>

      {/* index strip */}
      <div className="flex shrink-0 items-center border-b border-line bg-chrome/40 pr-4 text-body">
        <div className="flex items-center gap-2 whitespace-nowrap border-r border-line px-4 py-2.5">
          <span className="size-1.5 rounded-full bg-for" />
          <Eyebrow as="span" className="text-ink-3">Mkt Open</Eyebrow>
        </div>
        <div className="scrollbar-hidden flex min-w-0 flex-1 items-center overflow-x-auto overflow-y-hidden py-2.5">
          {INDICES.map((idx) => (
            <IndexPill key={idx.label} {...idx} />
          ))}
        </div>
        <div className="flex items-center gap-2 whitespace-nowrap pl-4">
          <span className="flex items-center gap-1.5 rounded-chip border border-line px-2 py-1 font-mono text-meta text-ink-3">
            <Icon as={Pause} size="inline" mark /> LIVE
          </span>
          <span className="font-mono text-meta text-ink-4">09:26:13 ET</span>
        </div>
      </div>

      {/* body: left column + right detail */}
      <div className="flex min-h-0 flex-1">
        <div className="flex min-w-0 flex-1 flex-col gap-4 overflow-hidden px-5 py-4">
          {/* market map */}
          <div className="shrink-0">
            <div className="mb-3 flex items-center gap-3">
              <h2 className="font-display text-lead font-semibold text-ink">Market Map</h2>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <button
                    type="button"
                    className="flex items-center gap-1.5 rounded-chip border border-line px-2 py-0.5 font-mono text-meta text-ink-2 transition-colors hover:border-line-2"
                  >
                    {datasetLabel} · {mapQuotes.length}
                    <Icon as={ChevronDown} size="inline" mark className="text-ink-4" />
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start" className="w-48">
                  {lists.length > 0 && (
                    <>
                      <DropdownMenuLabel>Watchlists</DropdownMenuLabel>
                      {lists.map((l) => (
                        <DropdownMenuItem
                          key={l.id}
                          onClick={() => {
                            setActive(l.id);
                            setMapView("watchlist");
                          }}
                          className="justify-between"
                        >
                          <span className="truncate">{l.name}</span>
                          {mapView === "watchlist" && l.id === activeId ? (
                            <Icon as={Check} size="inline" mark className="text-amber" />
                          ) : (
                            <span className="font-mono text-meta text-ink-4">{l.itemCount}</span>
                          )}
                        </DropdownMenuItem>
                      ))}
                      <DropdownMenuSeparator />
                    </>
                  )}
                  {MARKET_LENSES.map((d) => (
                    <DropdownMenuItem key={d.key} onClick={() => setMapView(d.key)} className="justify-between">
                      {d.label}
                      {d.key === mapView && <Icon as={Check} size="inline" mark className="text-amber" />}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
              <span className="text-meta text-ink-4">sized by cap · colored by day change</span>
              <span className="ml-auto flex items-center gap-3 font-mono text-meta">
                <span className="flex items-center gap-1 text-ink-4">
                  <span className="size-2 rounded-tap bg-for" /> {advancers} adv
                </span>
                <span className="flex items-center gap-1 text-ink-4">
                  <span className="size-2 rounded-tap bg-against" /> {decliners} dec
                </span>
              </span>
            </div>
            <MarketMap quotes={mapQuotes} selected={selectedQuote?.symbol ?? ""} onSelect={setSelected} />
          </div>

          {/* table */}
          <QuoteTable
            quotes={quotes}
            watched={watched}
            inAnyList={inAnyList}
            watchlistName={activeName}
            selected={selectedQuote?.symbol ?? ""}
            onSelect={setSelected}
          />
        </div>

        {/* right detail */}
        <div className="w-[400px] shrink-0 border-l border-line">
          {selectedQuote && (
            <TickerDetail quote={selectedQuote} onTrade={() => openTicker(selectedQuote.symbol)} />
          )}
        </div>
      </div>

      <WatchlistManagerDialog open={managerOpen} onOpenChange={setManagerOpen} />
    </div>
  );
}
