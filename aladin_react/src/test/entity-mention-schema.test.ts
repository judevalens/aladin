import { BlockNoteEditor } from "@blocknote/core";
import { describe, expect, it } from "vitest";

import { extractEntityMentions, pageSchema } from "@/modules/pages/editor/entity-mention";

describe("entityMention schema", () => {
  it("round-trips an @entity inline node through the document", () => {
    const editor = BlockNoteEditor.create({
      schema: pageSchema,
      initialContent: [
        {
          type: "paragraph",
          content: [
            { type: "text", text: "hello ", styles: {} },
            {
              type: "entityMention",
              props: { entityId: "e-openai", label: "OpenAI", kind: "org" },
            },
          ],
        },
      ],
    });

    const mentions = extractEntityMentions(editor.document);
    expect(mentions).toEqual([
      {
        entityId: "e-openai",
        blockId: expect.any(String),
        surface: "OpenAI",
        snippet: expect.stringContaining("OpenAI"),
      },
    ]);
  });

  it("inserts an @entity at the cursor and keeps it in the document", () => {
    const editor = BlockNoteEditor.create({
      schema: pageSchema,
      initialContent: [{ type: "paragraph", content: [{ type: "text", text: "x", styles: {} }] }],
    });

    const first = editor.document[0];
    editor.setTextCursorPosition(first.id, "end");
    editor.insertInlineContent([
      { type: "entityMention", props: { entityId: "e1", label: "Anthropic", kind: "org" } },
      " ",
    ]);

    const mentions = extractEntityMentions(editor.document);
    expect(mentions).toEqual([
      {
        entityId: "e1",
        blockId: first.id,
        surface: "Anthropic",
        snippet: expect.stringContaining("Anthropic"),
      },
    ]);
  });
});
