# Copilot Surface Polish Plan

## Goal

Make Copilot feel like a native Aladin agent surface rather than a chat pane that
happens to call tools.

The work has two tracks:

- reliability polish: turn lifecycle, approvals, reconnects, drafts, and state
- presentation polish: rich rendering, activity timeline, context cards, diffs,
  shard previews, and action affordances

The core principle: the model may produce prose and declarative directives, but
Aladin owns all rendering, permissions, and actions.

## Implementation Status

- Phase 1 is partially implemented: turn events are thread/session scoped, early
  realtime events can bind the active turn, pending approvals survive thread
  switches, queued follow-ups are scoped to their target thread/surface,
  reconnect state is visible, and watchdog reconciliation remains in place.
- Phase 2 is implemented: the composer has stable autogrow sizing, no focus-only
  helper row, fixed send/stop controls, queued follow-up visibility, and
  per-thread drafts.
- Phase 3 has a stronger first cut: the thread menu now has search, a
  new-thread action, active/running/approval states, relative saved times,
  persisted rename, soft archive, and pin/unpin ordering. Richer surface
  grouping remains.
- Phase 4 has a stronger first cut: live tool chips are now a compact activity
  timeline, completed-turn footers use readable tool labels, and tool events can
  carry bounded/redacted input and result summaries for expandable detail rows.
- Phase 5 is implemented as a local parser: `aladin-artifact`,
  `aladin-entity`, `aladin-ticker`, `aladin-activity`, and `aladin-actions`
  render native UI; unknown or malformed directives degrade to inert markdown
  text. Actions currently support send-prompt, open-artifact, and open-ticker.
- Phase 6 has a first cut: the current surface resolves active artifact
  title/type from the workspace tree, so composer copy and suggestions can use
  the actual page/shard/source context.
- Phase 7 is started: persisted `aladin-activity` blocks can render rich,
  bounded detail fields such as input summaries, result summaries, detail text,
  and timestamps; `aladin-actions` supports validated open actions plus
  deterministic continue/retry/send-prompt follow-ups.
- Phase 8 is started: persisted `aladin-approval` blocks can render bounded,
  native approval cards with action, target, risk, status, and exact detail rows;
  persisted `aladin-diff` blocks can render bounded, display-only change previews
  from JSON or unified diff text; persisted `aladin-shard-preview` blocks can
  render bounded build/preview status, diagnostics, and an open-shard action;
  persisted `aladin-error-recovery` blocks can render bounded recovery messages
  with validated follow-up/open actions.
- Phase 9 is started: rich directive parser tests now have render-level coverage
  for native block rendering, validated actions, and malformed fallback behavior;
  live activity timeline tests cover grouped tool runs, expandable summaries, and
  overflow counts; queued follow-up tests cover scoped thread/surface delivery;
  thread switcher tests cover search, empty states, approval/running/pinned
  badges, and row actions; proposal-card tests cover pending, in-flight, and
  settled approval states; composer surface tests cover placeholders,
  suggestions, scope summaries, and artifact-kind labels; reconnect banner tests
  cover open, reconnecting, and offline stream states; error banner tests cover
  exact error display and max-turn continue recovery; queued-follow-up banner
  tests cover visibility and clearing.
- The backend system prompt now advertises the supported rich directive contract
  and safety rules so final answers can use native blocks intentionally.

## Current Problems

The current surface works, but it exposes too much transport detail and too
little product intent.

Observed rough edges:

- turn state is global instead of per thread
- approvals can become stale after thread switches
- early realtime events can arrive before the UI knows the new session id
- queued follow-ups are a single global string, not tied to a thread or surface
- activity is reduced to terse text like `write_file x2`
- completed turn metadata renders raw tool names
- the composer says "this item" instead of the actual artifact/shard/page title
- the composer sizing jumps while typing/focusing and feels cramped
- thread management is only a dropdown, with no search, pinning, rename,
  archive, status, or surface grouping
- websocket recovery happens silently
- markdown is plain prose plus citations, not a native Aladin UI surface

## Target Experience

Copilot should show:

- what it is looking at
- what it is doing
- what changed
- what needs approval
- what failed and how to recover
- what sources or workspace objects are involved

The transcript should remain readable as markdown, but support native rich blocks:

