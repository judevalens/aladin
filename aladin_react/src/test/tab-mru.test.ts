import { beforeEach, describe, expect, it } from "vitest";
import { initialSessionState } from "@/app/state/session-slice";
import { useAppStore } from "@/app/state/store";
import {
  initialWorkspaceShellState,
  orderByMru,
  promoteMru,
  tabKey,
} from "@/modules/workspace/domain";

describe("tab MRU", () => {
  beforeEach(() => {
    useAppStore.setState({
      session: initialSessionState,
      workspace: initialWorkspaceShellState,
      tabSwitcherOpen: false,
    });
  });

  it("promotes the opened tab to the head, most recent first", () => {
    const store = useAppStore.getState();
    store.openArtifact("a1");
    store.openArtifact("a2");
    store.openArtifact("a3");

    expect(useAppStore.getState().workspace.tabMru).toEqual(["a3", "a2", "a1"]);
    // The strip's own order is untouched — that is the §12 invariant.
    expect(useAppStore.getState().workspace.openTabs.map(tabKey)).toEqual(["a1", "a2", "a3"]);
  });

  it("promotes on activate without duplicating the key", () => {
    const store = useAppStore.getState();
    store.openArtifact("a1");
    store.openArtifact("a2");
    useAppStore.getState().activateTab("a1");
    useAppStore.getState().activateTab("a1");

    expect(useAppStore.getState().workspace.tabMru).toEqual(["a1", "a2"]);
  });

  it("re-opening an already open tab promotes it rather than appending a second entry", () => {
    const store = useAppStore.getState();
    store.openArtifact("a1");
    store.openArtifact("a2");
    useAppStore.getState().openArtifact("a1");

    expect(useAppStore.getState().workspace.tabMru).toEqual(["a1", "a2"]);
    expect(useAppStore.getState().workspace.openTabs.map(tabKey)).toEqual(["a1", "a2"]);
  });

  it("removes a closed tab from the MRU list", () => {
    const store = useAppStore.getState();
    store.openArtifact("a1");
    store.openArtifact("a2");
    store.openArtifact("a3");
    useAppStore.getState().closeTab("a2");

    expect(useAppStore.getState().workspace.tabMru).toEqual(["a3", "a1"]);
  });

  it("tracks research view tabs under their own key", () => {
    const store = useAppStore.getState();
    store.openResearchTab("r1", "overview");
    store.openResearchTab("r1", "runs");

    expect(useAppStore.getState().workspace.tabMru).toEqual([
      "research:r1:runs",
      "research:r1:overview",
    ]);
  });

  describe("promoteMru", () => {
    it("is idempotent and accepts a key it has never seen", () => {
      expect(promoteMru(["a", "b"], "a")).toEqual(["a", "b"]);
      expect(promoteMru(["a", "b"], "c")).toEqual(["c", "a", "b"]);
      expect(promoteMru([], "a")).toEqual(["a"]);
    });
  });

  describe("orderByMru", () => {
    const items = [{ key: "a" }, { key: "b" }, { key: "c" }];

    it("orders by the MRU list", () => {
      expect(orderByMru(items, ["c", "a", "b"]).map((i) => i.key)).toEqual(["c", "a", "b"]);
    });

    it("drops MRU keys with no open tab, so a stale entry can never render a row", () => {
      expect(orderByMru(items, ["zz", "b", "a"]).map((i) => i.key)).toEqual(["b", "a", "c"]);
    });

    it("appends open tabs missing from the MRU list, so it can never hide a tab", () => {
      expect(orderByMru(items, ["c"]).map((i) => i.key)).toEqual(["c", "a", "b"]);
      expect(orderByMru(items, []).map((i) => i.key)).toEqual(["a", "b", "c"]);
    });
  });

  it("index 1 is the previously-active tab — what makes a repeated Ctrl+Tab a toggle", () => {
    const store = useAppStore.getState();
    store.openArtifact("a1");
    store.openArtifact("a2");

    const mru = useAppStore.getState().workspace.tabMru;
    expect(mru[0]).toBe(useAppStore.getState().workspace.activeTabKey);
    expect(mru[1]).toBe("a1");
  });
});
