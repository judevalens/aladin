import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { Dock } from "@/modules/board/ui/dock";

const action = vi.fn();

function props(tool: "select" | "pencil" | "arrow") {
  return {
    tool,
    subTool: "pen" as const,
    inkColor: "learn" as const,
    weight: 1 as const,
    drawWithFinger: false,
    insertOpen: false,
    pencilMenuOpen: false,
    styleOpen: false,
    appearance: "light" as const,
    zoomPct: 100,
    zoomLocked: false,
    canUndo: true,
    canRedo: false,
    onUndo: action,
    onRedo: action,
    onPickTool: action,
    onPickSubTool: action,
    onPickColor: action,
    onPickWeight: action,
    onToggleDrawWithFinger: action,
    onToggleInsert: action,
    onToggleStyle: action,
    onToggleAppearance: action,
    onZoomIn: action,
    onZoomOut: action,
    onResetZoom: action,
    onFit: action,
    onAddNote: action,
  };
}

function primaryButtonNames() {
  return within(screen.getByRole("toolbar", { name: "Board tools" }))
    .getAllByRole("button")
    .map((button) => button.getAttribute("aria-label") ?? button.textContent?.trim());
}

describe("Board Dock", () => {
  it("keeps every primary control in the same order when tools change", () => {
    const view = render(<Dock {...props("select")} />);
    const selectButtons = primaryButtonNames();

    view.rerender(<Dock {...props("pencil")} />);
    expect(primaryButtonNames()).toEqual(selectButtons);

    view.rerender(<Dock {...props("arrow")} />);
    expect(primaryButtonNames()).toEqual(selectButtons);
    expect(selectButtons).toEqual([
      "Select",
      "Pan",
      "Sticky note",
      "Text",
      "Pencil",
      "Connect",
      "Frame",
      "Add to board",
    ]);
  });

  it("shows Pencil options only while its contextual menu is open", () => {
    const view = render(<Dock {...props("pencil")} />);
    expect(screen.queryByRole("toolbar", { name: "Pencil tools" })).not.toBeInTheDocument();

    view.rerender(<Dock {...props("pencil")} pencilMenuOpen />);
    const pencilTools = screen.getByRole("toolbar", { name: "Pencil tools" });
    expect(within(pencilTools).getByRole("button", { name: "Pen" })).toBePressed();
    expect(within(pencilTools).getByRole("button", { name: "Highlighter" })).toBeVisible();
    expect(within(pencilTools).getByRole("button", { name: "Eraser" })).toBeVisible();
    expect(within(pencilTools).getByRole("button", { name: "Lasso" })).toBeVisible();
    expect(within(pencilTools).getByRole("button", { name: "Stroke settings" })).toBeVisible();
  });

  it("leaves zoom available while Pencil is active", () => {
    render(<Dock {...props("pencil")} />);
    const zoom = screen.getByRole("button", { name: "Reset zoom" });
    expect(zoom).toBeEnabled();
    expect(zoom).toHaveTextContent("100%");
  });

  it("retains the separate paper camera constraint", () => {
    render(<Dock {...props("select")} zoomLocked />);
    expect(screen.getByRole("button", { name: "Zoom in" })).toBeDisabled();
  });

  it("offers dedicated dark and light board modes", () => {
    const view = render(<Dock {...props("select")} />);
    expect(screen.getByRole("button", { name: "Use dark board" })).toBeVisible();
    view.rerender(<Dock {...props("select")} appearance="dark" />);
    expect(screen.getByRole("button", { name: "Use light board" })).toBeVisible();
  });
});
