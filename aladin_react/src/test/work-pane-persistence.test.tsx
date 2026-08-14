import { render, screen } from "@testing-library/react";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { WorkPaneTab } from "@/modules/workspace/hooks/use-workspace-state";

// Regression for "tabs get unmounted when we switch to another tab". The work pane was a switch
// on the ACTIVE artifact, so leaving a tab destroyed its pane and everything the user had built
// up in it — scroll position, editor selection, PDF page, a shard's runtime state.
//
// The assertion that matters is about MOUNT LIFETIME, not visibility: after switching, the first
// tab's pane element must still be in the document. Each mock pane counts its own mounts, so a
// remount is visible as a count going up.

// Each mock pane stamps the mount generation it was created in. A lazy useState initialiser
// runs exactly once per MOUNT, so the number moves only on a genuine teardown-and-rebuild —
// re-renders leave it alone. The last test proves the detector actually moves.
const mountCounts: Record<string, number> = {};

function probe(name: string) {
  return function Probe({ pageId, artifact }: { pageId?: string; artifact?: { id: string } }) {
    const id = pageId ?? artifact?.id ?? "?";
    const key = `${name}:${id}`;
    const [generation] = useState(() => {
      mountCounts[key] = (mountCounts[key] ?? 0) + 1;
      return mountCounts[key];
    });
    return (
      <div data-testid={`pane-${id}`} data-mounts={generation}>
        pane
      </div>
    );
  };
}

vi.mock("@/modules/pages/ui/page-editor-ui", () => ({ PageEditorUI: probe("note") }));
vi.mock("@/modules/doc-surface/ui/doc-surface-ui", () => ({ DocSurfaceUI: probe("app") }));
vi.mock("@/modules/artifacts/ui/artifact-ui", () => ({
  FileArtifactUI: probe("file"),
  LinkArtifactUI: probe("link"),
  VoiceArtifactUI: probe("voice"),
}));
vi.mock("@/modules/documents/ui/document-viewer-ui", () => ({ FileArtifactPaneUI: probe("doc") }));
vi.mock("@/modules/research/ui/research-pane-ui", () => ({ ResearchPaneUI: () => <div /> }));
vi.mock("@/modules/graph/ui/graph-side-pane-ui", () => ({ GraphSidePaneUI: () => <div /> }));

const note = (id: string): WorkPaneTab => ({
  key: id,
  label: id,
  tab: { kind: "artifact", artifactId: id },
  artifact: { id, kind: "note", title: id } as never,
});

let state: { tabs: WorkPaneTab[]; activeKey: string };

vi.mock("@/modules/workspace/hooks/use-workspace-state", () => ({
  useWorkPane: () => ({
    tabs: state.tabs,
    activeTab: state.tabs.find((t) => t.key === state.activeKey) ?? null,
    activeArtifact: state.tabs.find((t) => t.key === state.activeKey)?.artifact ?? null,
    breadcrumbFolders: [],
    artifactTitle: null,
    inspectorOpen: false,
    onActivateTab: vi.fn(),
    onCloseTab: vi.fn(),
    onToggleInspector: vi.fn(),
    onJumpToFolder: vi.fn(),
  }),
}));

const { WorkPaneUI } = await import("@/modules/workspace/ui/work-pane-ui");

describe("work pane tab persistence", () => {
  beforeEach(() => {
    for (const key of Object.keys(mountCounts)) delete mountCounts[key];
    state = { tabs: [note("a1"), note("a2"), note("a3")], activeKey: "a1" };
  });

  it("keeps a pane mounted after switching away from it", () => {
    const { rerender } = render(<WorkPaneUI />);
    expect(screen.getByTestId("pane-a1")).toBeInTheDocument();

    state = { ...state, activeKey: "a2" };
    rerender(<WorkPaneUI />);

    // The old behaviour removed this node outright. It must still be here.
    expect(screen.getByTestId("pane-a1")).toBeInTheDocument();
    expect(screen.getByTestId("pane-a2")).toBeInTheDocument();
  });

  it("does not remount a pane when returning to it", () => {
    const { rerender } = render(<WorkPaneUI />);
    const firstMount = screen.getByTestId("pane-a1").dataset.mounts;

    state = { ...state, activeKey: "a2" };
    rerender(<WorkPaneUI />);
    state = { ...state, activeKey: "a1" };
    rerender(<WorkPaneUI />);

    // Same mount generation: the component was never torn down and rebuilt.
    expect(screen.getByTestId("pane-a1").dataset.mounts).toBe(firstMount);
  });

  it("hides inactive panes rather than removing them, and takes them out of the a11y tree", () => {
    const { rerender } = render(<WorkPaneUI />);
    state = { ...state, activeKey: "a2" };
    rerender(<WorkPaneUI />);

    const inactive = screen.getByTestId("pane-a1").parentElement!;
    const active = screen.getByTestId("pane-a2").parentElement!;
    expect(inactive.className).toContain("hidden");
    expect(inactive).toHaveAttribute("aria-hidden", "true");
    expect(active.className).not.toContain("hidden");
    expect(active).not.toHaveAttribute("aria-hidden", "true");
  });

  it("mounts a pane only once it has been active — a never-visited tab costs nothing", () => {
    render(<WorkPaneUI />);
    // a2 and a3 are open but have never been active.
    expect(screen.queryByTestId("pane-a2")).not.toBeInTheDocument();
    expect(screen.queryByTestId("pane-a3")).not.toBeInTheDocument();
  });

  it("detects a real remount — otherwise the assertion above proves nothing", () => {
    const { rerender } = render(<WorkPaneUI />);
    const before = screen.getByTestId("pane-a1").dataset.mounts;

    // Close a1 and reopen it: a genuine teardown, so the generation must advance.
    state = { tabs: [note("a2")], activeKey: "a2" };
    rerender(<WorkPaneUI />);
    state = { tabs: [note("a1"), note("a2")], activeKey: "a1" };
    rerender(<WorkPaneUI />);

    expect(screen.getByTestId("pane-a1").dataset.mounts).not.toBe(before);
  });

  it("shows the placeholder when nothing is open", () => {
    state = { tabs: [], activeKey: "" };
    render(<WorkPaneUI />);
    expect(screen.getByText("Open a page")).toBeInTheDocument();
  });
});
