import { useMemo } from "react";

import { cn } from "@/lib/utils";
import { type Quote, fmtPct } from "@/modules/markets/market-data";
import { type Placed, squarify } from "@/modules/markets/treemap";

const SECTOR_ORDER = ["Technology", "Communication", "Consumer", "Financials", "Health", "Energy", "ETF"];

// Virtual layout space (≈2.2:1). Tiles are positioned as percentages of it, so the map is
// responsive without measuring the DOM.
const VW = 1000;
const VH = 460;

interface TileNode {
  quote: Quote;
  sector: string;
}

/**
 * The market map — a nested squarified treemap. Sectors pack into the map by total cap;
 * tiles pack into each sector by cap and are colored by day change. Small caps become
 * readable rectangles (2D packing) instead of full-height slivers.
 */
export function MarketMap({
  quotes,
  selected,
  onSelect,
}: {
  quotes: Quote[];
  selected: string;
  onSelect: (symbol: string) => void;
}) {
  const { tiles, sectors } = useMemo(() => {
    const bySector = new Map<string, Quote[]>();
    for (const q of quotes) {
      const arr = bySector.get(q.sector) ?? [];
      arr.push(q);
      bySector.set(q.sector, arr);
    }
    const sectorList = [...bySector.entries()]
      .map(([name, items]) => ({ name, items, cap: items.reduce((s, q) => s + q.marketCap, 0) }))
      .sort((a, b) => (SECTOR_ORDER.indexOf(a.name) + 1 || 99) - (SECTOR_ORDER.indexOf(b.name) + 1 || 99));

    // 1) sectors into the map, 2) tiles into each sector rect.
    const sectorRects = squarify(
      sectorList.map((s) => ({ value: s.cap, data: s })),
      { x: 0, y: 0, w: VW, h: VH },
    );
    const tiles: Placed<TileNode>[] = [];
    for (const sr of sectorRects) {
      const inner = squarify(
        sr.data.items
          .sort((a, b) => b.marketCap - a.marketCap)
          .map((q) => ({ value: q.marketCap, data: { quote: q, sector: sr.data.name } })),
        sr,
      );
      tiles.push(...inner);
    }
    return { tiles, sectors: sectorRects };
  }, [quotes]);

  return (
    // aspect-ratio drives height from width, but CAP it so a wide screen can't grow the map tall
    // enough to push the quote table out of the (overflow-hidden) column. Below the cap the ratio
    // holds (undistorted); above it the map fills width at a bounded height (finviz-style).
    <div className="relative w-full" style={{ aspectRatio: `${VW} / ${VH}`, maxHeight: "min(44vh, 440px)" }}>
      {/* sector labels, anchored to each sector rectangle */}
      {sectors.map((s) => (
        <span
          key={s.data.name}
          className="pointer-events-none absolute z-10 truncate font-mono text-[9px] uppercase tracking-[0.5px] text-ink/60"
          style={{ left: `${(s.x / VW) * 100}%`, top: `${(s.y / VH) * 100}%`, maxWidth: `${(s.w / VW) * 100}%`, padding: "3px 5px" }}
        >
          {s.data.name}
        </span>
      ))}

      {tiles.map((t) => {
        const q = t.data.quote;
        const up = q.change >= 0;
        const intensity = 0.22 + Math.min(0.62, Math.abs(q.changePct) / 7);
        const isSel = q.symbol === selected;
        const showSym = t.w >= 26 && t.h >= 22;
        const showPct = t.w >= 42 && t.h >= 40;
        return (
          <button
            key={q.symbol}
            type="button"
            onClick={() => onSelect(q.symbol)}
            title={`${q.symbol} ${fmtPct(q.changePct)}`}
            className={cn(
              "absolute overflow-hidden rounded-[5px] transition-all",
              isSel && "z-20 ring-2 ring-inset ring-ink",
            )}
            style={{
              left: `${(t.x / VW) * 100}%`,
              top: `${(t.y / VH) * 100}%`,
              width: `calc(${(t.w / VW) * 100}% - 3px)`,
              height: `calc(${(t.h / VH) * 100}% - 3px)`,
            }}
          >
            <span aria-hidden className={cn("absolute inset-0", up ? "bg-for" : "bg-against")} style={{ opacity: intensity }} />
            <span className="relative flex h-full w-full flex-col items-center justify-center overflow-hidden px-0.5 text-center leading-tight">
              {showSym && <span className="max-w-full truncate font-display text-[12px] font-semibold text-ink">{q.symbol}</span>}
              {showPct && <span className="max-w-full truncate font-mono text-[9px] text-ink-2">{fmtPct(q.changePct)}</span>}
            </span>
          </button>
        );
      })}
    </div>
  );
}
