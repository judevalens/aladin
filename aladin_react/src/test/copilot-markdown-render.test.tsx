import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import { CopilotMarkdown } from "@/modules/copilot/ui/copilot-markdown";

describe("CopilotMarkdown rich directives", () => {
  it("renders artifact leaf directives with trailing punctuation", () => {
    render(
      <MemoryRouter>
        <CopilotMarkdown
          text={
            '::aladin-artifact{id="artifact-e5eb2565-2dee-44a2-b759-902adbd6e167" kind="shard" title="Day Trading Playbook"}:'
          }
        />
      </MemoryRouter>,
    );

    expect(screen.getByRole("button", { name: /Day Trading Playbook/ })).toBeTruthy();
    expect(screen.getByText("shard")).toBeTruthy();
  });

  it("renders native recovery and shard preview blocks with validated actions", () => {
    const onPrompt = vi.fn();
    render(
      <MemoryRouter>
        <CopilotMarkdown
          onPrompt={onPrompt}
          text={[
            "::aladin-shard-preview",
            JSON.stringify({
              artifactId: "shd_1",
              title: "Collar payoff",
              status: "error",
              diagnostics: ["src/index.tsx:12 Missing prop"],
            }),
            "::",
            "",
            "::aladin-error-recovery",
            JSON.stringify({
              title: "Build failed",
              message: "The shard could not compile.",
              code: "BUILD_FAILED",
              actions: [{ action: "retry", label: "Retry build", prompt: "retry the build" }],
            }),
            "::",
          ].join("\n")}
        />
      </MemoryRouter>,
    );

    expect(screen.getByText("Collar payoff")).toBeTruthy();
    expect(screen.getByText("src/index.tsx:12 Missing prop")).toBeTruthy();
    expect(screen.getByText("Build failed")).toBeTruthy();
    expect(screen.getByText("BUILD_FAILED")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: /retry build/i }));
    expect(onPrompt).toHaveBeenCalledWith("retry the build");
  });

  it("renders malformed directives as inert markdown fallback", () => {
    render(
      <MemoryRouter>
        <CopilotMarkdown text={"::aladin-error-recovery\n{\n::"} />
      </MemoryRouter>,
    );

    expect(screen.getByText(/::aladin-error-recovery/)).toBeTruthy();
  });
});
