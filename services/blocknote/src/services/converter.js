// Markdown <-> BlockNote blocks conversion. A single headless
// ServerBlockNoteEditor is created once and reused per request — it exists
// precisely for Node-side conversions and needs no DOM.
//
// Conversion failures are treated as bad input (BadRequest → 400), matching
// the M6 converter's behavior the Go client expects.

import { ServerBlockNoteEditor } from "@blocknote/server-util";
import { BadRequest, errorMessage } from "../errors.js";
import { pageSchema } from "./page-schema.js";
import { decodeLinksToRefs, encodeRefsToLinks } from "./ref-codec.js";

const editor = ServerBlockNoteEditor.create({ schema: pageSchema });

export async function mdToBlocks(markdown) {
  const md = typeof markdown === "string" ? markdown : "";
  try {
    // Decode aladin-scheme links back into @entity / #ref nodes so agents can author
    // references in markdown (they'd otherwise be plain links).
    return decodeLinksToRefs(await editor.tryParseMarkdownToBlocks(md));
  } catch (err) {
    throw new BadRequest(`md-to-blocks failed: ${errorMessage(err)}`);
  }
}

export async function blocksToMd(blocks) {
  try {
    // Encode @entity / #ref nodes as aladin-scheme links first — the lossy serializer would
    // otherwise drop them (content:"none"), hiding references from anything reading markdown.
    return await editor.blocksToMarkdownLossy(encodeRefsToLinks(blocks));
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
