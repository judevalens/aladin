import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  DeleteConfirmDialog,
  type DeleteTarget,
} from "@/modules/workspace/ui/delete-confirm-dialog";

// Delete is the one destructive thing the tree can do, and tree rows are small and close
// together — a right-click landing one row off is easy. These pin the parts that make the
// confirmation trustworthy: it names the thing, it says how much goes with it, and it does
// NOT close on failure (a dialog that closes reads as success).

const folder: DeleteTarget = { kind: "folder", id: "f1", title: "Semis cycle", childCount: 4 };
const artifact: DeleteTarget = { kind: "artifact", id: "a1", title: "Momentum notes" };

function setup(target: DeleteTarget | null, onConfirm = vi.fn().mockResolvedValue(undefined)) {
  const onCancel = vi.fn();
  render(<DeleteConfirmDialog target={target} onCancel={onCancel} onConfirm={onConfirm} />);
  return { onConfirm, onCancel };
}

describe("DeleteConfirmDialog", () => {
  it("is closed when there is no target", () => {
    setup(null);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("names the thing being deleted and how much goes with it", () => {
    setup(folder);
    expect(screen.getByText("Delete folder?")).toBeInTheDocument();
    expect(screen.getByText("Semis cycle")).toBeInTheDocument();
    expect(screen.getByText(/4 items/)).toBeInTheDocument();
  });

  it("uses the singular for a single child", () => {
    setup({ ...folder, childCount: 1 });
    expect(screen.getByText(/1 item\b/)).toBeInTheDocument();
    expect(screen.queryByText(/1 items/)).not.toBeInTheDocument();
  });

  it("omits the count for an empty folder rather than saying '0 items'", () => {
    setup({ ...folder, childCount: 0 });
    expect(screen.queryByText(/0 item/)).not.toBeInTheDocument();
    expect(screen.getByText(/will be removed from your workspace/)).toBeInTheDocument();
  });

  it("calls the research folder a research folder", () => {
    setup({ ...folder, kind: "research" });
    expect(screen.getByText("Delete research folder?")).toBeInTheDocument();
  });

  it("does not promise permanence — the server tombstones and keeps the body", () => {
    setup(artifact);
    const dialog = screen.getByRole("dialog");
    expect(dialog.textContent).toContain("removed from your workspace");
    expect(dialog.textContent).not.toMatch(/permanent|cannot be undone|forever/i);
  });

  it("confirms with the target", async () => {
    const { onConfirm } = setup(artifact);
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(onConfirm).toHaveBeenCalledWith(artifact));
  });

  it("cancels without deleting", () => {
    const { onConfirm, onCancel } = setup(artifact);
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onConfirm).not.toHaveBeenCalled();
    expect(onCancel).toHaveBeenCalled();
  });

  it("stays open and shows the reason when the delete fails", async () => {
    const onConfirm = vi.fn().mockRejectedValue(new Error("Network unreachable"));
    const { onCancel } = setup(artifact, onConfirm);

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));

    await screen.findByText("Network unreachable");
    // Closing here would read as "deleted".
    expect(onCancel).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("re-enables the button after a failure so it can be retried", async () => {
    const onConfirm = vi.fn().mockRejectedValue(new Error("nope"));
    setup(artifact, onConfirm);
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    await screen.findByText("nope");
    const button = screen.getByRole("button", { name: "Delete" });
    expect(button).not.toBeDisabled();
    fireEvent.click(button);
    await waitFor(() => expect(onConfirm).toHaveBeenCalledTimes(2));
  });
});
