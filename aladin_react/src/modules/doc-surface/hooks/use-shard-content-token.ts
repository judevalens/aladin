import { useCallback, useEffect, useState } from "react";
import type { ContentTokenStore } from "@/shared/runtime/content-token-store";

// A token belongs to a document load. A build/page change must re-check freshness,
// while ordinary rerenders and tab visibility changes leave the live frame alone.
export function useShardContentToken(store: ContentTokenStore | null, loadKey: string) {
  const [attempt, setAttempt] = useState(0);
  const [result, setResult] = useState<{
    store: ContentTokenStore;
    loadKey: string;
    attempt: number;
    token: string | null;
  } | null>(null);

  useEffect(() => {
    if (!store) return;
    let alive = true;
    void store.get().catch(() => null).then((token) => {
      if (alive) setResult({ store, loadKey, attempt, token });
    });
    return () => { alive = false; };
  }, [store, loadKey, attempt]);

  const settled = result?.store === store && result?.loadKey === loadKey && result?.attempt === attempt;
  const token = settled ? result.token : null;
  const retry = useCallback(() => setAttempt((value) => value + 1), []);
  return { token, error: Boolean(store && settled && !token), retry };
}
