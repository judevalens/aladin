import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { initialSessionState } from "@/app/state/session-slice";
import { useAppStore } from "@/app/state/store";
import { initialWorkspaceShellState, tabKey } from "@/modules/workspace/domain";
import { TabSwitcher } from "@/modules/workspace/ui/tab-switcher";

// The strip's order, verbatim: two research tabs grouped under "Semis cycle", then loose
// artifact tabs. useWorkPane already applies §12 grouping, and the switcher reads that same
// array — so the fixture is what the strip would render.
const TABS = [
  {
    key: "research:r1:overview",
    label: "Semis cycle · Overview",
    tab: { kind: "research" as const, contextId: "r1", view: "overview" as const },
    groupTitle: "Semis cycle",
    viewLabel: "Overview",
  },
  {
    key: "research:r1:runs",
    label: "Semis cycle · Runs",
    tab: { kind: "research" as const, contextId: "r1", view: "runs" as const },
    groupTitle: "Semis cycle",
    viewLabel: "Runs",
  },
  {
    key: "a1",
    label: "Momentum notes",
    tab: { kind: "artifact" as const, artifactId: "a1" },
    artifact: { id: "a1", kind: "note", title: "Momentum notes", folderId: "f1" } as never,
    parentFolderTitle: "Ideas",
  },
  {
    key: "a2",
    label: "Vol surface",
    tab: { kind: "artifact" as const, artifactId: "a2" },
    artifact: { id: "a2", kind: "note", title: "Vol surface", folderId: "f2" } as never,
    parentFolderTitle: "Scratch",
  },
];

const activateTab = vi.fn();
const closeTab = vi.fn();

vi.mock("@/modules/workspace/hooks/use-workspace-state", () => ({
  useWorkPane: () => ({
    tabs: TABS,
    onActivateTab: activateTab,
    onCloseTab: closeTab,
  }),
}));

/** MRU: a2 is active, then a1, then the two research views. */
function seed(mru = ["a2", "a1", "research:r1:runs", "research:r1:overview"]) {
  useAppStore.setState({
    session: initialSessionState,
    workspace: { ...initialWorkspaceShellState, activeTabKey: mru[0] ?? null, tabMru: mru },
    tabSwitcherOpen: true,
  });
}

const rowNames = () =>
  screen
    .getAllByRole("button")
    .map((b) => b.textContent?.trim() ?? "")
    .filter(Boolean);

