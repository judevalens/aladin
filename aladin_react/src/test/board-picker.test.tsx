import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { PickerPanel } from "@/modules/board/ui/picker-panel";

function setup() {
  const props = { query: "", onQueryChange: vi.fn(), rows: [{ key: "source", icon: null, title: "Research paper", meta: "PDF", onPick: vi.fn() }], note: { kind: "none" as const }, onPaste: vi.fn(), onClose: vi.fn(), onAddLink: vi.fn(), onAddTask: vi.fn(), onAddCard: vi.fn() };
  render(<PickerPanel {...props} />);
  return props;
}
describe("board library", () => {
  it("adds real workspace rows and keeps legacy creation actions", () => {
    const props = setup();
    fireEvent.click(screen.getByRole("button", { name: /Research paper/ }));
    expect(props.rows[0].onPick).toHaveBeenCalledOnce();
    fireEvent.click(screen.getByRole("button", { name: "Add task" }));
    fireEvent.click(screen.getByRole("button", { name: "Add two-sided card" }));
    expect(props.onAddTask).toHaveBeenCalledOnce();
    expect(props.onAddCard).toHaveBeenCalledOnce();
  });
  it("only inserts complete HTTP(S) links", () => {
    const props = setup();
    fireEvent.change(screen.getByLabelText("Or drop in a link"), { target: { value: "javascript:alert(1)" } });
    fireEvent.click(screen.getByRole("button", { name: "Add link" }));
    expect(props.onAddLink).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent("http or https");
    fireEvent.change(screen.getByLabelText("Or drop in a link"), { target: { value: "https://example.com/method" } });
    fireEvent.click(screen.getByRole("button", { name: "Add link" }));
    expect(props.onAddLink).toHaveBeenCalledWith("https://example.com/method");
  });
  it("dismisses with Escape without leaking the shortcut", () => {
    const props = setup();
    fireEvent.keyDown(screen.getByLabelText("Search your workspace"), { key: "Escape" });
    expect(props.onClose).toHaveBeenCalledOnce();
  });
});
