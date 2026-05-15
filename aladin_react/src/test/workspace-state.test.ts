import { describe, expect, it } from "vitest";
import {
  initialWorkspaceState,
  workspaceReducer,
} from "@/features/workspace/workspace-state";

describe("workspaceReducer", () => {
  it("opens and activates artifacts without duplicating tabs", () => {
    const opened = workspaceReducer(initialWorkspaceState, {
      type: "open-artifact",
      artifactId: "a1",
    });
    const reopened = workspaceReducer(opened, {
      type: "open-artifact",
      artifactId: "a1",
    });

    expect(opened.activeArtifactId).toBe("a1");
    expect(opened.openArtifactIds).toEqual(["a1"]);
    expect(reopened.openArtifactIds).toEqual(["a1"]);
  });

  it("closes the active artifact and promotes the most recent remaining tab", () => {
    const state = {
      ...initialWorkspaceState,
      activeArtifactId: "a2",
      openArtifactIds: ["a1", "a2", "a3"],
    };
    const next = workspaceReducer(state, {
      type: "close-artifact",
      artifactId: "a2",
    });

    expect(next.openArtifactIds).toEqual(["a1", "a3"]);
    expect(next.activeArtifactId).toBe("a3");
  });
});
