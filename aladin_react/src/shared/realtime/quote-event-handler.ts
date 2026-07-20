import type { AppEventEnvelope } from "@/shared/realtime/app-event";
import { useAppStore } from "@/app/state/store";
import type { LiveQuote } from "@/app/state/market-slice";

// Routes "quote.update" broadcast events (the market stream) into the live-quote store.
// Ephemeral UI state — never touches the offline data layer.
export function createQuoteEventHandler() {
  return function handle(event: AppEventEnvelope) {
    if (event.type !== "quote.update") return;
    const q = event.payload as Partial<LiveQuote> | null;
    if (!q?.symbol || typeof q.last !== "number") return;
    useAppStore.getState().applyQuote({
      symbol: q.symbol,
      instrumentId: q.instrumentId,
      last: q.last,
      change: q.change,
      changePct: q.changePct,
      prevClose: q.prevClose,
      ts: q.ts,
    });
  };
}
