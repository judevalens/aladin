# Page edit history — design (spiked)

**Goal:** a page "History" view — a button opens a list of edits (humans *and*
agents), newest first. A *list* of edit events, not full version-restore (that
would be Yjs snapshots with `gc:false`, a separate, heavier feature).

## Spike result — feasible via the collab `onChange` hook

The collab server (Hocuspocus, `services/blocknote`) is the one place that sees
**every** edit: humans via their `HocuspocusProvider` connection, agents via the
bridge's `openDirectConnection`. The `onChangePayload` carries enough to
attribute each change:

- **`context`** — the connection's context. Our `onAuthenticate` returns
  `{ principal }`, so for a **human** edit `context.principal` is the user. ✅
- **`connection?`**, **`transactionOrigin`**, `update`, `document`, `documentName`.
- **`openDirectConnection(name, context?)` accepts a context** — so the bridge can
  pass an **agent identity** that then shows up in `onChange` too.

⇒ A **single capture point** (`onChange`) can attribute both humans and agents.
Distinguish them by the context: a real `principal` ⇒ human; an agent context
(or absent principal on the bridge connection) ⇒ agent.

## Design

**Capture (collab `onChange`):**
- Resolve the editor: `context.principal` (human) or the agent context (agent).
- **Coalesce** per `(page, editor)` within a window — humans type continuously, so
  one row per editing *burst* (e.g. debounce ~30s, or extend the latest open
  row's `ended_at` + bump an edit count). Agent ops are already discrete.
- Append to `page_edit_history`.
- Pass an agent context into the bridge's `openDirectConnection({ name, ... })`
  so agent edits carry identity here (can replace or sit beside the existing
  `block_attribution` stamp).

**Storage:** `page_edit_history(id, page_id, editor_kind text /* human|agent */,
editor_name text, occurred_at timestamptz, ended_at timestamptz, edits int)`.

**Read:** `GET /api/pages/{id}/history` → chronological list (limit N).

**Client:** a **History** button in the page header → a side panel / popover
listing entries: avatar/initials, name, relative time, kind (human/agent).

## Open decisions
- Coalescing window; whether to store a per-entry edit count / change summary.
- Keep `block_attribution` (current-state per-block markers) alongside history, or
  drop it in favor of history only.
- History retention / pruning.

## Build steps
1. Migration: `page_edit_history` table.
2. Bridge (`collab.js`): `onChange` capture + coalescing; pass an agent context to
   `openDirectConnection`.
3. Go API: `GET /api/pages/{id}/history` (+ service/repo read, user-scoped).
4. Client: History button + panel + fetch hook.

## Hazards / lessons baked in
- DO NOT render history/markers by mutating the editor DOM (that froze the editor —
  ProseMirror owns it). UI reads from the API, not the editor's DOM.
- Tests that touch Postgres must use `internal/dbtest.RequireTestDSN` (never the
  dev DB).
