import { beforeEach, describe, expect, it } from "vitest";
import { initialSessionState } from "@/app/state/session-slice";
import { useAppStore } from "@/app/state/store";
import { createDefaultPageSessionMeta } from "@/modules/pages/domain";
import { initialWorkspaceShellState } from "@/modules/workspace/domain";

describe("workspace store", () => {
  beforeEach(() => {
    useAppStore.setState({
      session: initialSessionState,
      workspace: initialWorkspaceShellState,
      pageSessions: {},
      sourcesUi: {
        addOpen: false,
        integrationsOpen: false,
        selectedSourceId: null,
      },
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

  it("creates and patches page session metadata without storing editor content", () => {
    useAppStore.getState().ensurePageSession("page-1");
    useAppStore.getState().setPageSaveState("page-1", "saving", "Saving…");
    useAppStore.getState().setPageRevision("page-1", 7);

    const session = useAppStore.getState().pageSessions["page-1"] ?? createDefaultPageSessionMeta();
    expect(session.saveState).toBe("saving");
    expect(session.message).toBe("Saving…");
    expect(session.revision).toBe(7);
  });
});
