import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { BrowserPaneUI } from "@/modules/workspace/ui/browser-pane-ui";

vi.mock("@/modules/workspace/hooks/use-workspace-state", () => ({
  useBrowserPane: () => ({
    loading: false,
    errorMessage: null,
    rows: [
      {
        id: "folder-1",
        depth: 0,
        kind: "folder",
        title: "Folder One",
        ancestorFolderIds: ["folder-1"],
        folderId: "folder-1",
      },
    ],
    activeArtifactId: null,
    canGoBack: false,
    browserPath: [],
    browserRootTitle: null,
    expandedFolderIds: ["folder-1"],
    browserScrollTop: 0,
    onToggleFolder: vi.fn(),
    onDrillIntoFolder: vi.fn(),
    onPopBrowserFrame: vi.fn(),
    onBrowserScroll: vi.fn(),
    onOpenArtifact: vi.fn(),
    onStartRenameFolder: vi.fn(),
    onStartRenameArtifact: vi.fn(),
    onCreateFolderHere: vi.fn(),
    onCreateNoteHere: vi.fn(),
  }),
}));

describe("BrowserPaneUI", () => {
  it("renders the row action and overflow menu as separate buttons", () => {
    render(<BrowserPaneUI />);

    const rowAction = screen.getByText("Folder One").closest("button");
    if (!rowAction) {
      throw new Error("Expected folder row action button");
    }
    const rowContainer = rowAction.parentElement;
    expect(rowContainer).not.toBeNull();

    const buttons = within(rowContainer as HTMLElement).getAllByRole("button");
    expect(buttons).toHaveLength(2);
    expect(screen.getByRole("button", { name: /more actions for folder one/i })).toBeInTheDocument();
  });
});
