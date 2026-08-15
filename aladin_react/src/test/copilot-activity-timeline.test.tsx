import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ActivityTimeline } from "@/modules/copilot/ui/copilot-transcript";
import type { CopilotToolRun } from "@/app/state/copilot-slice";

describe("ActivityTimeline", () => {
  it("groups consecutive runs and renders expandable summaries", () => {
    render(
      <ActivityTimeline
        trail={[
          tool("search", "Searching workspace", "ok", "query: NVDA", "3 results"),
          tool("search", "Searching workspace", "error", "query: CUDA", "timeout"),
          tool("build_app", "Building shard", "running", "pageId: shd_1"),
        ]}
      />,
    );

    expect(screen.getByText("activity")).toBeTruthy();
    expect(screen.getByText("Searching workspace")).toBeTruthy();
    expect(screen.getByText("x2")).toBeTruthy();
    expect(screen.getByText("in: query: CUDA")).toBeTruthy();
    expect(screen.getByText("out: timeout")).toBeTruthy();
    expect(screen.getByText("Building shard")).toBeTruthy();
  });

  it("shows only recent groups with an overflow count", () => {
    render(
      <ActivityTimeline
        trail={[
          tool("t0", "Step 0", "ok"),
          tool("t1", "Step 1", "ok"),
          tool("t2", "Step 2", "ok"),
          tool("t3", "Step 3", "ok"),
          tool("t4", "Step 4", "ok"),
          tool("t5", "Step 5", "ok"),
          tool("t6", "Step 6", "ok"),
          tool("t7", "Step 7", "running"),
        ]}
      />,
    );

    expect(screen.getByText("+2")).toBeTruthy();
    expect(screen.queryByText("Step 0")).toBeNull();
    expect(screen.queryByText("Step 1")).toBeNull();
    expect(screen.getByText("Step 7")).toBeTruthy();
  });
});

function tool(
  name: string,
  label: string,
  status: CopilotToolRun["status"],
  inputSummary?: string,
  resultSummary?: string,
): CopilotToolRun {
  return { name, label, status, inputSummary, resultSummary };
}
