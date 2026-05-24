import { beforeEach, describe, expect, it } from "vitest";
import { initialSessionState } from "@/app/state/session-slice";
import { useAppStore } from "@/app/state/store";
import { initialWorkspaceShellState } from "@/modules/workspace/domain";

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
    expect(workspace.activeArtifactId).toBe("a1");
    expect(workspace.openArtifactIds).toEqual(["a1"]);
  });

  it("closes the active artifact and promotes the most recent remaining tab", () => {
    useAppStore.setState({
      workspace: {
        ...initialWorkspaceShellState,
        activeArtifactId: "a2",
        openArtifactIds: ["a1", "a2", "a3"],
      },
    });

    useAppStore.getState().closeArtifact("a2");

    const workspace = useAppStore.getState().workspace;
    expect(workspace.openArtifactIds).toEqual(["a1", "a3"]);
    expect(workspace.activeArtifactId).toBe("a3");
  });

  it("drills into a folder and restores the previous browser frame on pop", () => {
    useAppStore.setState({
      workspace: {
        ...initialWorkspaceShellState,
        browserRootFolderId: "folder-root",
        browserScrollTop: 128,
        expandedFolderIds: ["folder-a", "folder-b"],
      },
    });

    useAppStore.getState().drillIntoFolder("folder-deep");
    let workspace = useAppStore.getState().workspace;
    expect(workspace.browserRootFolderId).toBe("folder-deep");
    expect(workspace.expandedFolderIds).toEqual([]);
    expect(workspace.browserScrollTop).toBe(0);
    expect(workspace.browserFrameStack).toEqual([
      {
        rootFolderId: "folder-root",
        expandedFolderIds: ["folder-a", "folder-b"],
        scrollTop: 128,
      },
    ]);

    useAppStore.getState().popBrowserFrame();
    workspace = useAppStore.getState().workspace;
    expect(workspace.browserRootFolderId).toBe("folder-root");
    expect(workspace.expandedFolderIds).toEqual(["folder-a", "folder-b"]);
    expect(workspace.browserScrollTop).toBe(128);
    expect(workspace.browserFrameStack).toEqual([]);
  });
});
