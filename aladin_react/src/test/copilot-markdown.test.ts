import { describe, expect, it } from "vitest";

import { parseActivityItems, parseCopilotMarkdown } from "@/modules/copilot/ui/copilot-markdown";

describe("parseCopilotMarkdown", () => {
  it("splits markdown around leaf directives", () => {
    expect(
      parseCopilotMarkdown(
        'Before\n\n::aladin-artifact{id="p1" kind="page" title="NVDA thesis"}\n\nAfter',
      ),
    ).toEqual([
      { kind: "markdown", text: "Before\n" },
      {
        kind: "directive",
        name: "aladin-artifact",
        attrs: { id: "p1", kind: "page", title: "NVDA thesis" },
        body: "",
      },
      { kind: "markdown", text: "\nAfter" },
    ]);
  });

  it("parses closed container directives and leaves unclosed ones as markdown", () => {
    expect(parseCopilotMarkdown('::aladin-activity\n[{"label":"Searched","status":"ok"}]\n::')).toEqual([
      {
        kind: "directive",
        name: "aladin-activity",
        attrs: {},
        body: '[{"label":"Searched","status":"ok"}]',
      },
    ]);

    expect(parseCopilotMarkdown("::aladin-activity\n[")).toEqual([
      { kind: "markdown", text: "::aladin-activity\n[" },
    ]);
  });

  it("keeps action directives as native directive segments", () => {
    expect(parseCopilotMarkdown('::aladin-actions\n[{"label":"Compare","prompt":"Compare this"}]\n::')).toEqual([
      {
        kind: "directive",
        name: "aladin-actions",
        attrs: {},
        body: '[{"label":"Compare","prompt":"Compare this"}]',
      },
    ]);
  });
});

describe("parseActivityItems", () => {
  it("accepts rich activity detail fields and caps noisy text", () => {
    const items = parseActivityItems(
      JSON.stringify({
        items: [
          {
            label: "Built shard",
            status: "error",
            inputSummary: "pageId: p1",
            resultSummary: "Build failed",
            detail: "x".repeat(540),
            finishedAt: "12:04",
          },
        ],
      }),
    );

    expect(items[0]).toMatchObject({
      label: "Built shard",
      status: "error",
      inputSummary: "pageId: p1",
      resultSummary: "Build failed",
      time: "12:04",
    });
    expect(items[0]?.detail).toHaveLength(501);
    expect(items[0]?.detail?.endsWith("…")).toBe(true);
  });

  it("drops invalid items and limits the rendered list", () => {
    const raw = Array.from({ length: 20 }, (_, i) => ({ label: i === 0 ? "" : `Step ${i}`, status: "ok" }));
    const items = parseActivityItems(JSON.stringify(raw));
    expect(items).toHaveLength(11);
    expect(parseActivityItems("{")).toEqual([]);
  });
});
