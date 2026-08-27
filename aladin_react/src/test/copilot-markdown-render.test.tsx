import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import { CopilotMarkdown } from "@/modules/copilot/ui/copilot-markdown";

describe("CopilotMarkdown rich directives", () => {
  it("keeps timestamps intact in headings and bold text", () => {
    const { container } = render(
      <MemoryRouter>
        <CopilotMarkdown text={"## Market now — Aug. 27, ~10:22 a.m. PT\n\nAs of **11:00 a.m. ET**, quotes are current."} />
      </MemoryRouter>,
    );

    expect(screen.getByRole("heading").textContent).toBe("Market now — Aug. 27, ~10:22 a.m. PT");
    expect(container.querySelector("strong")?.textContent).toBe("11:00 a.m. ET");
    expect(container.querySelector("h2 div, strong div")).toBeNull();
  });

  it("preserves ordinary colon syntax and unknown directives as inert text", () => {
    const text = 'Time 10:22:05, ratio 1:2, code HTTP:ERROR, and :note[hello]{title="note"}.\n\n::note[leaf]\n\n:::note\nbody\n:::';
    const { container } = render(<MemoryRouter><CopilotMarkdown text={text} /></MemoryRouter>);

    expect(container.querySelector("p")?.textContent).toBe(text.split("\n\n")[0]);
    expect(screen.getByText("::note[leaf]")).toBeTruthy();
    expect(screen.getByText(/:::note/).textContent).toBe(":::note\nbody\n:::");
    expect(container.querySelector("p div")).toBeNull();
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("keeps partial timestamps intact as streamed text grows", () => {
    const text = "## Market now — Aug. 27, ~10:22 a.m. PT";
    const { container, rerender } = render(<MemoryRouter><CopilotMarkdown text="" /></MemoryRouter>);
    for (let length = text.indexOf("10:") + 3; length <= text.length; length += 1) {
      const partial = text.slice(0, length);
      rerender(<MemoryRouter><CopilotMarkdown text={partial} /></MemoryRouter>);
      expect(screen.getByRole("heading").textContent).toBe(partial.slice(3).trimEnd());
      expect(container.querySelector("h2 div")).toBeNull();
    }
  });

  it("preserves code and links while still rendering Aladin directives next to timestamps", () => {
    const { container } = render(
      <MemoryRouter>
        <CopilotMarkdown text={'At **10:22**, check ::aladin-ticker{symbol="QQQ"}.\n\n[Source](https://example.com:8443/quote) and `10:22`.\n\n```\n10:22 :note[code]\n```'} />
      </MemoryRouter>,
    );

    expect(container.querySelector("strong")?.textContent).toBe("10:22");
    expect(screen.getByRole("button", { name: /QQQ/ })).toBeTruthy();
    expect(screen.getByRole("link", { name: "Source" }).getAttribute("href")).toBe("https://example.com:8443/quote");
    expect(container.querySelector("p > code")?.textContent).toBe("10:22");
    expect(container.querySelector("pre > code")?.textContent).toBe("10:22 :note[code]\n");
  });

  it("preserves paragraphs, list numbering, table alignment, and fenced code without a language", () => {
    const { container } = render(
      <MemoryRouter>
        <CopilotMarkdown text={'Checking.\n\n## Result\n\n**Market** snapshot\n\n3. Third\n4. Fourth\n\n| Price |\n| ---: |\n| 100 |\n\n```\nline 1\n  line 2\n```\n\nInline `quote`.\n\n```json\n{"ok":true}\n```'} />
      </MemoryRouter>,
    );
    expect(screen.getByRole("heading", { name: "Result" })).toBeTruthy();
    expect(container.querySelector("strong")?.textContent).toBe("Market");
    expect(screen.getByRole("list").getAttribute("start")).toBe("3");
    expect(screen.getByRole("cell").style.textAlign).toBe("right");
    expect(container.querySelector("pre > code")?.textContent).toBe("line 1\n  line 2\n");
    expect(container.querySelector("pre > code.language-json")?.textContent).toBe('{"ok":true}\n');
    expect(container.querySelector("p > code")?.textContent).toBe("quote");
  });

  it("renders embedded legacy references inline with trailing punctuation", () => {
    render(
      <MemoryRouter>
        <CopilotMarkdown
          text={
            'also ::aladin-artifact{id="artifact-e5eb2565-2dee-44a2-b759-902adbd6e167" kind="shard" title="Day Trading Playbook"}: still works'
          }
        />
      </MemoryRouter>,
    );

    expect(screen.getByText(/also/)).toBeTruthy();
    expect(screen.getByRole("button", { name: /Day Trading Playbook/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: /Day Trading Playbook/ }).closest("p")).toBeTruthy();
    expect(screen.queryByText("shard")).toBeNull();
    expect(screen.getByText(/: still works/)).toBeTruthy();
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
