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

  it("toggles inline folder expansion", () => {
    useAppStore.getState().toggleFolder("folder-a");
    expect(useAppStore.getState().workspace.expandedFolderIds).toEqual(["folder-a"]);

    useAppStore.getState().toggleFolder("folder-b");
    expect(useAppStore.getState().workspace.expandedFolderIds).toEqual(["folder-a", "folder-b"]);

    useAppStore.getState().toggleFolder("folder-a");
    expect(useAppStore.getState().workspace.expandedFolderIds).toEqual(["folder-b"]);
  });
});
