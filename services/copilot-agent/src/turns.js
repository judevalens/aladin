// In-memory registry of running turns. One turn = one POST /turn request; the
// Go API cancels or resolves approvals by turnId while the NDJSON response is
// still streaming.

const turns = new Map();

export function createTurn(turnId) {
  if (turns.has(turnId)) {
    return null; // duplicate turn id — caller 409s
  }
  const turn = {
    id: turnId,
    abortController: new AbortController(),
    // approvalId → {resolve, timer} for gated tool calls held open in canUseTool.
    approvals: new Map(),
    // toolUseID → approvalId, so the eventual tool_result event can carry the
    // approvalId it settles (the dock's action_result).
    approvalByToolUse: new Map(),
  };
  turns.set(turnId, turn);
  return turn;
}

export function getTurn(turnId) {
  return turns.get(turnId) ?? null;
}

// endTurn aborts the SDK query (if still running), denies every held approval,
// and forgets the turn. Idempotent.
export function endTurn(turnId) {
  const turn = turns.get(turnId);
  if (!turn) return;
  turns.delete(turnId);
  for (const [, pending] of turn.approvals) {
    clearTimeout(pending.timer);
    pending.resolve({ approved: false, timedOut: false, aborted: true });
  }
  turn.approvals.clear();
  turn.abortController.abort();
}
