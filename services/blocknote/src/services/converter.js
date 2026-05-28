// Markdown <-> BlockNote blocks conversion. A single headless
// ServerBlockNoteEditor is created once and reused per request — it exists
// precisely for Node-side conversions and needs no DOM.
//
// Conversion failures are treated as bad input (BadRequest → 400), matching
// the M6 converter's behavior the Go client expects.

import { ServerBlockNoteEditor } from "@blocknote/server-util";
import { BadRequest, errorMessage } from "../errors.js";

const editor = ServerBlockNoteEditor.create();

export async function mdToBlocks(markdown) {
  const md = typeof markdown === "string" ? markdown : "";
  try {
    return await editor.tryParseMarkdownToBlocks(md);
  } catch (err) {
    throw new BadRequest(`md-to-blocks failed: ${errorMessage(err)}`);
  }
}

export async function blocksToMd(blocks) {
  try {
    return await editor.blocksToMarkdownLossy(blocks);
  } catch (err) {
    throw new BadRequest(`blocks-to-md failed: ${errorMessage(err)}`);
  }
}

export async function blocksToMdBatch(batches) {
  const markdowns = [];
  for (const blocks of batches) {
    markdowns.push(await blocksToMd(blocks));
  }
  return markdowns;
}
