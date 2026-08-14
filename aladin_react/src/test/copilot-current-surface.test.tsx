import { renderHook } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { MemoryRouter } from "react-router-dom";
import { of } from "rxjs";
import { beforeEach, describe, expect, it } from "vitest";
import { AppCompositionContext } from "@/app/composition/app-composition";
import type { AppComposition } from "@/app/composition/create-app-composition";
import { useAppStore } from "@/app/state/store";
import { useCurrentSurface } from "@/modules/copilot/hooks/use-current-surface";
import { initialWorkspaceShellState } from "@/modules/workspace/domain";
import type { BrowserTreeNode } from "@/shared/api/models";

describe("useCurrentSurface", () => {
  beforeEach(() => {
    useAppStore.setState({
      workspace: initialWorkspaceShellState,
      openTickerSymbol: null,
    });
  });

  it("keeps artifact context compatible while adding the visible label and kind", () => {
    useAppStore.getState().openArtifact("artifact-1");
    const tree: BrowserTreeNode[] = [
      {
        id: "node-1",
        kind: "artifact",
        title: "Collar payoff",
        artifactPreview: {
          id: "artifact-1",
          title: "Collar payoff",
          kind: "app",
        },
        children: [],
      },
    ];

    const { result } = renderHook(() => useCurrentSurface(), {
      wrapper: ({ children }: PropsWithChildren) => (
        <AppCompositionContext.Provider value={compositionForTree(tree)}>
          <MemoryRouter initialEntries={["/home"]}>{children}</MemoryRouter>
        </AppCompositionContext.Provider>
      ),
    });

    expect(result.current).toEqual({
      kind: "artifact",
      id: "artifact-1",
      label: "Collar payoff",
      artifactKind: "app",
    });
  });
});

function compositionForTree(tree: BrowserTreeNode[]): AppComposition {
  return {
    services: {
      workspace: {
        tree: () => of(tree),
      },
    },
  } as unknown as AppComposition;
}
