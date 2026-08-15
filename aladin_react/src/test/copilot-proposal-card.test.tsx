import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { CopilotProposal } from "@/app/state/copilot-slice";
import { ProposalCard } from "@/modules/copilot/ui/copilot-transcript";

describe("ProposalCard", () => {
  it("renders a pending approval with actionable approve and reject buttons", () => {
    const approve = vi.fn();
    const reject = vi.fn();
    render(<ProposalCard proposal={proposal("pending")} onApprove={approve} onReject={reject} />);

    expect(screen.getByText("Publish the shard")).toBeTruthy();
    expect(screen.getByText("Needs your approval — the copilot is waiting")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    expect(approve).toHaveBeenCalledOnce();
    expect(reject).toHaveBeenCalledOnce();
  });

  it("locks actions while approving or rejecting", () => {
    const { rerender } = render(
      <ProposalCard proposal={proposal("approving")} onApprove={vi.fn()} onReject={vi.fn()} />,
    );
    expect(screen.getByText("Approving…")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Approve" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Reject" })).toBeDisabled();

    rerender(<ProposalCard proposal={proposal("rejecting")} onApprove={vi.fn()} onReject={vi.fn()} />);
    expect(screen.getByText("Dismissing…")).toBeTruthy();
  });

  it("collapses settled approvals to a one-line note", () => {
    render(
      <ProposalCard
        proposal={{ ...proposal("approved"), message: "Published." }}
        onApprove={vi.fn()}
        onReject={vi.fn()}
      />,
    );

    expect(screen.getByText("Published.")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Approve" })).toBeNull();
  });
});

function proposal(status: CopilotProposal["status"]): CopilotProposal {
  return {
    actionId: "a1",
    threadId: "t1",
    sessionId: "s1",
    tool: "publish_app",
    summary: "Publish the shard",
    status,
  };
}
