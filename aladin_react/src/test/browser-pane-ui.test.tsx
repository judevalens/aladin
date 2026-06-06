import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { BrowserPaneUI } from "@/modules/workspace/ui/browser-pane-ui";

vi.mock("@/modules/workspace/hooks/use-workspace-state", () => ({
  useBrowserPane: () => ({
    loading: false,
    errorMessage: null,
    tree: [],
    rows: [
      {
        id: "folder-1",
        depth: 0,
        kind: "folder",
        title: "Folder One",
        ancestorFolderIds: ["folder-1"],
        folderId: "folder-1",
        childCount: 2,
      },
    ],
    activeArtifactId: null,
    expandedFolderIds: ["folder-1"],
    browserScrollTop: 0,
    onToggleFolder: vi.fn(),
    onBrowserScroll: vi.fn(),
    onOpenArtifact: vi.fn(),
    onStartRenameFolder: vi.fn(),
    onStartRenameArtifact: vi.fn(),
    onCreateFolderHere: vi.fn(),
    onCreateNoteHere: vi.fn(),
  }),
}));

describe("BrowserPaneUI", () => {
  it("renders a folder row as a single clickable button (actions via right-click)", () => {
    render(<BrowserPaneUI />);

    const row = screen.getByRole("button", { name: /folder one/i });
    expect(row).toBeInTheDocument();
    // The explorer header exposes the columns (Miller) trigger.
    expect(screen.getByRole("button", { name: /browse in columns/i })).toBeInTheDocument();
  });
});