```md
I found the key issue in the current shard.

::aladin-artifact{id="shd_123" kind="shard" title="Collar payoff"}

::aladin-activity
[
  {"label":"Read shard files","status":"ok"},
  {"label":"Built the shard","status":"error"},
  {"label":"Fixed index.tsx","status":"ok"}
]
::

The build now passes.
```

Unknown or invalid directives must degrade gracefully.

## Phase 1: Stabilize Turn Lifecycle

Fix the interaction foundation before making the UI richer.

### Work

- Move live turn state from one global slot to per-thread state.
- Preserve pending approvals across thread switches.
- Prevent approvals from expiring just because the active thread changed.
- Buffer realtime events that arrive before `beginCopilotTurn`.
- Make queued messages thread-aware and surface-aware.
- Add visible websocket/recovery states in the dock.

### Likely Files

- `aladin_react/src/app/state/copilot-slice.ts`
- `aladin_react/src/modules/copilot/hooks/use-copilot.ts`
- `aladin_react/src/shared/realtime/copilot-event-handler.ts`
- `aladin_react/src/app/composition/create-app-composition.ts`
- `aladin_react/src/modules/copilot/ui/copilot-dock-ui.tsx`

### Acceptance

- Switching threads during a running turn does not lose the live turn state.
- A pending approval remains actionable after navigating away and back.
- If a token/tool event arrives before the send response resolves, it still lands
  in the correct turn.
- A queued follow-up sends to the thread/surface it was composed for.
- Reconnect/reconcile is visible but quiet.

## Phase 2: Composer Ergonomics

Make the input feel calm, stable, and built for repeated use.

### Work

- Use a comfortable fixed minimum height instead of a one-line field.
- Autogrow after React commits input changes, capped with internal scrolling.
- Remove focus-only helper rows that resize the composer.
- Keep stop/send/queue controls fixed in place while typing.
- Preserve per-thread draft text when switching conversations.
- Add optional command affordances later through menus/tooltips, not visible
  keyboard-instruction text inside the composer.

### Acceptance

- Focusing the composer does not change its height.
- Typing the first few lines does not cause layout flicker.
- Long prompts scroll inside the textarea after the cap.
- Switching threads does not throw away an unsent draft.
- Queueing while a turn runs feels intentional and recoverable.

## Phase 3: Thread Management UX

Make Copilot conversations manageable once the product has real agent history.

### Work

- Replace the tiny thread dropdown with a thread switcher/search view.
- Show active/running/approval-needed/error states in the thread list.
- Add rename, archive/delete, pin, and "new from current surface" affordances.
- Group or filter by surface when useful: current shard/page/ticker/global.
- Preserve per-thread live state, queued follow-up, and draft.
- Add empty states for no threads, archived threads, and failed loads.

### Acceptance

- The user can find a prior Copilot thread quickly.
- A thread needing approval is obvious even when inactive.
- Starting a new thread from the current shard/page preserves that context.
- Long-running shard authoring threads do not make the global chat list messy.

## Phase 4: Rich Activity Timeline

Replace the tiny live trail with a proper agent timeline.

### Work

- Extend tool events with sanitized inputs and optional result summaries.
- Store activity items with display labels, status, timestamps, and detail.
- Render a collapsible timeline inside the in-flight turn.
- Render a compact completed-turn summary using display labels, not raw tool
  names.

### Event Shape

```ts
interface CopilotActivityEvent {
  sessionId: string;
  threadId: string;
  tool: string;
  label: string;
  status: "running" | "ok" | "error";
  inputSummary?: string;
  resultSummary?: string;
  startedAt?: string;
  finishedAt?: string;
}
```

### Acceptance

- Shard authoring reads like an understandable sequence:
  "Created shard", "Wrote index.tsx", "Build failed", "Fixed build",
  "Previewed shard", "Published shard".
- Failed tool calls are visible and expandable.
- The final footer never shows raw names like `write_file`.

## Phase 5: Markdown Directives Foundation

Add a constrained rich-rendering protocol inside assistant markdown.

### Work

- Add directive parsing to `CopilotMarkdown`.
- Support a small allowlist of Aladin directives.
- Validate directive attributes/body with schemas.
- Render unknown/invalid directives as inert fallback text.
- Keep raw HTML disabled.

### Candidate Libraries

