import { randomUUID } from "node:crypto";

// The approval gate: gated tools are held open inside the SDK's canUseTool
// callback until the Go API relays the user's approve/reject (or the hold
// times out). The turn keeps streaming — the dock shows the proposal card
// while the agent waits.

// createApproval registers a pending approval on the turn and returns
// {approvalId, promise}. The promise resolves {approved, timedOut, aborted}.
export function createApproval(turn, timeoutMs) {
  const approvalId = randomUUID();
  const promise = new Promise((resolve) => {
    const timer = setTimeout(() => {
      turn.approvals.delete(approvalId);
      resolve({ approved: false, timedOut: true, aborted: false });
    }, timeoutMs);
    turn.approvals.set(approvalId, { resolve, timer });
  });
  return { approvalId, promise };
}

// resolveApproval settles a held approval. Returns false when the approval is
// unknown (already resolved, timed out, or a bogus id).
export function resolveApproval(turn, approvalId, approved) {
  const pending = turn.approvals.get(approvalId);
  if (!pending) return false;
  turn.approvals.delete(approvalId);
  clearTimeout(pending.timer);
  pending.resolve({ approved, timedOut: false, aborted: false });
  return true;
}

// makeCanUseTool builds the SDK permission callback for one turn: non-gated
// tools pass straight through; gated tools emit proposed_action, hold until
// approve/reject/timeout, then emit approval_resolved.
export function makeCanUseTool({ turn, gatedTools, emit, timeoutMs }) {
  const gated = new Set(gatedTools ?? []);
  return async (toolName, input, options) => {
    const name = stripMcpPrefix(toolName);
    if (!gated.has(name)) {
      return { behavior: "allow", updatedInput: input };
    }
    const { approvalId, promise } = createApproval(turn, timeoutMs);
    emit({ type: "proposed_action", approvalId, tool: name, input });
    const { approved, timedOut, aborted } = await promise;
    if (aborted) {
      // Turn is being torn down — deny quietly, no approval_resolved event
      // (the stream is already closing).
      return { behavior: "deny", message: "The turn was cancelled.", interrupt: true };
    }
    emit({ type: "approval_resolved", approvalId, approved, timedOut });
    if (approved) {
      // Remember the tool_use id so the tool's eventual result event carries
      // this approvalId (the dock resolves its proposal card on action_result).
      if (options?.toolUseID) {
        turn.approvalByToolUse.set(options.toolUseID, approvalId);
      }
      return { behavior: "allow", updatedInput: input };
    }
    return {
      behavior: "deny",
      message: timedOut
        ? "The user did not respond to the approval request in time. Tell them it expired; they can ask again."
        : "The user declined this action. Do not retry it; tell the user it was dismissed.",
    };
  };
}

// Tool names arrive as mcp__<server>__<tool>; the Go side (labels, gate list,
// dock affordances) speaks bare tool names.
export function stripMcpPrefix(toolName) {
  const m = /^mcp__[^_]+(?:_[^_]+)*__(.+)$/.exec(toolName);
  if (m) return m[1];
  return toolName;
}
