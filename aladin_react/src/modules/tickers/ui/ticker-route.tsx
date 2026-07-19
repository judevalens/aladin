import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { CandlestickChart } from "lucide-react";
import { useAppComposition } from "@/app/composition/app-composition";
import type { InstrumentHit } from "@/shared/api/models";

/**
 * Ticker detail surface — the landing for a security selected from the command box.
 * For now it resolves the instrument by symbol (via search) and shows identity; the
 * chart, quote, and the entity/research panel arrive with bars (T1) and the entity
 * bridge. Deliberately thin so the navigation target is real end to end.
 */
export function TickerRoute() {
  const { symbol = "" } = useParams();
  const { repos } = useAppComposition();
  const [instrument, setInstrument] = useState<InstrumentHit | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    repos.instruments
      .search(symbol)
      .then((hits) => {
        if (cancelled) return;
        const exact = hits.find((h) => h.symbol.toLowerCase() === symbol.toLowerCase());
        setInstrument(exact ?? hits[0] ?? null);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [symbol, repos.instruments]);

  return (
    <div className="flex h-full flex-col overflow-y-auto bg-bg">
      <header className="flex items-baseline gap-3 border-b border-line px-6 py-5">
        <h1 className="font-mono text-2xl font-semibold text-ink">{symbol.toUpperCase()}</h1>
        <span className="text-sm text-ink-3">
          {loading ? "Resolving…" : (instrument?.name ?? "Unknown symbol")}
        </span>
        {instrument?.exchange && (
          <span className="ml-auto rounded-chip border border-line px-2 py-0.5 font-mono text-[11px] text-ink-4">
            {instrument.exchange}
          </span>
        )}
      </header>

      <div className="flex flex-1 items-center justify-center p-6">
        <div className="flex flex-col items-center gap-3 text-center text-ink-4">
          <CandlestickChart className="size-8" strokeWidth={1.25} />
          <p className="max-w-sm text-sm text-ink-3">
            Price chart lands with the bar store (T1). This ticker resolves from the
            instruments registry; bars, quote, and the research panel come next.
          </p>
        </div>
      </div>
    </div>
  );
}
