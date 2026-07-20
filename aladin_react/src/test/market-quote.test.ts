import { beforeEach, describe, expect, it } from "vitest";
import { useAppStore } from "@/app/state/store";
import { createQuoteEventHandler } from "@/shared/realtime/quote-event-handler";
import type { AppEventEnvelope } from "@/shared/realtime/app-event";

function quoteEvent(type: string, payload: unknown): AppEventEnvelope {
  return {
    eventId: `evt-${Math.random()}`,
    type,
    subscriptionKey: { stream: "market", resourceKind: "quote", resourceId: "inst-nvda" },
    payload,
    occurredAt: "2026-07-19T00:00:00Z",
  };
}

describe("quote event handler → market slice", () => {
  beforeEach(() => {
    useAppStore.setState({ liveQuotes: {} });
  });

  it("routes a quote.update into liveQuotes, keyed by uppercase symbol", () => {
    const handle = createQuoteEventHandler();
    handle(quoteEvent("quote.update", { symbol: "nvda", instrumentId: "inst-nvda", last: 1200.5, change: 17, changePct: 1.4 }));
    const q = useAppStore.getState().liveQuotes["NVDA"];
    expect(q?.last).toBe(1200.5);
    expect(q?.changePct).toBe(1.4);
  });

  it("ignores non-quote events and malformed payloads", () => {
    const handle = createQuoteEventHandler();
    handle(quoteEvent("artifact.updated", { symbol: "AAPL", last: 227 }));
    handle(quoteEvent("quote.update", { symbol: "AAPL" })); // no last
    handle(quoteEvent("quote.update", { last: 227 })); // no symbol
    expect(Object.keys(useAppStore.getState().liveQuotes)).toHaveLength(0);
  });
});
