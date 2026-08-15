import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { HealthWarningBanner } from "@/modules/copilot/ui/copilot-banners";

describe("HealthWarningBanner", () => {
  it("stays hidden without a warning", () => {
    const { container } = render(<HealthWarningBanner message={null} onDismiss={vi.fn()} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows the preflight warning message", () => {
    render(
      <HealthWarningBanner
        message="Copilot tools are not reachable. You can still chat, but workspace actions may fail."
        onDismiss={vi.fn()}
      />,
    );

    expect(screen.getByText(/Copilot tools are not reachable/)).toBeTruthy();
  });

  it("dismisses the warning from the close button", () => {
    const onDismiss = vi.fn();
    render(<HealthWarningBanner message="Tool server is offline." onDismiss={onDismiss} />);

    fireEvent.click(screen.getByLabelText("Dismiss warning"));
    expect(onDismiss).toHaveBeenCalledOnce();
  });
});
