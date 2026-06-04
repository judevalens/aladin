// Hocuspocus collaboration server + the M8c MCP admin bridge.
//
// Owns the in-memory Y.Doc per page, syncs to connected clients, and persists
// the full Y.Doc binary to page_ydoc via @hocuspocus/extension-database
// (debounced store). M8c adds three server-side capabilities on top:
//
//   - applyOperation(input): the MCP write path. Opens a server-side
//     DirectConnection to the live Y.Doc, applies a block op, and returns the
//     affected blocks rendered to markdown. Because it mutates the same
//     in-memory doc the editors hold, the edit broadcasts to every connected
//     client live — no reload.
//   - getPageContent(pageId): the MCP fresh-read path (get_page) — materializes
//     the live Y.Doc to blocks + markdown.
//   - a debounced JSON projection (M8.7): on every doc change, coalesce writes
//     of page_documents.blocks / search_text so search / list / cold reads see
//     a current view without materializing a Y.Doc per query.
//
// Block ops are read-modify-write at the blocks-array level: read the current
// blocks (ids intact), splice the change in JS, then rebuild the "document"
// XmlFragment via @blocknote/server-util. server-util has no fine-grained
// "apply one op preserving history" call — blocksToYXmlFragment is a full
// rebuild — but untouched blocks keep their ids (verified), so id-targeting is
// stable. The cost is a coarse Yjs update: a concurrent human edit landing
// during an agent write resolves last-write-wins at the fragment level. That's
// the accepted consensus tradeoff for low-frequency agent edits, single
// instance. DirectConnection exposes transact()/disconnect() only (no
// awareness API), so the agent does not get a presence cursor — deferred.

import { Server } from "@hocuspocus/server";
import { Database } from "@hocuspocus/extension-database";
import { ServerBlockNoteEditor } from "@blocknote/server-util";
import { BadRequest, errorMessage } from "../errors.js";

// BlockNote binds its ProseMirror doc to a Y.XmlFragment named "document".
const FRAGMENT = "document";

const VALID_OPS = new Set([
  "replace_all",
  "replace_block",
  "insert_blocks",
  "delete_block",
]);

// One headless editor, reused for all conversions (same pattern as converter).
const editor = ServerBlockNoteEditor.create();

const clamp = (n, lo, hi) => Math.max(lo, Math.min(hi, n));

// Flatten a block tree to plain text for the search_text projection column.
function blocksToSearchText(blocks) {
  const parts = [];
  const walk = (bs) => {
    for (const b of bs ?? []) {
      if (Array.isArray(b.content)) {
        for (const c of b.content) {
          if (c && typeof c.text === "string") parts.push(c.text);
        }
      }
      if (Array.isArray(b.children) && b.children.length) walk(b.children);
    }
  };
  walk(blocks);
  return parts.join(" ").replace(/\s+/g, " ").trim();
}

// Pure planner: given the current blocks and an op, produce the next blocks
// array plus the [start,count) range of blocks the op affected (for the
// response markdown). Throws BadRequest on malformed input / missing ids.
function planOp(current, input) {
  const { op, block_id: blockId, position, placement, blocks } = input;
  const incoming = Array.isArray(blocks) ? blocks : [];
  switch (op) {
    case "replace_all":
      if (!Array.isArray(blocks)) {
        throw new BadRequest("replace_all requires blocks[]");
      }
      return { next: incoming, affectedStart: 0, affectedCount: incoming.length };
    case "replace_block": {
      if (!blockId) throw new BadRequest("replace_block requires block_id");
      const idx = current.findIndex((b) => b.id === blockId);
      if (idx === -1) throw new BadRequest(`block not found: ${blockId}`);
      return {
        next: [...current.slice(0, idx), ...incoming, ...current.slice(idx + 1)],
        affectedStart: idx,
        affectedCount: incoming.length,
      };
    }
    case "insert_blocks": {
      if (incoming.length === 0) {
        throw new BadRequest("insert_blocks requires blocks[]");
      }
      let at;
      if (blockId) {
        const idx = current.findIndex((b) => b.id === blockId);
        if (idx === -1) throw new BadRequest(`block not found: ${blockId}`);
        at = placement === "before" ? idx : idx + 1;
      } else {
        at =
          typeof position === "number"
            ? clamp(position, 0, current.length)
            : current.length;
      }
      return {
        next: [...current.slice(0, at), ...incoming, ...current.slice(at)],
        affectedStart: at,
        affectedCount: incoming.length,
      };
    }
    case "delete_block": {
      if (!blockId) throw new BadRequest("delete_block requires block_id");
      const idx = current.findIndex((b) => b.id === blockId);
      if (idx === -1) throw new BadRequest(`block not found: ${blockId}`);
      // A BlockNote doc must keep at least one block; refuse to empty it.
      if (current.length <= 1) {
        throw new BadRequest("cannot delete the last block on a page");
      }
      return {
        next: current.filter((b) => b.id !== blockId),
        affectedStart: 0,
        affectedCount: 0,
      };
    }
    default:
      throw new BadRequest(`unknown op: ${op}`);
  }
}

