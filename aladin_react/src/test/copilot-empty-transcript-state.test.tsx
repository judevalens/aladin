import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { EmptyTranscriptState } from "@/modules/copilot/ui/copilot-dock-ui";

describe("EmptyTranscriptState", () => {
  it("shows grounded empty-state copy and the current surface label", () => {
    render(
      <EmptyTranscriptState
        surface={{ kind: "ticker", symbol: "NVDA" }}
        surfaceLabel="NVDA"
        onPrompt={vi.fn()}
      />,
    );

    expect(screen.getByText("Ask about your research — grounded in your Aladin data.")).toBeTruthy();
    expect(screen.getByText("Looking at NVDA")).toBeTruthy();
  });

  it("shows surface-aware suggestions", () => {
    render(
      <EmptyTranscriptState
        surface={{ kind: "artifact", artifactKind: "app" }}
        surfaceLabel="Momentum shard"
        onPrompt={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: "Polish the interaction design" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "What would make this clearer?" })).toBeTruthy();
  });

  it("sends the clicked suggestion", () => {
    const onPrompt = vi.fn();
    render(
      <EmptyTranscriptState surface={{ kind: "page" }} surfaceLabel={null} onPrompt={onPrompt} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Summarize what I'm looking at" }));
    expect(onPrompt).toHaveBeenCalledWith("Summarize what I'm looking at");
  });
});
