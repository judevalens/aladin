import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { BrowserPaneUI } from "@/modules/workspace/ui/browser-pane-ui";

// The hook is mocked with a mutable state object so each test can shape the pane —
// rows, selection — and assert which handler the UI routes a gesture to.
const paneState = {
  loading: false,
  errorMessage: null as string | null,
  tree: [] as unknown[],
  rows: [] as unknown[],
  activeArtifactId: null as string | null,
  expandedFolderIds: ["folder-1"],
  browserScrollTop: 0,
  onToggleFolder: vi.fn(),
  onBrowserScroll: vi.fn(),
  onOpenArtifact: vi.fn(),
  onStartRenameFolder: vi.fn(),
  onStartRenameArtifact: vi.fn(),
  onCreateFolderHere: vi.fn(),
  onCreateNoteHere: vi.fn(),
  selectedRowKeys: [] as string[],
  onToggleRowSelected: vi.fn(),
  onSelectRangeTo: vi.fn(),
  onClearSelection: vi.fn(),
  onRetargetSelection: vi.fn(),
  selectionTargets: [] as unknown[],
  onDeleteTargets: vi.fn(),
};

vi.mock("@/modules/workspace/hooks/use-workspace-state", () => ({
  useBrowserPane: () => paneState,
}));

const folderRow = {
  id: "folder-1",
  depth: 0,
  kind: "folder",
  title: "Folder One",
  ancestorFolderIds: ["folder-1"],
  folderId: "folder-1",
  childCount: 2,
};
const noteRow = {
  id: "node-note",
  depth: 1,
  kind: "artifact",
  title: "Note One",
  ancestorFolderIds: ["folder-1"],
  artifactId: "note-1",
  artifactKind: "note",
};

beforeEach(() => {
  vi.clearAllMocks();
  paneState.rows = [folderRow, noteRow];
  paneState.selectedRowKeys = [];
  paneState.selectionTargets = [];
});

describe("BrowserPaneUI", () => {
  it("renders a folder row as a single clickable button (actions via right-click)", () => {
    render(<BrowserPaneUI />);

    const row = screen.getByRole("button", { name: /folder one/i });
    expect(row).toBeInTheDocument();
    // The explorer header exposes the columns (Miller) trigger.
    expect(screen.getByRole("button", { name: /browse in columns/i })).toBeInTheDocument();
  });

  it("routes a plain click to open, a ⌘-click to selection", () => {
    render(<BrowserPaneUI />);
    const row = screen.getByRole("button", { name: /note one/i });

    fireEvent.click(row);
    expect(paneState.onOpenArtifact).toHaveBeenCalledWith("note-1");
    expect(paneState.onToggleRowSelected).not.toHaveBeenCalled();

    fireEvent.click(row, { metaKey: true });
    // Selection keys are ROW ids, not artifact ids.
    expect(paneState.onToggleRowSelected).toHaveBeenCalledWith("node-note");
    expect(paneState.onOpenArtifact).toHaveBeenCalledTimes(1);
  });

  it("routes ⇧-click to a range only while a selection exists", () => {
    render(<BrowserPaneUI />);
    const row = screen.getByRole("button", { name: /note one/i });

    fireEvent.click(row, { shiftKey: true });
    expect(paneState.onSelectRangeTo).not.toHaveBeenCalled();

    paneState.selectedRowKeys = ["folder-1"];
    render(<BrowserPaneUI />);
    fireEvent.click(screen.getAllByRole("button", { name: /note one/i }).at(-1)!, {
      shiftKey: true,
    });
    expect(paneState.onSelectRangeTo).toHaveBeenCalledWith("node-note");
  });

  it("shows the docked selection bar only while something is selected", () => {
    render(<BrowserPaneUI />);
    expect(screen.queryByText(/selected/)).not.toBeInTheDocument();

    paneState.selectedRowKeys = ["folder-1", "node-note"];
    render(<BrowserPaneUI />);
    expect(screen.getByText("2 selected")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Clear" })).toBeInTheDocument();
  });

  it("opens the bulk confirm from the selection bar with the coalesced targets", () => {
    paneState.selectedRowKeys = ["folder-1", "node-note"];
    paneState.selectionTargets = [
      { kind: "folder", id: "folder-1", title: "Folder One", childCount: 2 },
    ];
    render(<BrowserPaneUI />);

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByText("Delete folder?")).toBeInTheDocument();
  });

  it("clears a plain-clicked selection before opening", () => {
    paneState.selectedRowKeys = ["folder-1"];
    render(<BrowserPaneUI />);

    fireEvent.click(screen.getByRole("button", { name: /note one/i }));
    expect(paneState.onClearSelection).toHaveBeenCalled();
    expect(paneState.onOpenArtifact).toHaveBeenCalledWith("note-1");
  });
});
