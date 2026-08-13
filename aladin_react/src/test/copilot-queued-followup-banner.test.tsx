import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { QueuedFollowupBanner } from "@/modules/copilot/ui/copilot-dock-ui";

describe("QueuedFollowupBanner", () => {
  it("stays hidden without queued text", () => {
    const { container } = render(<QueuedFollowupBanner text={null} onClear={vi.fn()} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows the queued follow-up text", () => {
    render(<QueuedFollowupBanner text="then publish it" onClear={vi.fn()} />);
    expect(screen.getByText(/queued — sends when the copilot finishes/)).toBeTruthy();
    expect(screen.getByText(/then publish it/)).toBeTruthy();
  });

  it("clears the queued follow-up from the remove button", () => {
    const onClear = vi.fn();
    render(<QueuedFollowupBanner text="continue" onClear={onClear} />);

    fireEvent.click(screen.getByLabelText("Remove queued message"));
    expect(onClear).toHaveBeenCalledOnce();
  });
});
