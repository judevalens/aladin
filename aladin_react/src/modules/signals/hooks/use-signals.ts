import { useEffect, useMemo, useState, useSyncExternalStore } from "react";

import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";
import type { ClaimSignal, SignalLens, SignalSort } from "@/modules/signals/signal-types";

export interface UseSignals {
  signals: ClaimSignal[];
  loading: boolean;
  error: string | null;
  lens: SignalLens;
  setLens: (lens: SignalLens) => void;
  sort: SignalSort;
  setSort: (sort: SignalSort) => void;
}

// Two lenses over one surface. "inbox" reads the LOCAL signals cache and re-ranks live as sync
// frames apply (true push, via useSyncExternalStore). "book" one-shot fetches your authored
// theses over HTTP, each marked to market by supporting/contradicting discovered sources.
export function useSignals(): UseSignals {
  const { repos } = useAppComposition();
  const [lens, setLens] = useState<SignalLens>("inbox");
  const [sort, setSort] = useState<SignalSort>("recent");
  const clearSignalsUnread = useAppStore((s) => s.clearSignalsUnread);

  // inbox: reactive local store
  const store = useMemo(() => repos.localSignals.observe(sort), [repos.localSignals, sort]);
  const inbox = useSyncExternalStore(store.subscribe, store.snapshot);

  // book: one-shot HTTP fetch (mirrors use-signal-detail); undefined signals = loading
  const [book, setBook] = useState<{ signals?: ClaimSignal[]; error: string | null }>({ error: null });
  useEffect(() => {
    if (lens !== "book") return;
    let cancelled = false;
    setBook({ error: null });
    repos.bookSignals
      .list(sort)
      .then((signals) => {
        if (!cancelled) setBook({ signals, error: null });
      })
      .catch((e: unknown) => {
        if (!cancelled) setBook({ signals: [], error: e instanceof Error ? e.message : String(e) });
      });
    return () => {
      cancelled = true;
    };
  }, [lens, sort, repos.bookSignals]);

  const inboxLoading = inbox === undefined;
  useEffect(() => {
    if (lens === "inbox" && !inboxLoading) clearSignalsUnread();
  }, [lens, inboxLoading, inbox, clearSignalsUnread]);

  if (lens === "book") {
    return {
      signals: book.signals ?? [],
      loading: book.signals === undefined && book.error === null,
      error: book.error,
      lens,
      setLens,
      sort,
      setSort,
    };
  }

  return { signals: inbox ?? [], loading: inboxLoading, error: null, lens, setLens, sort, setSort };
}
