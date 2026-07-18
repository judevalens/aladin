import { describe, expect, it } from "vitest";

import { extractEntityMentions } from "@/modules/pages/editor/entity-mention";

describe("extractEntityMentions", () => {
  it("pulls @entity occurrences with their block id, deduped by (entity, block)", () => {
    const doc = [
      {
        id: "b1",
        content: [
          { type: "text", text: "OpenAI " },
          { type: "entityMention", props: { entityId: "e-openai", label: "OpenAI", kind: "org" } },
          { type: "text", text: " and again " },
          { type: "entityMention", props: { entityId: "e-openai", label: "OpenAI", kind: "org" } },
        ],
      },
      {
        id: "b2",
        content: [
          { type: "entityMention", props: { entityId: "e-anthropic", label: "Anthropic", kind: "org" } },
        ],
        children: [
          {
            id: "b3",
            content: [
              { type: "entityMention", props: { entityId: "e-altman", label: "Sam Altman", kind: "person" } },
            ],
          },
        ],
      },
    ];

    const mentions = extractEntityMentions(doc);
    // Each mention carries its block's plain text: chips render as their labels, so the
    // snippet reads the way the block looks. This is what the Entity Context surface
    // shows as "your note".
    expect(mentions).toEqual([
      {
        entityId: "e-openai",
        blockId: "b1",
        surface: "OpenAI",
        snippet: "OpenAI OpenAI and again OpenAI",
      },
      { entityId: "e-anthropic", blockId: "b2", surface: "Anthropic", snippet: "Anthropic" },
      { entityId: "e-altman", blockId: "b3", surface: "Sam Altman", snippet: "Sam Altman" },
    ]);
  });

  it("ignores non-mention content and missing entity ids", () => {
    const doc = [
      {
        id: "b1",
        content: [
          { type: "text", text: "no mentions here" },
          { type: "entityMention", props: { entityId: "", label: "broken" } },
        ],
      },
    ];
    expect(extractEntityMentions(doc)).toEqual([]);
  });

  it("is safe on empty / malformed input", () => {
    expect(extractEntityMentions(undefined)).toEqual([]);
    expect(extractEntityMentions([])).toEqual([]);
    expect(extractEntityMentions([{ id: "b1" }])).toEqual([]);
  });
});
