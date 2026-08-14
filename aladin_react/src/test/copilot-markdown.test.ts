import { describe, expect, it } from "vitest";

import {
  parseActionItems,
  parseActivityItems,
  parseApprovalBlock,
  parseCopilotMarkdown,
  parseDiffBlock,
  parseErrorRecoveryBlock,
  parseShardPreviewBlock,
} from "@/modules/copilot/ui/copilot-markdown";

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

  it("accepts punctuation immediately after leaf directives", () => {
    expect(
      parseCopilotMarkdown(
        '::aladin-artifact{id="artifact-e5eb2565-2dee-44a2-b759-902adbd6e167" kind="shard" title="Day Trading Playbook"}:',
      ),
    ).toEqual([
      {
        kind: "directive",
        name: "aladin-artifact",
        attrs: {
          id: "artifact-e5eb2565-2dee-44a2-b759-902adbd6e167",
          kind: "shard",
          title: "Day Trading Playbook",
        },
        body: "",
      },
      { kind: "markdown", text: ":" },
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

describe("parseActionItems", () => {
  it("supports native follow-up actions with deterministic fallback prompts", () => {
    expect(
      parseActionItems(
        JSON.stringify([
          { action: "continue", label: "Continue" },
          { action: "retry", label: "Retry" },
        ]),
      ),
    ).toEqual([
      { action: "continue", label: "Continue", prompt: "continue", target: "continue" },
      { action: "retry", label: "Retry", prompt: "try again", target: "try again" },
    ]);
  });

  it("validates open actions and accepts artifact id aliases", () => {
    expect(
      parseActionItems(
        JSON.stringify([
          { action: "open_artifact", label: "Open shard", id: "shd_1", kind: "app" },
          { action: "open_ticker", label: "Open NVDA", symbol: "nvda" },
          { action: "open_ticker", label: "Bad", symbol: "NVDA<script>" },
        ]),
      ),
    ).toEqual([
      { action: "open_artifact", label: "Open shard", id: "shd_1", kind: "shard", target: "shd_1" },
      { action: "open_ticker", label: "Open NVDA", symbol: "NVDA", target: "NVDA" },
    ]);
  });
});

describe("parseApprovalBlock", () => {
  it("parses approval cards from JSON body and caps detail rows", () => {
    const block = parseApprovalBlock(
      {},
      JSON.stringify({
        action: "Publish shard",
        target: "Collar payoff",
        status: "pending",
        risk: "Makes this shard visible in the workspace.",
        details: ["bundle: dist/index.js", "x".repeat(250), "extra"],
      }),
    );

    expect(block).toMatchObject({
      action: "Publish shard",
      target: "Collar payoff",
      status: "pending",
      risk: "Makes this shard visible in the workspace.",
    });
    expect(block?.details).toHaveLength(3);
    expect(block?.details[1]).toHaveLength(221);
    expect(block?.details[1]?.endsWith("…")).toBe(true);
  });

  it("supports leaf attributes and rejects malformed cards", () => {
    expect(
      parseApprovalBlock({ action: "Delete file", target: "old.tsx", status: "approved" }, ""),
    ).toEqual({
      action: "Delete file",
      target: "old.tsx",
      status: "approved",
      details: [],
    });
    expect(parseApprovalBlock({ action: "Delete file" }, "")).toBeNull();
    expect(parseApprovalBlock({}, "{")).toBeNull();
  });
});

describe("parseDiffBlock", () => {
  it("parses JSON diff blocks with bounded lines and stats", () => {
    const lines = Array.from({ length: 90 }, (_, i) => ({ kind: i % 2 === 0 ? "add" : "remove", text: `line ${i}` }));
    const diff = parseDiffBlock(
      {},
      JSON.stringify({
        title: "Update shard",
        path: "src/index.tsx",
        lines,
      }),
    );

    expect(diff).toMatchObject({ title: "Update shard", path: "src/index.tsx", added: 40, removed: 40 });
    expect(diff?.lines).toHaveLength(80);
  });

  it("parses unified diff bodies and ignores file headers", () => {
    expect(
      parseDiffBlock(
        { title: "Patch" },
        ["--- a/src/index.tsx", "+++ b/src/index.tsx", " import React from 'react';", "-old", "+new"].join("\n"),
      ),
    ).toEqual({
      title: "Patch",
      added: 1,
      removed: 1,
      lines: [
        { kind: "context", text: "import React from 'react';" },
        { kind: "remove", text: "old" },
        { kind: "add", text: "new" },
      ],
    });
    expect(parseDiffBlock({}, "")).toBeNull();
  });
});

describe("parseShardPreviewBlock", () => {
  it("parses ready shard previews from attrs and safe preview paths", () => {
    expect(
      parseShardPreviewBlock(
        { artifactId: "shd_1", title: "Collar payoff", status: "published" },
        JSON.stringify({ previewUrl: "/preview/shd_1" }),
      ),
    ).toEqual({
      id: "shd_1",
      title: "Collar payoff",
      status: "published",
      subtitle: "preview: /preview/shd_1",
      diagnostics: [],
    });
  });

  it("shows failed builds with bounded diagnostics and rejects unsafe URLs", () => {
    const preview = parseShardPreviewBlock(
      {},
      JSON.stringify({
        pageId: "p1",
        title: "Broken shard",
        buildOk: false,
        previewUrl: "https://example.com/preview",
        diagnostics: ["src/index.tsx:12 Missing prop", "x".repeat(260), "third"],
      }),
    );

    expect(preview).toMatchObject({
      id: "p1",
      title: "Broken shard",
      status: "error",
      subtitle: "build needs attention",
    });
    expect(preview?.diagnostics).toHaveLength(3);
    expect(preview?.diagnostics[1]).toHaveLength(221);
    expect(preview?.diagnostics[1]?.endsWith("…")).toBe(true);
  });

  it("rejects malformed preview bodies and empty titles", () => {
    expect(parseShardPreviewBlock({}, "{")).toBeNull();
    expect(parseShardPreviewBlock({}, "{}")).toBeNull();
  });
});

describe("parseErrorRecoveryBlock", () => {
  it("parses recovery messages with validated native actions", () => {
    expect(
      parseErrorRecoveryBlock(
        JSON.stringify({
          title: "Build failed",
          message: "The shard could not compile.",
          code: "BUILD_FAILED",
          actions: [
            { action: "retry", label: "Retry build", prompt: "retry the build" },
            { action: "open_artifact", label: "Open shard", artifactId: "shd_1", kind: "shard" },
            { action: "open_ticker", label: "Bad ticker", symbol: "<script>" },
          ],
        }),
      ),
    ).toEqual({
      title: "Build failed",
      message: "The shard could not compile.",
      code: "BUILD_FAILED",
      actions: [
        { action: "retry", label: "Retry build", prompt: "retry the build", target: "retry the build" },
        { action: "open_artifact", label: "Open shard", id: "shd_1", kind: "shard", target: "shd_1" },
      ],
    });
  });

  it("supports a deterministic retry prompt fallback and rejects malformed blocks", () => {
    expect(
      parseErrorRecoveryBlock(JSON.stringify({ message: "Timed out.", retryPrompt: "continue after timeout" })),
    ).toEqual({
      title: "Couldn’t complete that",
      message: "Timed out.",
      actions: [{ action: "retry", label: "Try again", prompt: "continue after timeout", target: "continue after timeout" }],
    });
    expect(parseErrorRecoveryBlock("{")).toBeNull();
    expect(parseErrorRecoveryBlock(JSON.stringify({ title: "No message" }))).toBeNull();
  });
});
