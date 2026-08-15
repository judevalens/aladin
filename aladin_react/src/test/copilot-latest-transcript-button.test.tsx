import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { LatestTranscriptButton } from "@/modules/copilot/ui/copilot-banners";

describe("LatestTranscriptButton", () => {
  it("stays hidden when the transcript is pinned or idle", () => {
    const { container } = render(<LatestTranscriptButton visible={false} onClick={vi.fn()} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows the latest control when the transcript is unpinned during work", () => {
    render(<LatestTranscriptButton visible onClick={vi.fn()} />);
    expect(screen.getByRole("button", { name: /latest/i })).toBeTruthy();
  });

  it("repins the transcript from the latest button", () => {
    const onClick = vi.fn();
    render(<LatestTranscriptButton visible onClick={onClick} />);

    fireEvent.click(screen.getByRole("button", { name: /latest/i }));
    expect(onClick).toHaveBeenCalledOnce();
  });
});
