import { beforeEach, describe, expect, it } from "vitest";
import { initialSessionState } from "@/app/state/session-slice";
import { useAppStore } from "@/app/state/store";
import { initialWorkspaceShellState, tabKey } from "@/modules/workspace/domain";

describe("workspace store", () => {
  beforeEach(() => {
    useAppStore.setState({
      session: initialSessionState,
      workspace: initialWorkspaceShellState,
    });
  });

  it("opens and activates artifacts without duplicating tabs", () => {
    useAppStore.getState().openArtifact("a1");
    useAppStore.getState().openArtifact("a1");

    const workspace = useAppStore.getState().workspace;
    expect(workspace.activeTabKey).toBe("a1");
    expect(workspace.openTabs).toEqual([{ kind: "artifact", artifactId: "a1" }]);
  });

  it("closes the active tab and promotes the most recent remaining tab", () => {
    useAppStore.setState({
      workspace: {
        ...initialWorkspaceShellState,
        activeTabKey: "a2",
        openTabs: [
          { kind: "artifact", artifactId: "a1" },
          { kind: "artifact", artifactId: "a2" },
          { kind: "artifact", artifactId: "a3" },
        ],
      },
    });

    useAppStore.getState().closeTab("a2");

    const workspace = useAppStore.getState().workspace;
    expect(workspace.openTabs.map(tabKey)).toEqual(["a1", "a3"]);
    expect(workspace.activeTabKey).toBe("a3");
  });

  // §11: research views are tabs on the SAME row, keyed by contextId + view — they are
  // not artifacts and must not collide with artifact ids.
  it("opens research views as distinct tabs on one research folder", () => {
    useAppStore.getState().openResearchTab("research-1", "overview");
    useAppStore.getState().openResearchTab("research-1", "runs");
    useAppStore.getState().openResearchTab("research-1", "overview"); // re-activate, no dupe

    const workspace = useAppStore.getState().workspace;
    expect(workspace.openTabs.map(tabKey)).toEqual([
      "research:research-1:overview",
      "research:research-1:runs",
    ]);
    // §12: order is stable — re-activating does NOT move the tab to the end.
    expect(workspace.activeTabKey).toBe("research:research-1:overview");
  });

  it("closes one research view without touching its siblings", () => {
    useAppStore.getState().openResearchTab("research-1", "overview");
    useAppStore.getState().openResearchTab("research-1", "runs");

    useAppStore.getState().closeTab("research:research-1:runs");

    expect(useAppStore.getState().workspace.openTabs.map(tabKey)).toEqual([
      "research:research-1:overview",
    ]);
  });

  it("toggles inline folder expansion", () => {
    useAppStore.getState().toggleFolder("folder-a");
    expect(useAppStore.getState().workspace.expandedFolderIds).toEqual(["folder-a"]);

    useAppStore.getState().toggleFolder("folder-b");
    expect(useAppStore.getState().workspace.expandedFolderIds).toEqual(["folder-a", "folder-b"]);

    useAppStore.getState().toggleFolder("folder-a");
    expect(useAppStore.getState().workspace.expandedFolderIds).toEqual(["folder-b"]);
  });
});
