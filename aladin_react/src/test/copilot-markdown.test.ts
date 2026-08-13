import { describe, expect, it } from "vitest";

import { parseCopilotMarkdown } from "@/modules/copilot/ui/copilot-markdown";

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
