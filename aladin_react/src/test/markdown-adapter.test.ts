import { BlockNoteEditor } from "@blocknote/core";
import { describe, expect, it } from "vitest";
import {
  exportEditorToMarkdown,
  loadMarkdownIntoEditor,
} from "@/modules/pages/editor/markdown-adapter";

describe("markdown adapter", () => {
  it("loads markdown into BlockNote and exports it back", async () => {
    const editor = BlockNoteEditor.create();
    await loadMarkdownIntoEditor(editor, "# Heading\n\nBody copy");
    const markdown = exportEditorToMarkdown(editor);

    expect(markdown).toContain("Heading");
    expect(markdown).toContain("Body copy");
  });
});