export function createCollabServer({
  pool,
  resolveToken,
  port,
  debounceMs,
  maxDebounceMs,
  projectionDebounceMs,
  projectionMaxCommits,
  projectionSweepMs,
}) {
  // M8.7 projection debounce: pageId -> { timer, pendingCommits, document }.
  const projectionDebounce = projectionDebounceMs ?? 500;
  const projectionCommitCap = projectionMaxCommits ?? 50;
  const pending = new Map();

  async function writeProjection(pageId, document) {
    const blocks = editor.yDocToBlocks(document, FRAGMENT);
    const blocksJson = JSON.stringify(blocks);
    const searchText = blocksToSearchText(blocks);
    await pool.query(
      `INSERT INTO page_documents
           (artifact_id, blocks, search_text, last_collab_commit_at, revision, created_at, updated_at)
         VALUES ($1, $2::jsonb, $3, now(), 1, now(), now())
       ON CONFLICT (artifact_id) DO UPDATE
           SET blocks = EXCLUDED.blocks,
               search_text = EXCLUDED.search_text,
               last_collab_commit_at = now(),
               revision = page_documents.revision + 1,
               updated_at = now()`,
      [pageId, blocksJson, searchText],
    );
  }

  // Block-level agent presence: record which agent last touched each affected
  // block into page_documents.block_attribution — a side map { blockId: { by,
  // at } }, pruned to the page's current block ids so deleted blocks drop out.
  // The bridge is the only writer that knows "an agent did this" (human edits
  // ride Yjs and aren't stamped). Best-effort: a failure must not break the op.
  async function stampAttribution(pageId, affectedIds, currentIds, editorName) {
    if (!Array.isArray(affectedIds) || affectedIds.length === 0) return;
    const { rows } = await pool.query(
      "SELECT block_attribution FROM page_documents WHERE artifact_id = $1",
      [pageId],
    );
    const current = rows[0]?.block_attribution ?? {};
    const keep = new Set(currentIds);
    const next = {};
    for (const [id, v] of Object.entries(current)) {
      if (keep.has(id)) next[id] = v;
    }
    const at = new Date().toISOString();
    for (const id of affectedIds) next[id] = { by: editorName, at };
    await pool.query(
      `INSERT INTO page_documents (artifact_id, block_attribution, created_at, updated_at)
         VALUES ($1, $2::jsonb, now(), now())
       ON CONFLICT (artifact_id) DO UPDATE
         SET block_attribution = EXCLUDED.block_attribution, updated_at = now()`,
      [pageId, JSON.stringify(next)],
    );
  }

  // --- page edit history (coalesced sessions) -----------------------------
  // The collab server is the one place that sees every edit. Humans record via
  // onChange (the connection's principal); agents record via applyOperation.
  // Edits by the same editor on the same page within HISTORY_WINDOW_MS extend a
  // single session row (occurred_at=start, ended_at=last, edits=count) instead
  // of one row per keystroke. All best-effort — never block an edit.
  const HISTORY_WINDOW_MS = 45000;
  const editSessions = new Map(); // key -> { rowId, edits, lastChange, timer }

  async function closeEditSession(key) {
    const s = editSessions.get(key);
    if (!s) return;
    editSessions.delete(key);
    clearTimeout(s.timer);
    try {
      await pool.query(
        "UPDATE page_edit_history SET ended_at = $2, edits = $3 WHERE id = $1",
        [s.rowId, s.lastChange.toISOString(), s.edits],
      );
    } catch (err) {
      console.error("history close failed:", errorMessage(err));
    }
  }

  // Record one edit by `editor` ({ kind, name }) on `pageId`.
  function recordEdit(pageId, editor) {
    if (!editor || !editor.name) return;
    const key = `${pageId}::${editor.kind}:${editor.name}`;
    const now = new Date();
    const existing = editSessions.get(key);
    if (existing) {
      existing.edits += 1;
      existing.lastChange = now;
      clearTimeout(existing.timer);
      existing.timer = setTimeout(() => closeEditSession(key), HISTORY_WINDOW_MS);
      return;
    }
    pool
      .query(
        `INSERT INTO page_edit_history (page_id, editor_kind, editor_name)
         VALUES ($1, $2, $3) RETURNING id`,
        [pageId, editor.kind, editor.name],
      )
      .then(({ rows }) => {
        const timer = setTimeout(() => closeEditSession(key), HISTORY_WINDOW_MS);
        editSessions.set(key, { rowId: rows[0].id, edits: 1, lastChange: now, timer });
      })
      .catch((err) => console.error(`history insert failed for ${pageId}:`, errorMessage(err)));
  }

  // The human editor behind a connection context (onAuthenticate sets
  // { principal }). Agents edit via the bridge (no principal on that direct
  // connection) and are recorded separately, so a missing principal → skip.
  function humanEditorFromContext(context) {
    const p = context?.principal;
    if (!p) return null;
    return { kind: "human", name: String(p.email || p.userId || p.actorId || "user") };
  }

  function flushProjection(pageId) {
    const entry = pending.get(pageId);
    if (!entry) return;
    clearTimeout(entry.timer);
    pending.delete(pageId);
    // Fire-and-forget — a projection failure must never crash the broker or
    // block the WebSocket event loop. Search/list go stale; the next commit
    // (or the periodic sweep) retries.
    writeProjection(pageId, entry.document).catch((err) =>
      console.error(`projection write failed for ${pageId}:`, errorMessage(err)),
    );
  }

  function scheduleProjection(pageId, document) {
    const entry = pending.get(pageId);
    if (!entry) {
      pending.set(pageId, {
        document,
        pendingCommits: 1,
        timer: setTimeout(() => flushProjection(pageId), projectionDebounce),
      });
      return;
    }
    entry.document = document;
    entry.pendingCommits += 1;
    // Commit-cap: a burst of edits flushes without waiting out the timer.
    if (entry.pendingCommits >= projectionCommitCap) {
      flushProjection(pageId);
    }
  }

  // Periodic sweep: flush any entry whose timer was orphaned by a process
  // restart mid-debounce (M8.1 failure-handling contract, item 4).
  const sweep = setInterval(() => {
    for (const pageId of [...pending.keys()]) flushProjection(pageId);
  }, projectionSweepMs ?? 5000);
  if (typeof sweep.unref === "function") sweep.unref();

  const server = new Server({
    port: port ?? 3501,

    // Write-efficiency knobs only — durability lives on the clients +
    // y-indexeddb. See the plan's durability model.
    debounce: debounceMs ?? 2000,
    maxDebounce: maxDebounceMs ?? 10000,

    extensions: [
      new Database({
        // documentName is the Aladin page id.
        fetch: async ({ documentName }) => {
          const { rows } = await pool.query(
            "SELECT state FROM page_ydoc WHERE page_id = $1",
            [documentName],
          );
          // BYTEA comes back as a Buffer (a Uint8Array); null => new doc.
          // Unknown page id => empty doc; no legacy-JSON hydration (M8.2 wipe).
          return rows[0]?.state ?? null;
        },
        store: async ({ documentName, state }) => {
          await pool.query(
            `INSERT INTO page_ydoc (page_id, state, updated_at)
                 VALUES ($1, $2, now())
             ON CONFLICT (page_id)
                 DO UPDATE SET state = EXCLUDED.state, updated_at = now()`,
            [documentName, state],
          );
        },
      }),
    ],

    // Returned value becomes the connection context (read by awareness +
    // write-authz). Throwing rejects the connection.
    onAuthenticate: async ({ token }) => {
      const principal = await resolveToken(token);
      return { principal };
    },

    // M8.7: every doc change schedules a debounced projection write. Fires for
    // edits from clients AND from the bridge's own DirectConnection writes.
    onChange: async ({ documentName, document, context }) => {
      scheduleProjection(documentName, document);
      // Page history: attribute human edits to the connection's principal.
      recordEdit(documentName, humanEditorFromContext(context));
    },
  });

  // --- M8c bridge: server-side writes/reads via DirectConnection ----------

  // Apply a block operation to a page's live Y.Doc. Mutating the in-memory doc
  // broadcasts to all connected editors. Returns the affected blocks' markdown
  // and the full current block-id list (so the agent can target follow-ups
  // without a re-read).
  async function applyOperation(input) {
    const pageId = input?.page_id;
    if (!pageId) throw new BadRequest("page_id is required");
    // Validate the op before opening a connection so a malformed request
    // doesn't load (and immediately tear down) a live Y.Doc.
    if (!VALID_OPS.has(input?.op)) {
      throw new BadRequest(`unknown op: ${input?.op}`);
    }

    const connection = await server.hocuspocus.openDirectConnection(pageId);
    try {
      let plan;
      await connection.transact((document) => {
        const fragment = document.getXmlFragment(FRAGMENT);
        const current = editor.yXmlFragmentToBlocks(fragment);
        plan = planOp(current, input);
        fragment.delete(0, fragment.length);
        editor.blocksToYXmlFragment(plan.next, fragment);
      });

      // Read back post-mutation to get ids assigned to any new blocks.
      const written = editor.yDocToBlocks(connection.document, FRAGMENT);
      const affected = written.slice(
        plan.affectedStart,
        plan.affectedStart + plan.affectedCount,
      );
      const markdown = await editor.blocksToMarkdownLossy(affected);

      // Stamp block-level agent attribution (best-effort). `agent` is the
      // bridge's { id, name } object (or absent); prefer name, then id.
      const agent = input.agent;
      const editorName =
        (agent && typeof agent === "object"
          ? String(agent.name || agent.id || "").trim()
          : typeof agent === "string"
            ? agent.trim()
            : "") || "agent";
      try {
        await stampAttribution(
          pageId,
          affected.map((b) => b.id),
          written.map((b) => b.id),
          editorName,
        );
      } catch (err) {
        console.error(
          `attribution stamp failed for ${pageId}:`,
          errorMessage(err),
        );
      }

      // Page history: record the agent edit (coalesced, like human edits).
      recordEdit(pageId, { kind: "agent", name: editorName });

      return {
        blockIds: written.map((b) => b.id),
        affectedBlockIds: affected.map((b) => b.id),
        markdown,
      };
    } finally {
      try {
        await connection.disconnect();
      } catch (err) {
        console.error(
          `bridge disconnect failed for ${pageId}:`,
          errorMessage(err),
        );
      }
    }
  }

  // Materialize a page's live Y.Doc to blocks + markdown (the fresh get_page
  // read — the precursor to editing wants a current baseline).
  async function getPageContent(pageId) {
    if (!pageId) throw new BadRequest("page_id is required");
    const connection = await server.hocuspocus.openDirectConnection(pageId);
    try {
      const blocks = editor.yDocToBlocks(connection.document, FRAGMENT);
      const markdown = await editor.blocksToMarkdownLossy(blocks);
      return { blocks, markdown };
    } finally {
      try {
        await connection.disconnect();
      } catch (err) {
        console.error(
          `bridge disconnect failed for ${pageId}:`,
          errorMessage(err),
        );
      }
    }
  }

  return {
    hocuspocus: server.hocuspocus,
    listen: (...args) => server.listen(...args),
    get address() {
      return server.address;
    },
    applyOperation,
    getPageContent,
    async destroy() {
      clearInterval(sweep);
      for (const entry of pending.values()) clearTimeout(entry.timer);
      pending.clear();
      await server.destroy();
    },
  };
}
