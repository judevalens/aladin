import { act, renderHook, waitFor } from "@testing-library/react";
import { of } from "rxjs";
import { beforeEach, describe, expect, it } from "vitest";

import { AppCompositionContext } from "@/app/composition/app-composition";
import type { AppComposition } from "@/app/composition/create-app-composition";
import { initialSessionState } from "@/app/state/session-slice";
import { useAppStore } from "@/app/state/store";
import { initialWorkspaceShellState } from "@/modules/workspace/domain";
import { useWorkPane } from "@/modules/workspace/hooks/use-workspace-state";
import type { Artifact } from "@/shared/api/models";
import { KeyedStream } from "@/shared/flow/keyed-stream";

// Regression for "deleting one open document reloads the others".
//
// The work pane builds ONE artifact-cache stream over the ids of the open tabs, so the stream
// is rebuilt whenever a tab opens or closes — and every per-id stream in it is seeded with
// `startWith(undefined)`. That made the first frame after a close report every open artifact
// as missing, which drops each pane into TabPane's placeholder: a PDF's pdf.js document is
// destroyed and re-fetched, a note's Yjs socket reconnects, a shard's iframe is rebuilt.
//
// The assertion is that closing one tab never blanks another tab's artifact.

const FILE = (id: string): Artifact =>
  ({ id, kind: "file", title: id, folderId: "f1" }) as unknown as Artifact;

// The real read model, over a fake repo: the point of the test is that the KEYED STREAM holds
// the current value, so a rebuilt cache stream re-reads it instead of going blank. Faking
// `artifactById` with a hand-rolled observable would test the fake instead.
const artifacts = new KeyedStream<string, Artifact>(
  (artifact) => artifact.id,
  // Asynchronous, like the query against the local replica it stands in for.
  async (id) => await Promise.resolve(FILE(id)),
);
const artifactById = (artifactId: string) => artifacts.observe(artifactId);

// One instance, not a fresh observable per call: the hook feeds this straight into
// useObservableState, which rebuilds its store whenever the observable's identity changes —
// a new one each render would resubscribe, emit, and render forever.
const TREE = of({ ok: true, value: [] });

const composition = {
  services: {
    workspace: {
      artifactById,
      tree: () => TREE,
    },
  },
} as unknown as AppComposition;

function wrapper({ children }: { children: React.ReactNode }) {
  return (
    <AppCompositionContext.Provider value={composition}>{children}</AppCompositionContext.Provider>
  );
}

describe("work pane artifact cache", () => {
  beforeEach(() => {
    useAppStore.setState({
      session: initialSessionState,
      workspace: {
        ...initialWorkspaceShellState,
        activeTabKey: "a1",
        openTabs: [
          { kind: "artifact", artifactId: "a1" },
          { kind: "artifact", artifactId: "a2" },
        ],
      },
    });
  });

  it("keeps an open tab's artifact when another tab is closed", async () => {
    const { result } = renderHook(() => useWorkPane(), { wrapper });

    await waitFor(() => {
      expect(result.current.tabs.map((t) => t.artifact?.id)).toEqual(["a1", "a2"]);
    });

    // The delete path: the artifact is removed, then its tab is closed.
    act(() => {
      useAppStore.getState().closeTab("a2");
    });

    // Synchronously after the close — the frame the placeholder used to render in.
    expect(result.current.tabs.map((t) => t.artifact?.id)).toEqual(["a1"]);
    expect(result.current.activeArtifact?.id).toBe("a1");
  });
});
