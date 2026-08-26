/**
 * The wormhole's store half: `openArtifactAt` opens the tab AND parks a nonce-keyed page
 * the reader watches. Nonces make the same page re-fire; entries persist while the tab is
 * open, but CLOSING the tab drops its entry — "continue" is now the synced reading
 * position, and a stale cite must not outrank it on reopen.
 */
import { describe, expect, it } from "vitest";
import { create } from "zustand";

import { createWorkspaceSlice, type WorkspaceSlice } from "@/app/state/workspace-slice";

function makeStore() {
  return create<WorkspaceSlice>()((...args) => createWorkspaceSlice(...args));
}

describe("openArtifactAt", () => {
  it("opens the tab and parks the page", () => {
    const store = makeStore();
    store.getState().openArtifactAt("doc-1", 94);
    const s = store.getState();
    expect(s.workspace.openTabs).toEqual([{ kind: "artifact", artifactId: "doc-1" }]);
    expect(s.workspace.activeTabKey).toBe("doc-1");
    expect(s.pendingDocLocations["doc-1"]).toEqual({ page: 94, nonce: 1 });
  });

  it("re-citing the same page bumps the nonce so the reader re-scrolls", () => {
    const store = makeStore();
    store.getState().openArtifactAt("doc-1", 94);
    store.getState().openArtifactAt("doc-1", 94);
    expect(store.getState().pendingDocLocations["doc-1"]).toEqual({ page: 94, nonce: 2 });
  });

  it("does not duplicate an open tab, and locations are per-artifact", () => {
    const store = makeStore();
    store.getState().openArtifact("doc-1");
    store.getState().openArtifactAt("doc-1", 12);
    store.getState().openArtifactAt("doc-2", 3);
    const s = store.getState();
    expect(s.workspace.openTabs.map((t) => ("artifactId" in t ? t.artifactId : ""))).toEqual([
      "doc-1",
      "doc-2",
    ]);
    expect(s.pendingDocLocations).toEqual({
      "doc-1": { page: 12, nonce: 1 },
      "doc-2": { page: 3, nonce: 1 },
    });
  });

  it("closing the tab drops its parked location (synced position wins on reopen)", () => {
    const store = makeStore();
    store.getState().openArtifactAt("doc-1", 94);
    store.getState().openArtifactAt("doc-2", 3);
    store.getState().closeTab("doc-1");
    expect(store.getState().pendingDocLocations).toEqual({
      "doc-2": { page: 3, nonce: 1 },
    });
  });
});
