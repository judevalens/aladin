import { fireEvent, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import type { CopilotProposal, CopilotThreadView } from "@/app/state/copilot-slice";
import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { ThreadMenuItems, threadMenuSections } from "@/modules/copilot/ui/copilot-dock-ui";

describe("ThreadMenuItems", () => {
  it("renders running, pinned, and approval-needed state", () => {
    renderMenu(
      <ThreadMenuItems
        threads={threads}
        query=""
        activeThreadId="t1"
        proposals={[proposal("t2")]}
        status="streaming"
        onOpenThread={vi.fn()}
        onRenameThread={async () => true}
        onArchiveThread={async () => true}
        onSetThreadPinned={async () => true}
      />,
    );

    expect(screen.getByText("Pinned shard work")).toBeTruthy();
    expect(screen.getByText("pinned")).toBeTruthy();
    expect(screen.getByText("running")).toBeTruthy();
    expect(screen.getByText("Approval thread")).toBeTruthy();
    expect(screen.getByText("approval")).toBeTruthy();
  });

  it("filters by title and shows empty states", () => {
    const noop = vi.fn();
    const props = {
      activeThreadId: null,
      proposals: [],
      status: "idle" as const,
      onOpenThread: noop,
      onRenameThread: async () => true,
      onArchiveThread: async () => true,
      onSetThreadPinned: async () => true,
    };

    const { rerender } = renderMenu(<ThreadMenuItems {...props} threads={threads} query="approval" />);
    expect(screen.getByText("Approval thread")).toBeTruthy();
    expect(screen.queryByText("Pinned shard work")).toBeNull();

    rerender(menu(<ThreadMenuItems {...props} threads={threads} query="missing" />));
    expect(screen.getByText("No matching threads")).toBeTruthy();

    rerender(menu(<ThreadMenuItems {...props} threads={[]} query="" />));
    expect(screen.getByText("No saved threads")).toBeTruthy();
  });

  it("groups threads by active, approval, pinned, and recent priority", () => {
    const grouped = threadMenuSections(
      [
        { id: "t1", title: "Pinned shard work", updatedAt: "2026-01-02T00:00:00Z", pinned: true },
        { id: "t2", title: "Approval thread", updatedAt: "2026-01-01T00:00:00Z" },
        { id: "t3", title: "Running thread", updatedAt: "2026-01-03T00:00:00Z", pinned: true },
        { id: "t4", title: "Recent research", updatedAt: "2026-01-04T00:00:00Z" },
      ],
      "t3",
      [proposal("t2")],
      "streaming",
    );

    expect(grouped.map((section) => section.label)).toEqual([
      "Active",
      "Needs Approval",
      "Pinned",
      "Recent",
    ]);
    expect(grouped.map((section) => section.rows.map((row) => row.thread.id))).toEqual([
      ["t3"],
      ["t2"],
      ["t1"],
      ["t4"],
    ]);
  });

  it("renders thread groups in the menu", () => {
    renderMenu(
      <ThreadMenuItems
        threads={[...threads, { id: "t3", title: "Recent research", updatedAt: "2026-01-03T00:00:00Z" }]}
        query=""
        activeThreadId="t1"
        proposals={[proposal("t2")]}
        status="streaming"
        onOpenThread={vi.fn()}
        onRenameThread={async () => true}
        onArchiveThread={async () => true}
        onSetThreadPinned={async () => true}
      />,
    );

    expect(screen.getByText("Active")).toBeTruthy();
    expect(screen.getByText("Needs Approval")).toBeTruthy();
    expect(screen.getByText("Recent")).toBeTruthy();
    expect(screen.getByText("Recent research")).toBeTruthy();
  });

  it("opens, pins, renames, and archives without triggering the row open action", async () => {
    const open = vi.fn();
    const rename = vi.fn(async () => true);
    const archive = vi.fn(async () => true);
    const setPinned = vi.fn(async () => true);

    renderMenu(
      <ThreadMenuItems
        threads={threads}
        query=""
        activeThreadId={null}
        proposals={[]}
        status="idle"
        onOpenThread={open}
        onRenameThread={rename}
        onArchiveThread={archive}
        onSetThreadPinned={setPinned}
      />,
    );

    fireEvent.click(screen.getByLabelText("Unpin thread"));
    expect(setPinned).toHaveBeenCalledWith("t1", false);
    expect(open).not.toHaveBeenCalled();

    fireEvent.click(screen.getAllByLabelText("Archive thread")[0]);
    expect(archive).toHaveBeenCalledWith("t1");
    expect(open).not.toHaveBeenCalled();

    fireEvent.click(screen.getAllByLabelText("Rename thread")[0]);
    const input = screen.getByDisplayValue("Pinned shard work");
    fireEvent.change(input, { target: { value: "Better title" } });
    fireEvent.click(screen.getByLabelText("Save thread title"));
    expect(rename).toHaveBeenCalledWith("t1", "Better title");
  });
});

const threads: CopilotThreadView[] = [
  { id: "t1", title: "Pinned shard work", updatedAt: "2026-01-02T00:00:00Z", pinned: true },
  { id: "t2", title: "Approval thread", updatedAt: "2026-01-01T00:00:00Z" },
];

function proposal(threadId: string): CopilotProposal {
  return {
    actionId: "a1",
    threadId,
    sessionId: "s1",
    tool: "publish_app",
    summary: "Publish the shard",
    status: "pending",
  };
}

function renderMenu(children: ReactNode) {
  return render(menu(children));
}

function menu(children: ReactNode) {
  return (
    <DropdownMenu open modal={false}>
      <DropdownMenuTrigger>Threads</DropdownMenuTrigger>
      <DropdownMenuContent forceMount>{children}</DropdownMenuContent>
    </DropdownMenu>
  );
}
