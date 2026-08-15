import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { CopilotErrorBanner } from "@/modules/copilot/ui/copilot-banners";

describe("CopilotErrorBanner", () => {
  it("stays hidden without an error", () => {
    const { container } = render(<CopilotErrorBanner error={null} code={null} onContinue={vi.fn()} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows the exact error without recovery chrome by default", () => {
    render(<CopilotErrorBanner error="MCP server is down" code={null} onContinue={vi.fn()} />);
    expect(screen.getByText("MCP server is down")).toBeTruthy();
    expect(screen.queryByRole("button", { name: /continue/i })).toBeNull();
  });

  it("offers a continue action for max-turn recoveries", () => {
    const onContinue = vi.fn();
    render(<CopilotErrorBanner error="Turn reached the step limit" code="max_turns" onContinue={onContinue} />);

    fireEvent.click(screen.getByRole("button", { name: "Continue where it left off" }));
    expect(onContinue).toHaveBeenCalledOnce();
  });
});
