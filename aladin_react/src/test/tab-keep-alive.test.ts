import { describe, expect, it } from "vitest";

import {
  capHeavyKeys,
  HEAVY_KEEP_ALIVE,
  LIGHT_KEEP_ALIVE,
} from "@/modules/workspace/hooks/use-keep-alive";

// Switching tabs used to unmount the pane — the work pane was a switch on the ACTIVE artifact,
// so leaving a tab threw away its scroll position, editor selection and PDF page. Panes are now
// kept mounted and hidden with CSS, bounded by an LRU window.
//
// The window has to be KIND-AWARE: a note editor holds a Yjs document and a Hocuspocus socket
// and a shard holds a live iframe, so "20 tabs open" must not mean 20 documents syncing in the
// background — while cheap panes have no reason to be evicted at all.
describe("keep-alive window", () => {
  const heavy = new Set(["n1", "n2", "n3", "n4", "n5", "n6", "n7", "n8", "n9", "n10"]);
  const isHeavy = (key: string) => heavy.has(key);

  it("keeps every light pane", () => {
    const keys = ["l1", "l2", "l3", "l4", "l5"];
    expect(capHeavyKeys(keys, isHeavy)).toEqual(keys);
  });

  it("keeps only the most recent heavy panes, and keys arrive most-recent-first", () => {
    const keys = ["n1", "n2", "n3", "n4", "n5", "n6", "n7", "n8", "n9", "n10"];
    expect(capHeavyKeys(keys, isHeavy, 3)).toEqual(["n1", "n2", "n3"]);
  });

  it("does not let heavy panes evict light ones", () => {
    // 4 heavy interleaved with light, cap 2: the light panes all survive.
    const keys = ["n1", "l1", "n2", "l2", "n3", "l3", "n4"];
    expect(capHeavyKeys(keys, isHeavy, 2)).toEqual(["n1", "l1", "n2", "l2", "l3"]);
  });

  it("keeps the active tab, which is always first", () => {
    const keys = ["n9", "n1", "n2", "n3"];
    expect(capHeavyKeys(keys, isHeavy, 1)).toEqual(["n9"]);
  });

  it("caps heavy panes well below the light window, so 20 open tabs stay cheap", () => {
    expect(HEAVY_KEEP_ALIVE).toBeLessThan(LIGHT_KEEP_ALIVE);
    expect(LIGHT_KEEP_ALIVE).toBeGreaterThanOrEqual(20);
  });

  it("is stable when nothing needs evicting", () => {
    const keys = ["n1", "n2"];
    expect(capHeavyKeys(keys, isHeavy)).toEqual(keys);
  });
});
