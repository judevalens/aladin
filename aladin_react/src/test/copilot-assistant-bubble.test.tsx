import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AssistantBubble } from "@/modules/copilot/ui/copilot-dock-ui";

const navCitation = vi.fn();

vi.mock("@/modules/copilot/hooks/use-citation-nav", () => ({
  useCitationNav: () => navCitation,
}));

describe("AssistantBubble", () => {
  beforeEach(() => {
    navCitation.mockReset();
  });

  it("dedupes citations by kind and id", () => {
    render(
      <AssistantBubble
        content="Found it."
        citations={[
          { kind: "ticker", id: "NVDA", title: "NVIDIA" },
          { kind: "ticker", id: "NVDA", title: "NVIDIA duplicate" },
          { kind: "artifact", id: "shard-1", title: "Momentum shard" },
        ]}
        onPrompt={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: "NVIDIA" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "NVIDIA duplicate" })).toBeNull();
    expect(screen.getByRole("button", { name: "Momentum shard" })).toBeTruthy();
  });

  it("navigates from citation chips", () => {
    render(
      <AssistantBubble
        content="Open the shard."
        citations={[{ kind: "artifact", id: "shard-1", title: "Momentum shard" }]}
        onPrompt={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Momentum shard" }));
    expect(navCitation).toHaveBeenCalledWith({
      kind: "artifact",
      id: "shard-1",
      title: "Momentum shard",
    });
  });

  it("shows completed turn digest and hides it while streaming", () => {
    const meta = {
      activity: [
        { name: "read_file", ok: true },
        { name: "read_file", ok: true },
        { name: "build_app", ok: false },
      ],
      costUsd: 0.14,
      numTurns: 3,
    };
    const { rerender } = render(
      <AssistantBubble content="Done." citations={[]} meta={meta} onPrompt={vi.fn()} />,
    );

    expect(screen.getByText("read shard files ×2 · built shard ✗ — $0.14 · 3 steps")).toBeTruthy();

    rerender(
      <AssistantBubble content="Still working." citations={[]} meta={meta} streaming onPrompt={vi.fn()} />,
    );
    expect(screen.queryByText(/read shard files/)).toBeNull();
  });
});