- `remark-directive`
- `unist-util-visit`
- a small local transformer that converts directives to known nodes

### Directive Rules

- Only directives with names beginning `aladin-` are considered rich UI.
- Each directive has a strict schema.
- Directives may render data and safe app actions, but never arbitrary React,
  HTML, JavaScript, CSS, or URLs without validation.
- The persisted message remains markdown text.

### Initial Directives

```text
::aladin-artifact{id kind title}
::aladin-entity{id kind title}
::aladin-ticker{symbol}
::aladin-activity
::aladin-actions
```

### Acceptance

- Existing markdown messages render exactly as before.
- Known directives render native components.
- Bad directive JSON/attributes do not break the transcript.
- Tests cover directive parsing and fallback behavior.

## Phase 6: Native Context Cards

Make "what Copilot can see" obvious.

### Work

- Improve `useCurrentSurface` to include artifact title/type when available.
- Render a compact context chip in the composer.
- Add a popover explaining the current scope:
  current ticker, current shard/page/file, markets/watchlist, entity, or route.
- Add quick actions appropriate to the surface.

### Acceptance

- Composer says "asking about Collar payoff shard", not "asking about this item".
- The user can see and change scope before sending.
- Empty-state suggestions are based on the actual surface type.

## Phase 7: High-Value Rich Blocks

Use directives for the blocks that make Copilot feel native.

### Blocks

#### `aladin-artifact`

Renders a page/shard/file/link card with title, type, and open action.

```md
::aladin-artifact{id="p1" kind="page" title="NVDA thesis"}
```

#### `aladin-ticker`

Renders a compact quote card. The client fetches current quote data; the model
only declares the symbol.

```md
::aladin-ticker{symbol="NVDA"}
```

#### `aladin-activity`

Renders a persisted activity summary.

```md
::aladin-activity
[
  {"label":"Searched pages","status":"ok"},
  {"label":"Read NVDA thesis","status":"ok"}
]
::
```

#### `aladin-actions`

Renders deterministic action buttons such as open artifact, continue, retry, or
create follow-up.

```md
::aladin-actions
[
  {"label":"Open shard","action":"open_artifact","artifactId":"shd_123"},
  {"label":"Continue","action":"send_prompt","prompt":"continue"}
]
::
```

### Acceptance

- Rich blocks are useful but not noisy.
- Action buttons call trusted client handlers, not model-provided code.
- Directive content remains readable in raw markdown.

## Phase 8: Approval, Diff, and Shard Preview Blocks

Move the highest-risk interactions into rich UI.

### Blocks

```text
aladin-approval
aladin-diff
aladin-shard-preview
aladin-error-recovery
```

### Work

- Approval cards show target, risk, exact action, and pending state.
- Page/shard edits can show a diff before approval when available.
- Shard builds can show build status, diagnostics, and preview/open actions.
- Recoverable errors show next-step actions.

### Acceptance

- Publishing or destructive edits feel deliberate and inspectable.
- Build failures are understandable without reading raw logs first.
- The user can recover from common failures with one clear next action.

## Phase 9: QA And Polish

### Tests

- copilot slice lifecycle tests
- event buffering tests
- thread switch with pending approval test
- queued follow-up thread/surface test
- composer stable-height/autogrow tests
- thread draft preservation tests
- markdown directive render tests
- unknown directive fallback tests
- activity timeline render tests

### Visual QA

- empty dock
- simple Q&A
- long streaming answer
- long tool run
- approval wait
- build failure
- shard creation success
- reconnect during stream
- mobile/narrow dock behavior if applicable

## Open Questions

- Should rich directives be generated by the model directly, or should Go inject
  some of them from known tool metadata?
- Should activity directives be persisted in markdown, or should activity remain
  in `meta` and only render as a block client-side?
- Do we want one Copilot thread per shard/page by default, or a global assistant
  with surface-scoped turns?
- Should archived threads remain searchable globally, or only behind an explicit
  archived filter?
- Should approvals be represented as directives in the transcript, or stay as
  live ephemeral cards until resolved?

## Recommended First Cut

Implement phases 1 through 5 first.

That gives us:

- reliable turns
- reliable approvals
- reliable queues
- a stable composer
- credible thread management
- a rich-rendering foundation

Then phases 6 through 8 can add visible native UI without compounding lifecycle
bugs.
