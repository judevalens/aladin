import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { CandlestickChart, Plus, Search, X } from "lucide-react";

import { useAppComposition } from "@/app/composition/app-composition";
import { useWatchlist } from "@/modules/markets/hooks/use-watchlist";
import type { InstrumentHit } from "@/shared/api/models";
import { cn } from "@/lib/utils";

// Search-to-add box: debounced ticker typeahead (reuses the instruments repo). Picking a
// hit adds it to the watchlist and clears the query.
function AddTicker({ onAdd }: { onAdd: (instrumentId: string) => void }) {
  const { repos } = useAppComposition();
  const [query, setQuery] = useState("");
  const [hits, setHits] = useState<InstrumentHit[]>([]);
  const [open, setOpen] = useState(false);
  const boxRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const q = query.trim();
    if (!q) {
      setHits([]);
      return;
    }
    let cancelled = false;
    const handle = setTimeout(() => {
      repos.instruments
        .search(q, 8)
        .then((res) => {
          if (!cancelled) {
            setHits(res);
            setOpen(true);
          }
        })
        .catch(() => {
          if (!cancelled) setHits([]);
        });
    }, 160);
    return () => {
      cancelled = true;
      clearTimeout(handle);
    };
  }, [query, repos.instruments]);

  // Close the dropdown on outside click.
  useEffect(() => {
    const onClick = (e: MouseEvent) => {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, []);

  const pick = (hit: InstrumentHit) => {
    onAdd(hit.id);
    setQuery("");
    setHits([]);
    setOpen(false);
  };

  return (
    <div ref={boxRef} className="relative w-full max-w-[420px]">
      <div className="flex items-center gap-2 rounded-card border border-line bg-field px-3 py-2">
        <Search className="size-4 shrink-0 text-ink-4" strokeWidth={1.75} />
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onFocus={() => hits.length > 0 && setOpen(true)}
          placeholder="Add a ticker to your watchlist…"
          className="w-full bg-transparent text-sm text-ink outline-none placeholder:text-ink-4"
        />
      </div>
      {open && hits.length > 0 && (
        <div className="absolute z-20 mt-1 w-full overflow-hidden rounded-card border border-line bg-explorer shadow-modal">
          {hits.map((h) => (
            <button
              key={h.id}
              type="button"
              onClick={() => pick(h)}
              className="flex w-full items-center gap-2 px-3 py-2 text-left transition-colors hover:bg-card"
            >
              <Plus className="size-[14px] shrink-0 text-ink-4" strokeWidth={1.9} />
              <span className="font-mono text-sm text-ink">{h.symbol}</span>
              <span className="truncate text-[12px] text-ink-3">{h.name}</span>
              <span className="ml-auto shrink-0 font-mono text-[10px] text-ink-4">{h.exchange}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

/**
 * The Markets surface — the first trading-platform screen. A watchlist of tickers you're
 * tracking: search-to-add, click through to the ticker detail, remove. Quotes and
 * sparklines land here once the bar store (T1) exists.
 */
export function MarketsUI() {
  const navigate = useNavigate();
  const { items, loading, error, add, remove } = useWatchlist();

  return (
    <div className="flex h-full flex-col overflow-y-auto bg-bg">
      <header className="flex flex-col gap-4 border-b border-line px-6 py-5">
        <div className="flex items-baseline gap-3">
          <h1 className="font-display text-xl font-semibold text-ink">Markets</h1>
          <span className="text-sm text-ink-3">Your watchlist</span>
          <span className="ml-auto font-mono text-[11px] text-ink-4">{items.length} tracked</span>
        </div>
        <AddTicker onAdd={add} />
      </header>

      <div className="flex-1 px-6 py-4">
        {error && <p className="text-[13px] text-against">{error}</p>}
        {!error && loading && <p className="text-[13px] text-ink-3">Loading…</p>}
        {!error && !loading && items.length === 0 && (
          <div className="flex flex-col items-center gap-3 py-16 text-center text-ink-4">
            <CandlestickChart className="size-8" strokeWidth={1.25} />
            <p className="max-w-sm text-sm text-ink-3">
              Nothing tracked yet. Search a ticker above to add it — or hit ⌘K and search
              from anywhere.
            </p>
          </div>
        )}
        {!error && !loading && items.length > 0 && (
          <div className="flex flex-col">
            {items.map((it) => (
              <div
                key={it.instrumentId}
                className={cn(
                  "group flex items-center gap-3 border-b border-line-2 px-2 py-3",
                  "transition-colors hover:bg-card",
                )}
              >
                <button
                  type="button"
                  onClick={() => navigate(`/ticker/${encodeURIComponent(it.symbol)}`)}
                  className="flex flex-1 items-center gap-3 text-left"
                >
                  <span className="w-[72px] shrink-0 font-mono text-sm font-semibold text-ink">
                    {it.symbol}
                  </span>
                  <span className="truncate text-[13px] text-ink-2">{it.name}</span>
                  <span className="ml-auto shrink-0 font-mono text-[10px] text-ink-4">
                    {it.exchange}
                  </span>
                </button>
                <button
                  type="button"
                  onClick={() => remove(it.instrumentId)}
                  aria-label={`Remove ${it.symbol} from watchlist`}
                  className="shrink-0 rounded p-1 text-ink-4 opacity-0 transition-opacity hover:text-ink-2 group-hover:opacity-100"
                >
                  <X className="size-4" strokeWidth={1.9} />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
