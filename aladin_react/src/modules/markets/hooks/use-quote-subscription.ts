import { useEffect, useRef } from "react";

import { useAppComposition } from "@/app/composition/app-composition";

// Registers live-quote demand for a set of symbols: subscribes on mount / when the set
// changes, unsubscribes on cleanup. Diff-based so re-renders don't thrash the hub. The
// backend refcounts, so overlapping subscribers across components are fine.
export function useQuoteSubscription(symbols: string[]) {
  const { repos } = useAppComposition();
  const prevRef = useRef<string[]>([]);
  // Stable key so the effect only fires when the actual symbol set changes.
  const key = [...new Set(symbols.map((s) => s.toUpperCase()))].sort().join(",");

  useEffect(() => {
    const next = key ? key.split(",") : [];
    const prev = prevRef.current;
    const added = next.filter((s) => !prev.includes(s));
    const removed = prev.filter((s) => !next.includes(s));
    prevRef.current = next;
    if (added.length) void repos.market.subscribe(added);
    if (removed.length) void repos.market.unsubscribe(removed);
  }, [key, repos.market]);

  // Release everything on unmount.
  useEffect(() => {
    return () => {
      if (prevRef.current.length) void repos.market.unsubscribe(prevRef.current);
    };
  }, [repos.market]);
}
