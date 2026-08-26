/** The capture inbox: bounded, drained-and-cleared, capture-first (works with no board open). */
import { describe, expect, it } from "vitest";
import { create } from "zustand";

import { createWorkspaceSlice, type WorkspaceSlice } from "@/app/state/workspace-slice";

const item = (n: number) => ({
  text: `t${n}`,
  sourceArtifactId: "src",
  sourceTitle: "Source",
  page: n,
});

describe("excerpt queue", () => {
  it("queues, drains and clears", () => {
    const store = create<WorkspaceSlice>()((...a) => createWorkspaceSlice(...a));
    store.getState().queueExcerpt(item(1));
    store.getState().queueExcerpt(item(2));
    expect(store.getState().takeQueuedExcerpts().map((e) => e.page)).toEqual([1, 2]);
    expect(store.getState().queuedExcerpts).toEqual([]);
    expect(store.getState().takeQueuedExcerpts()).toEqual([]);
  });

  it("keeps only the newest 10", () => {
    const store = create<WorkspaceSlice>()((...a) => createWorkspaceSlice(...a));
    for (let i = 1; i <= 14; i++) store.getState().queueExcerpt(item(i));
    const pages = store.getState().takeQueuedExcerpts().map((e) => e.page);
    expect(pages).toEqual([5, 6, 7, 8, 9, 10, 11, 12, 13, 14]);
  });
});
