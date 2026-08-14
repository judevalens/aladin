import { useEffect, useState } from "react";

/**
 * The most-recently-active keys, LRU-ordered, capped.
 *
 * This is what makes a tab survive being switched away from. It started life private to
 * `doc-surface-ui.tsx`, where only shard iframes got the treatment; every other pane was
 * unmounted on tab switch and lost its scroll position, its editor selection, its PDF page.
 *
 * The active key is always included by the caller even before the effect lands, otherwise the
 * frame in which `activeKey` changes renders nothing at all.
 */
export function useKeepAliveKeys(activeKey: string | null, cap: number): string[] {
  const [keys, setKeys] = useState<string[]>(() => (activeKey ? [activeKey] : []));
  useEffect(() => {
    if (!activeKey) return;
    setKeys((prev) => {
      if (prev[0] === activeKey) return prev;
      return [activeKey, ...prev.filter((key) => key !== activeKey)].slice(0, cap);
    });
  }, [activeKey, cap]);
  return keys;
}

/**
 * A pane that is expensive to hold open. A note editor keeps a Yjs document and a Hocuspocus
 * websocket; a shard keeps a live iframe. Twenty of either is not free, and "I want 20 tabs"
 * means twenty tabs open — not twenty documents synchronising in the background.
 */
export const HEAVY_KEEP_ALIVE = 8;

/**
 * Everything else — link, voice and file cards, research views. These are ordinary React trees
 * reading data that is already in the local replica, so the cap exists only as a backstop.
 */
export const LIGHT_KEEP_ALIVE = 24;

/**
 * Trims an LRU key list so at most `heavyCap` heavy panes stay mounted, keeping the most recent
 * ones. Light panes are untouched.
 *
 * Applying one flat cap to both would mean either evicting cheap panes for no reason or holding
 * twenty websockets open. Keys are assumed to arrive most-recent-first.
 */
export function capHeavyKeys(
  keys: string[],
  isHeavy: (key: string) => boolean,
  heavyCap = HEAVY_KEEP_ALIVE,
): string[] {
  let heavySeen = 0;
  return keys.filter((key) => {
    if (!isHeavy(key)) return true;
    heavySeen += 1;
    return heavySeen <= heavyCap;
  });
}
