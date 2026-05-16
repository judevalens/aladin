import type { BlockNoteEditor, PartialBlock } from "@blocknote/core";

export async function markdownToBlockNoteDocument(
  editor: BlockNoteEditor,
  markdown: string,
): Promise<PartialBlock[]> {
  const blocks = await editor.tryParseMarkdownToBlocks(markdown);
  return blocks.length > 0 ? blocks : [{ type: "paragraph" }];
}

export async function loadMarkdownIntoEditor(
  editor: BlockNoteEditor,
  markdown: string,
): Promise<void> {
  const blocks = await markdownToBlockNoteDocument(editor, markdown);
  editor.replaceBlocks(editor.document, blocks);
}

export function exportEditorToMarkdown(editor: BlockNoteEditor): string {
  return editor.blocksToMarkdownLossy(editor.document);
}
