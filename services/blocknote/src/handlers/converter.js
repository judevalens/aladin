// Thin HTTP handlers: validate the request shape, call the service, shape
// the response. No try/catch — the error boundary (wrap) catches sync throws
// and async rejections and routes them to the error handler.

import * as converter from "../services/converter.js";
import { BadRequest } from "../errors.js";

export async function mdToBlocks(req, res) {
  const blocks = await converter.mdToBlocks(req.body?.markdown);
  res.json({ blocks });
}

export async function blocksToMd(req, res) {
  const blocks = req.body?.blocks;
  if (!Array.isArray(blocks)) {
    throw new BadRequest("blocks must be an array");
  }
  res.json({ markdown: await converter.blocksToMd(blocks) });
}

export async function blocksToMdBatch(req, res) {
  const batches = req.body?.blocks;
  if (!Array.isArray(batches)) {
    throw new BadRequest("blocks must be an array of arrays");
  }
  for (const blocks of batches) {
    if (!Array.isArray(blocks)) {
      throw new BadRequest("each element must be an array of blocks");
    }
  }
  res.json({ markdowns: await converter.blocksToMdBatch(batches) });
}