describe("TabSwitcher", () => {
  beforeEach(() => {
    activateTab.mockClear();
    closeTab.mockClear();
    seed();
  });

  it("lists tabs in MRU order, not the strip's order", () => {
    render(<TabSwitcher />);
    // Strip order is overview, runs, a1, a2 — MRU is the reverse-ish of that.
    expect(rowNames()).toEqual(["Vol surfaceScratch", "Momentum notesIdeas", "Runs", "Overview"]);
  });

  it("opens with the highlight on index 1, the previously-active tab", () => {
    render(<TabSwitcher />);
    fireEvent.keyUp(document, { key: "Control" });
    // Committing straight away activates a1, which is the toggle-to-previous behaviour.
    expect(activateTab).toHaveBeenCalledWith("a1");
  });

  it("advances one row per Tab and commits exactly where it lands", () => {
    render(<TabSwitcher />);
    fireEvent.keyDown(document, { key: "Tab", ctrlKey: true });
    fireEvent.keyDown(document, { key: "Tab", ctrlKey: true });
    fireEvent.keyUp(document, { key: "Control" });
    // index 1 → 2 → 3 = the 4th MRU entry.
    expect(activateTab).toHaveBeenCalledWith("research:r1:overview");
  });

  it("wraps at the end", () => {
    render(<TabSwitcher />);
    for (let i = 0; i < 4; i += 1) fireEvent.keyDown(document, { key: "Tab", ctrlKey: true });
    fireEvent.keyUp(document, { key: "Control" });
    // 1 → 2 → 3 → 0 → 1
    expect(activateTab).toHaveBeenCalledWith("a1");
  });

  it("Shift+Tab moves backwards", () => {
    render(<TabSwitcher />);
    fireEvent.keyDown(document, { key: "Tab", ctrlKey: true, shiftKey: true });
    fireEvent.keyUp(document, { key: "Control" });
    expect(activateTab).toHaveBeenCalledWith("a2");
  });

  it("Esc leaves the active tab unchanged and closes", () => {
    render(<TabSwitcher />);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(activateTab).not.toHaveBeenCalled();
    expect(useAppStore.getState().tabSwitcherOpen).toBe(false);
  });

  it("filters on the label and on the research folder title", () => {
    render(<TabSwitcher />);
    fireEvent.keyDown(document, { key: "s" });
    fireEvent.keyDown(document, { key: "e" });
    fireEvent.keyDown(document, { key: "m" });
    // "sem" matches only the two Semis cycle rows, via groupTitle — the rows themselves show
    // just "Overview"/"Runs", so matching the label alone would find nothing.
    expect(rowNames()).toEqual(["Runs", "Overview"]);
  });

  it("shows the empty state when a filter matches nothing, and Enter does nothing", () => {
    render(<TabSwitcher />);
    fireEvent.keyDown(document, { key: "z" });
    fireEvent.keyDown(document, { key: "z" });
    expect(screen.getByText("No open tab matches.")).toBeInTheDocument();
    fireEvent.keyDown(document, { key: "Enter" });
    expect(activateTab).not.toHaveBeenCalled();
  });

  it("Backspace closes the highlighted tab and keeps the overlay open", () => {
    render(<TabSwitcher />);
    fireEvent.keyDown(document, { key: "Backspace" });
    expect(closeTab).toHaveBeenCalledWith("a1");
    expect(useAppStore.getState().tabSwitcherOpen).toBe(true);
  });

  it("clicking a row commits that row", () => {
    render(<TabSwitcher />);
    fireEvent.click(screen.getByRole("button", { name: /vol surface/i }));
    expect(activateTab).toHaveBeenCalledWith("a2");
  });

  it("clicking the scrim cancels", () => {
    render(<TabSwitcher />);
    fireEvent.mouseDown(screen.getByTestId("tab-switcher-scrim"));
    expect(activateTab).not.toHaveBeenCalled();
    expect(useAppStore.getState().tabSwitcherOpen).toBe(false);
  });

  it("renders research rows under their folder header, in the strip's group order", () => {
    render(<TabSwitcher />);
    expect(screen.getByText("Semis cycle")).toBeInTheDocument();
    // One header for the group, not one per row.
    expect(screen.getAllByText("Semis cycle")).toHaveLength(1);
  });

  it("never mutates openTabs — the §12 grouping invariant survives any interaction", () => {
    const openTabs = [
      { kind: "research" as const, contextId: "r1", view: "overview" as const },
      { kind: "research" as const, contextId: "r1", view: "runs" as const },
      { kind: "artifact" as const, artifactId: "a1" },
      { kind: "artifact" as const, artifactId: "a2" },
    ];
    useAppStore.setState({
      workspace: {
        ...initialWorkspaceShellState,
        openTabs,
        activeTabKey: "a2",
        tabMru: ["a2", "a1", "research:r1:runs", "research:r1:overview"],
      },
      tabSwitcherOpen: true,
    });
    const before = useAppStore.getState().workspace.openTabs.map(tabKey);

    render(<TabSwitcher />);
    fireEvent.keyDown(document, { key: "Tab", ctrlKey: true });
    fireEvent.keyDown(document, { key: "Tab", ctrlKey: true });
    fireEvent.keyUp(document, { key: "Control" });

    expect(useAppStore.getState().workspace.openTabs.map(tabKey)).toEqual(before);
  });
});
