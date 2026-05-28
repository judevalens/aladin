// MCP admin bridge handlers (M8c). Thin: validate the request shape, delegate
// to the collab service, return its result. The shared-secret middleware
// guards the routes; the error boundary maps thrown BadRequest → 400 and any
// other throw → 500 without crashing the process.
//
// Factory-injected with the collab wrapper so routes can reach
// applyOperation / getPageContent (the collab service owns the live Y.Docs).

import { BadRequest } from "../errors.js";

export function createCollabAdminHandlers(collab) {
  // POST /admin/apply — apply a block op to a page's live Y.Doc.
  // Body: { page_id, op, block_id?, position?, placement?, blocks?, agent? }.
  async function apply(req, res) {
    const body = req.body ?? {};
    if (!body.page_id) throw new BadRequest("page_id is required");
    if (!body.op) throw new BadRequest("op is required");
    const result = await collab.applyOperation(body);
    res.json(result);
  }

  // GET /admin/page/:id — fresh materialized read of a page's live Y.Doc.
  async function getPage(req, res) {
    const result = await collab.getPageContent(req.params.id);
    res.json(result);
  }

  return { apply, getPage };
}
