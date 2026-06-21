import { useEffect, useMemo, useState, useSyncExternalStore } from "react";

import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";
import type { ClaimSignal, SignalSort } from "@/modules/signals/signal-types";

export interface UseSignals {
  signals: ClaimSignal[];
  loading: boolean;
  error: string | null;
  sort: SignalSort;
  setSort: (sort: SignalSort) => void;
}

// Reads the claim feed from the LOCAL signals cache and re-ranks live as sync frames apply
// (true push, no HTTP fetch). Binds the per-sort external store via useSyncExternalStore; clears
// the rail unread dot while the surface is mounted.
export function useSignals(): UseSignals {
  const { repos } = useAppComposition();
  const [sort, setSort] = useState<SignalSort>("recent");
  const clearSignalsUnread = useAppStore((s) => s.clearSignalsUnread);

  const store = useMemo(() => repos.localSignals.observe(sort), [repos.localSignals, sort]);
  const snapshot = useSyncExternalStore(store.subscribe, store.snapshot);
  const loading = snapshot === undefined;

  useEffect(() => {
    if (!loading) clearSignalsUnread();
  }, [loading, snapshot, clearSignalsUnread]);

  return { signals: snapshot ?? [], loading, error: null, sort, setSort };
}
