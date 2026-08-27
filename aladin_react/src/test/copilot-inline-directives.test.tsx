import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CopilotMarkdown } from "@/modules/copilot/ui/copilot-markdown";

const navigate = vi.fn();
vi.mock("@/modules/copilot/hooks/use-citation-nav", () => ({ useCitationNav: () => navigate }));

describe("Copilot inline directives", () => {
  beforeEach(() => navigate.mockReset());

  it("renders compact references within a sentence and navigates using their actual identifiers", () => {
    const { container } = render(<CopilotMarkdown text={'Compare :aladin-ticker[NVIDIA]{symbol="nvda"} with :aladin-entity[NVIDIA Corp.]{id="entity-1"}, then open :aladin-artifact[my thesis]{id="page-1" kind="page"}.'} />);

    expect(container.querySelector("p")?.textContent).toBe("Compare NVIDIA with NVIDIA Corp., then open my thesis.");
    for (const button of screen.getAllByRole("button")) {
      expect(button.classList.contains("inline")).toBe(true);
      expect(button.classList.contains("w-full")).toBe(false);
      expect(button.closest("p")).toBeTruthy();
      fireEvent.click(button);
    }
    expect(navigate.mock.calls.map(([citation]) => citation)).toEqual([
      { kind: "ticker", id: "NVDA", title: "NVDA" },
      { kind: "entity", id: "entity-1", title: "NVIDIA Corp." },
      { kind: "page", id: "page-1", title: "my thesis" },
    ]);
    expect(container.querySelector("p div, p p, button button")).toBeNull();
  });

  it("uses symbol/title fallbacks without requiring a label", () => {
    render(<CopilotMarkdown text={':aladin-ticker{symbol="qqq"} and :aladin-artifact{id="shard-1" kind="app" title="Research"} and :aladin-entity{id="entity-1" title="Company"}'} />);
    fireEvent.click(screen.getByRole("button", { name: "QQQ" }));
    fireEvent.click(screen.getByRole("button", { name: "Research" }));
    fireEvent.click(screen.getByRole("button", { name: "Company" }));
    expect(navigate.mock.calls.map(([citation]) => citation.kind)).toEqual(["ticker", "shard", "entity"]);
  });

  it("supports inline references in headings, emphasis, lists and table cells", () => {
    const { container } = render(<CopilotMarkdown text={'## Tracking :aladin-ticker{symbol="SPY"}\n\n**Watch :aladin-ticker{symbol="QQQ"}**\n\n- Compare :aladin-ticker{symbol="IWM"}.\n\n| Name |\n| --- |\n| :aladin-artifact[Note]{id="p1" kind="page"} |'} />);
    expect(screen.getByRole("button", { name: "SPY" }).closest("h2")).toBeTruthy();
    expect(screen.getByRole("button", { name: "QQQ" }).closest("strong")).toBeTruthy();
    expect(screen.getByRole("button", { name: "IWM" }).closest("li")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Note" }).closest("td")).toBeTruthy();
    expect(container.querySelector("h2 div, strong div, p div")).toBeNull();
  });

  it("keeps standalone double-colon references as cards", () => {
    render(<CopilotMarkdown text={'::aladin-ticker{symbol="SPY"}\n\n::aladin-artifact{id="p1" kind="page" title="Note"}\n\n::aladin-entity{id="e1" title="Company"}'} />);
    for (const button of screen.getAllByRole("button")) {
      expect(button.classList.contains("w-full")).toBe(true);
      expect(button.closest("p")).toBeNull();
    }
  });

  it.each([
    ':aladin-artifact[Missing ID]',
    ':aladin-ticker{symbol="<script>"}',
    ':aladin-approval{action="Publish" target="Note"}',
    ':aladin-diff{title="Changes"}',
    ':aladin-actions',
    ':aladin-unknown[label]',
  ])("keeps invalid or block-only inline syntax inert: %s", (directive) => {
    const { container } = render(<CopilotMarkdown text={`Before ${directive} after.`} />);
    expect(container.querySelector("p")?.textContent).toBe(`Before ${directive} after.`);
    expect(screen.queryByRole("button")).toBeNull();
    expect(container.querySelector("p div, p p, script")).toBeNull();
  });

  it("does not nest a clickable directive inside a markdown link", () => {
    render(<CopilotMarkdown text={'[See :aladin-ticker{symbol="SPY"}](https://example.com)'} />);
    expect(screen.getByRole("link").textContent).toContain(':aladin-ticker{symbol="SPY"}');
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("does not activate directive examples in code", () => {
    render(<CopilotMarkdown text={'`:aladin-ticker{symbol="SPY"}`\n\n```\n:aladin-artifact[Note]{id="p1"}\n::aladin-ticker{symbol="QQQ"}\n```'} />);
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.getByText(':aladin-ticker{symbol="SPY"}').tagName).toBe("CODE");
  });

  it("handles an unfinished streamed directive without inserting a block into prose", () => {
    const text = 'Watch :aladin-ticker[NVIDIA]{symbol="NVDA"} today.';
    const { container, rerender } = render(<CopilotMarkdown text="" />);
    for (let i = 1; i <= text.length; i += 1) {
      rerender(<CopilotMarkdown text={text.slice(0, i)} />);
      expect(container.querySelector("p div, p p, button button")).toBeNull();
    }
    expect(container.querySelector("p")?.textContent).toBe("Watch NVIDIA today.");
    expect(screen.getByRole("button", { name: "NVIDIA" })).toBeTruthy();
  });
});
