import { useCallback, useEffect, useState } from "react";

import { useAppComposition } from "@/app/composition/app-composition";
import type { MergeQueueItem } from "@/modules/entities/entity-list-types";

export interface UseMergeQueue {
  items: MergeQueueItem[];
  loading: boolean;
  error: string | null;
  /** Decide a proposal; the card leaves the queue immediately (optimistic, no refetch). */
  accept: (item: MergeQueueItem) => Promise<void>;
  reject: (item: MergeQueueItem) => Promise<void>;
}

// The Entities inbox: the judge's pending merge decisions. Deciding one removes it from
// the list in place — no refetch (consistent with the index's no-refetch stance); a
// failure restores the card so nothing is silently lost.
export function useMergeQueue(): UseMergeQueue {
  const { repos } = useAppComposition();
  const [items, setItems] = useState<MergeQueueItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    repos.graphPane
      .mergeQueue()
      .then((result) => {
        if (!cancelled) setItems(result);
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "Failed to load the inbox");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [repos]);

  const decide = useCallback(
    async (item: MergeQueueItem, run: (fromId: string, mergeId: string) => Promise<void>) => {
      setItems((prev) => prev.filter((i) => i.mergeId !== item.mergeId));
      try {
        await run(item.fromId, item.mergeId);
      } catch (err) {
        setItems((prev) => [item, ...prev]); // restore on failure
        throw err;
      }
    },
    [],
  );

  const accept = useCallback(
    (item: MergeQueueItem) => decide(item, repos.graphPane.acceptEntityMerge),
    [decide, repos],
  );
  const reject = useCallback(
    (item: MergeQueueItem) => decide(item, repos.graphPane.rejectEntityMerge),
    [decide, repos],
  );

  return { items, loading, error, accept, reject };
}
